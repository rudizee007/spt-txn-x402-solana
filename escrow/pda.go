package escrow

import (
	"crypto/sha256"
	"errors"
	"math/big"
)

// PDA seeds, mirroring constants.rs. These strings are part of the account
// addresses themselves — changing one does not "rename" anything, it points the
// program at a different account that does not exist.
const (
	SeedConfig = "config"
	SeedEscrow = "escrow"
	SeedVault  = "vault"

	// SeedSpent marks a binding as permanently consumed. The marker account is
	// created by release_with_proof and never closed, which is what makes
	// single use a property of the account system rather than of program logic:
	// a second release of the same binding fails at account initialization,
	// before any of our code runs. This is the structural fix for the
	// captured-attestation replay found in adversarial review (Finding 1).
	SeedSpent = "spent"
)

// pdaMarker is appended to every PDA preimage by the runtime, so that a PDA
// preimage can never collide with some other SHA-256 use.
const pdaMarker = "ProgramDerivedAddress"

var ErrNoPDABump = errors.New("escrow: unable to find a PDA bump seed")

// FindProgramAddress reproduces Solana's PDA derivation: hash the seeds, a
// candidate bump, the program id and the marker, and accept the first result
// that is NOT a valid Ed25519 curve point.
//
// The off-curve requirement is the whole security property. A point on the curve
// could have a corresponding private key, and therefore an owner who could sign
// for it; an off-curve address provably has none, so the only thing that can
// authorize it is the program, via invoke_signed.
//
// Bumps descend from 255 so that the derivation is deterministic — "the" PDA is
// the one with the highest bump that works. The escrow program stores each bump
// it used, so a caller that derives a different (valid but lower) bump gets a
// seeds-constraint failure rather than access to anything.
//
// This is duplicated from the Solana SDK deliberately: it keeps this package
// dependency-free and offline-testable. cmd/escrowdevnet cross-checks every
// address this function produces against the SDK's own derivation before
// sending, so a divergence aborts instead of burning a transaction.
func FindProgramAddress(seeds [][]byte, programID [32]byte) (addr [32]byte, bump uint8, err error) {
	for b := 255; b >= 0; b-- {
		candidate := createProgramAddress(seeds, uint8(b), programID)
		if !IsOnCurve(candidate) {
			return candidate, uint8(b), nil
		}
	}
	return addr, 0, ErrNoPDABump
}

func createProgramAddress(seeds [][]byte, bump uint8, programID [32]byte) [32]byte {
	h := sha256.New()
	for _, s := range seeds {
		h.Write(s)
	}
	h.Write([]byte{bump})
	h.Write(programID[:])
	h.Write([]byte(pdaMarker))
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// ConfigPDA returns the singleton config account address.
func ConfigPDA(programID [32]byte) ([32]byte, uint8, error) {
	return FindProgramAddress([][]byte{[]byte(SeedConfig)}, programID)
}

// EscrowPDA returns the escrow account address for one authorization.
//
// The binding is a seed, so the escrow's address is itself a commitment to
// (payer, mint, amount, recipient, resource, nonce). You cannot point a release
// at an escrow whose parameters differ from the ones the attestation covers,
// because such an escrow lives at a different address.
func EscrowPDA(payer, recipient, binding [32]byte, programID [32]byte) ([32]byte, uint8, error) {
	return FindProgramAddress([][]byte{
		[]byte(SeedEscrow), payer[:], recipient[:], binding[:],
	}, programID)
}

// VaultPDA returns the token account that custodies the escrowed funds.
func VaultPDA(escrowKey, programID [32]byte) ([32]byte, uint8, error) {
	return FindProgramAddress([][]byte{[]byte(SeedVault), escrowKey[:]}, programID)
}

// SpentPDA returns the permanent single-use marker for a binding.
func SpentPDA(binding, programID [32]byte) ([32]byte, uint8, error) {
	return FindProgramAddress([][]byte{[]byte(SeedSpent), binding[:]}, programID)
}

// AssociatedTokenAddress derives the associated token account for (owner, mint)
// under the classic SPL Token program.
//
// This matters for a reason that is easy to get wrong: the escrow's `recipient`
// field is a *wallet*, but the account that actually receives tokens is that
// wallet's ATA, and the program enforces `recipient_ata.owner == escrow.recipient`.
// Passing the wallet where the ATA belongs — or the reverse — is the single most
// likely integration mistake in this path. See link.go.
func AssociatedTokenAddress(owner, mint [32]byte) ([32]byte, uint8, error) {
	return FindProgramAddress([][]byte{
		owner[:], TokenProgramID[:], mint[:],
	}, AssociatedTokenProgramID)
}

// ── Ed25519 curve-point test ────────────────────────────────────────────────

var (
	// p = 2^255 - 19
	curveP = func() *big.Int {
		p := new(big.Int).Lsh(big.NewInt(1), 255)
		return p.Sub(p, big.NewInt(19))
	}()
	// d = -121665 / 121666 mod p
	curveD = func() *big.Int {
		num := new(big.Int).Neg(big.NewInt(121665))
		den := big.NewInt(121666)
		inv := new(big.Int).ModInverse(den, curveP)
		return new(big.Int).Mod(new(big.Int).Mul(num, inv), curveP)
	}()
	// sqrtM1 = 2^((p-1)/4) mod p, the square root of -1
	sqrtM1 = func() *big.Int {
		e := new(big.Int).Sub(curveP, big.NewInt(1))
		e.Rsh(e, 2)
		return new(big.Int).Exp(big.NewInt(2), e, curveP)
	}()
	// (p+3)/8, the exponent used to take a candidate square root
	expP38 = func() *big.Int {
		e := new(big.Int).Add(curveP, big.NewInt(3))
		return e.Rsh(e, 3)
	}()
)

// IsOnCurve reports whether 32 bytes decode to a valid compressed Ed25519 point.
//
// This is point decompression, not a membership shortcut: the input encodes y in
// little-endian with the top bit carrying the sign of x. Recover x² from the
// curve equation, take its square root, and check the result actually squares
// back. Anything that fails is off-curve, which for PDA purposes is what we
// want.
//
// The one input the encoding cannot represent is x == 0 with the sign bit set —
// there is no negative zero — so that is rejected explicitly.
func IsOnCurve(b [32]byte) bool {
	le := make([]byte, 32)
	copy(le, b[:])
	sign := le[31] >> 7
	le[31] &= 0x7F

	// Interpret little-endian.
	be := make([]byte, 32)
	for i := range le {
		be[31-i] = le[i]
	}
	y := new(big.Int).SetBytes(be)
	y.Mod(y, curveP)

	// x^2 = (y^2 - 1) / (d*y^2 + 1)
	y2 := new(big.Int).Mul(y, y)
	y2.Mod(y2, curveP)

	u := new(big.Int).Sub(y2, big.NewInt(1))
	u.Mod(u, curveP)

	v := new(big.Int).Mul(curveD, y2)
	v.Add(v, big.NewInt(1))
	v.Mod(v, curveP)

	vInv := new(big.Int).ModInverse(v, curveP)
	if vInv == nil {
		return false // v == 0: no such point
	}
	x2 := new(big.Int).Mul(u, vInv)
	x2.Mod(x2, curveP)

	if x2.Sign() == 0 {
		// x == 0 is only a point if the sign bit agrees.
		return sign == 0
	}

	// Candidate root: x = x2^((p+3)/8)
	x := new(big.Int).Exp(x2, expP38, curveP)
	chk := new(big.Int).Mul(x, x)
	chk.Mod(chk, curveP)
	if chk.Cmp(x2) != 0 {
		// Try the other root: x * sqrt(-1)
		x.Mul(x, sqrtM1)
		x.Mod(x, curveP)
		chk.Mul(x, x)
		chk.Mod(chk, curveP)
		if chk.Cmp(x2) != 0 {
			return false // x2 is not a quadratic residue: off curve
		}
	}
	return true
}
