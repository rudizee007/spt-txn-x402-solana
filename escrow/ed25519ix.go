package escrow

import (
	"encoding/binary"
	"errors"
)

// Fixed offsets in a single-signature Ed25519 precompile instruction.
//
//	[0]      num_signatures u8   (must be 1)
//	[1]      padding u8
//	[2..16)  offsets struct, seven little-endian u16 in this order:
//	         sig_off, sig_ix, pk_off, pk_ix, msg_off, msg_size, msg_ix
//	[16..48) public key   (32 bytes)
//	[48..112) signature   (64 bytes)
//	[112..)  message
//
// The three *_ix fields are instruction indices. Setting them to u16::MAX
// (ixSelf) means "the data lives in this same instruction". The escrow program
// REQUIRES that sentinel for the pubkey and the message: an attestation whose
// bytes are sourced from some other instruction in the transaction is out of
// scope and denied, because the program cannot cheaply prove what that other
// instruction contains (THREAT-MODEL T2).
const (
	ed25519HeaderLen = 16
	ed25519PubOff    = 16
	ed25519SigOff    = 48
	ed25519MsgOff    = 112

	ixSelf uint16 = 0xFFFF
)

var (
	ErrPubkeyLength    = errors.New("escrow: ed25519 pubkey must be 32 bytes")
	ErrSignatureLength = errors.New("escrow: ed25519 signature must be 64 bytes")
	ErrMessageTooLong  = errors.New("escrow: message length exceeds u16")
	ErrIxMalformed     = errors.New("escrow: malformed ed25519 precompile instruction")
	ErrIxNotSelf       = errors.New("escrow: ed25519 offsets reference another instruction")
	ErrIxSigCount      = errors.New("escrow: ed25519 instruction must carry exactly one signature")
)

// BuildEd25519Instruction encodes the data payload for the native Ed25519
// precompile instruction that must immediately precede release_with_proof.
//
// It emits exactly one signature. Batching is rejected by the program: with more
// than one signature the introspection stops being total — the program would
// have to decide *which* verified pair authorizes the release, and any such
// choice is an opportunity to smuggle in an attacker-chosen pair alongside a
// legitimate one.
func BuildEd25519Instruction(pubkey, sig, msg []byte) ([]byte, error) {
	if len(pubkey) != 32 {
		return nil, ErrPubkeyLength
	}
	if len(sig) != 64 {
		return nil, ErrSignatureLength
	}
	if len(msg) > 0xFFFF {
		return nil, ErrMessageTooLong
	}

	data := make([]byte, ed25519MsgOff, ed25519MsgOff+len(msg))
	data[0] = 1 // num_signatures
	data[1] = 0 // padding

	offsets := []uint16{
		ed25519SigOff, ixSelf, // sig_off, sig_ix
		ed25519PubOff, ixSelf, // pk_off,  pk_ix
		ed25519MsgOff, uint16(len(msg)), ixSelf, // msg_off, msg_size, msg_ix
	}
	for i, v := range offsets {
		binary.LittleEndian.PutUint16(data[2+2*i:], v)
	}

	copy(data[ed25519PubOff:], pubkey)
	copy(data[ed25519SigOff:], sig)
	return append(data, msg...), nil
}

// ParseEd25519Instruction mirrors the program's parser
// (verify.rs::parse_ed25519_instruction) field for field and check for check.
//
// It exists so the off-chain side can assert, before spending a transaction,
// that the bytes it is about to submit are the bytes the verifier will read. A
// divergence between this function and the Rust one is exactly the class of bug
// that produces an authorization bypass, so it is written as a transcription of
// the Rust rather than as an independent "better" parser.
func ParseEd25519Instruction(data []byte) (pubkey [32]byte, msg []byte, err error) {
	if len(data) < ed25519HeaderLen {
		return pubkey, nil, ErrIxMalformed
	}
	if data[0] != 1 {
		return pubkey, nil, ErrIxSigCount
	}

	const b = 2
	pkOff := int(binary.LittleEndian.Uint16(data[b+4:]))
	pkIx := binary.LittleEndian.Uint16(data[b+6:])
	msgOff := int(binary.LittleEndian.Uint16(data[b+8:]))
	msgSize := int(binary.LittleEndian.Uint16(data[b+10:]))
	msgIx := binary.LittleEndian.Uint16(data[b+12:])

	if pkIx != ixSelf || msgIx != ixSelf {
		return pubkey, nil, ErrIxNotSelf
	}
	if pkOff+32 > len(data) {
		return pubkey, nil, ErrIxMalformed
	}
	copy(pubkey[:], data[pkOff:pkOff+32])

	if msgSize != AttestMsgLen {
		return pubkey, nil, ErrAttestLength
	}
	if msgOff+msgSize > len(data) {
		return pubkey, nil, ErrIxMalformed
	}
	return pubkey, data[msgOff : msgOff+msgSize], nil
}
