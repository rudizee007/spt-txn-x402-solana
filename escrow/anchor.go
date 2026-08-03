package escrow

import (
	"crypto/sha256"
	"encoding/binary"
)

// Anchor prefixes every instruction payload with an 8-byte discriminator derived
// from the instruction's snake_case name, and every account with one derived
// from its type name. Getting a discriminator wrong is not a security failure —
// the program simply does not recognise the instruction — but it is a cheap way
// to waste a devnet round trip, so the six the escrow program exposes are pinned
// by known-answer test in anchor_test.go.
const (
	IxInitConfig       = "init_config"
	IxAddIssuer        = "add_issuer"
	IxRemoveIssuer     = "remove_issuer"
	IxInitEscrow       = "init_escrow"
	IxReleaseWithProof = "release_with_proof"
	IxRefundExpired    = "refund_expired"
)

// Discriminator returns Anchor's 8-byte instruction discriminator:
// the first 8 bytes of SHA-256("global:" + name).
func Discriminator(name string) [8]byte {
	sum := sha256.Sum256([]byte("global:" + name))
	var out [8]byte
	copy(out[:], sum[:8])
	return out
}

// AccountDiscriminator returns Anchor's 8-byte account discriminator:
// the first 8 bytes of SHA-256("account:" + TypeName). Used to sanity-check a
// fetched account before trusting its contents.
func AccountDiscriminator(typeName string) [8]byte {
	sum := sha256.Sum256([]byte("account:" + typeName))
	var out [8]byte
	copy(out[:], sum[:8])
	return out
}

// InitEscrowData encodes the instruction payload for init_escrow.
//
// Note what is NOT sent: the binding. The program recomputes it on-chain from
// the real account keys and these arguments (lib.rs::init_escrow), so a caller
// cannot store a binding that disagrees with the escrow it actually funded. The
// off-chain ComputeBinding in this package is for deriving the PDA address and
// for building the matching attestation — never for telling the program what to
// believe.
func InitEscrowData(amount uint64, resourceID, nonce [32]byte) []byte {
	d := Discriminator(IxInitEscrow)
	out := make([]byte, 0, 8+8+32+32)
	out = append(out, d[:]...)
	var amt [8]byte
	binary.LittleEndian.PutUint64(amt[:], amount)
	out = append(out, amt[:]...)
	out = append(out, resourceID[:]...)
	out = append(out, nonce[:]...)
	return out
}

// ReleaseWithProofData encodes the instruction payload for release_with_proof.
//
// It takes no arguments at all. Every input to the decision is either already on
// chain (the escrow's stored binding, the issuer allowlist, the validator clock)
// or comes from the Ed25519 precompile instruction that must precede it in the
// same transaction. There is no field here an attacker could vary.
func ReleaseWithProofData() []byte {
	d := Discriminator(IxReleaseWithProof)
	return d[:]
}

// RefundExpiredData encodes the instruction payload for refund_expired, which
// also takes no arguments: the program checks the escrow's own expiry against
// the clock.
func RefundExpiredData() []byte {
	d := Discriminator(IxRefundExpired)
	return d[:]
}

// AddIssuerData encodes add_issuer(issuer: Pubkey).
func AddIssuerData(issuer [32]byte) []byte {
	d := Discriminator(IxAddIssuer)
	out := make([]byte, 0, 8+32)
	out = append(out, d[:]...)
	return append(out, issuer[:]...)
}

// RemoveIssuerData encodes remove_issuer(issuer: Pubkey).
func RemoveIssuerData(issuer [32]byte) []byte {
	d := Discriminator(IxRemoveIssuer)
	out := make([]byte, 0, 8+32)
	out = append(out, d[:]...)
	return append(out, issuer[:]...)
}

// InitConfigData encodes init_config(), which takes no arguments.
func InitConfigData() []byte {
	d := Discriminator(IxInitConfig)
	return d[:]
}
