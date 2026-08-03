package escrow

import (
	"errors"
	"math/big"
)

// Well-known account addresses, in their canonical base58 form.
//
// They are declared as strings and decoded at init rather than pasted in as byte
// arrays on purpose: a mistyped byte in a hand-transcribed 32-byte literal is
// invisible on review and produces a transaction that fails for reasons that
// look nothing like "wrong constant". Decoding from the string that a human can
// actually read against an explorer, and re-encoding it in the tests, makes the
// error impossible to introduce silently.
const (
	// Ed25519ProgramIDBase58 is Solana's native signature-verification
	// precompile. The escrow program verifies no signatures itself: it looks
	// for an instruction addressed to this program earlier in the same
	// transaction and reads the (pubkey, message) pair the runtime has already
	// verified. No custom crypto in BPF (SPEC §6).
	Ed25519ProgramIDBase58 = "Ed25519SigVerify111111111111111111111111111"

	// InstructionsSysvarBase58 is the read-only account through which a program
	// can introspect the other instructions in its own transaction. This is the
	// mechanism that makes the precompile result observable at all.
	InstructionsSysvarBase58 = "Sysvar1nstructions1111111111111111111111111"

	// EscrowProgramIDBase58 is the deployed spt_x402_escrow program on devnet.
	EscrowProgramIDBase58 = "C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ"

	// TokenProgramIDBase58 is the SPL Token program (not Token-2022; the escrow
	// program's account constraints are written against classic SPL).
	TokenProgramIDBase58 = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

	// AssociatedTokenProgramIDBase58 derives and creates associated token
	// accounts.
	AssociatedTokenProgramIDBase58 = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"

	// SystemProgramIDBase58 is the all-zero address.
	SystemProgramIDBase58 = "11111111111111111111111111111111"
)

// Decoded forms of the constants above.
var (
	Ed25519ProgramID         = mustDecodePubkey(Ed25519ProgramIDBase58)
	InstructionsSysvarID     = mustDecodePubkey(InstructionsSysvarBase58)
	EscrowProgramID          = mustDecodePubkey(EscrowProgramIDBase58)
	TokenProgramID           = mustDecodePubkey(TokenProgramIDBase58)
	AssociatedTokenProgramID = mustDecodePubkey(AssociatedTokenProgramIDBase58)
	SystemProgramID          = mustDecodePubkey(SystemProgramIDBase58)
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var (
	ErrBase58Char   = errors.New("escrow: invalid base58 character")
	ErrBase58Length = errors.New("escrow: decoded value is not a 32-byte pubkey")
)

// DecodePubkey decodes a base58 Solana address to its 32 raw bytes.
//
// Leading '1' characters encode leading zero bytes, which is why the length
// check is on the decoded output and not on the input string: the System program
// address is 32 characters, the Ed25519 precompile 43, and both are valid
// 32-byte keys.
func DecodePubkey(s string) ([32]byte, error) {
	var out [32]byte
	n := new(big.Int)
	radix := big.NewInt(58)
	for _, c := range []byte(s) {
		idx := indexBase58(c)
		if idx < 0 {
			return out, ErrBase58Char
		}
		n.Mul(n, radix)
		n.Add(n, big.NewInt(int64(idx)))
	}
	be := n.Bytes()
	if len(be) > 32 {
		return out, ErrBase58Length
	}
	// Right-align: the big.Int has dropped leading zero bytes, and for an
	// address those zeros are part of the value.
	copy(out[32-len(be):], be)

	// A string of leading '1's longer than the zero prefix we just produced
	// would mean the caller handed us something that is not 32 bytes wide.
	lead := 0
	for lead < len(s) && s[lead] == '1' {
		lead++
	}
	if lead > 32-len(be) {
		return out, ErrBase58Length
	}
	return out, nil
}

func indexBase58(c byte) int {
	for i := 0; i < len(base58Alphabet); i++ {
		if base58Alphabet[i] == c {
			return i
		}
	}
	return -1
}

func mustDecodePubkey(s string) [32]byte {
	k, err := DecodePubkey(s)
	if err != nil {
		panic("escrow: bad built-in address " + s + ": " + err.Error())
	}
	return k
}
