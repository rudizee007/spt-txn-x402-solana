// Package escrow is the off-chain (issuer) half of the on-chain release-on-proof
// settlement path. It produces exactly the bytes the Anchor program
// `spt_x402_escrow` verifies, and nothing else.
//
// Where the `gate` package decides ALLOW/DENY *before* a transfer is signed
// (docs/SPEC-X402.md §4), this package carries that decision *into* the chain:
// the payer's funds sit in a program-owned vault and are released only against a
// fresh, issuer-signed attestation that is cryptographically bound to that one
// escrow. The gate is advisory to a cooperating client; the escrow is not.
//
// # Why this file is security-critical
//
// The single highest-risk property in the whole design is that the issuer and
// the on-chain verifier canonicalize the authorization *byte for byte* the same
// way. If they diverge, an attestation issued for one payment can satisfy the
// program's check for a different one — a full authorization bypass
// (docs/THREAT-MODEL.md T1). There is no JSON here, no field ordering, no
// optional values, nothing to negotiate: every construction below is fixed
// width, fixed order, and domain separated.
//
// The Rust side of this contract is
// `programs/spt_x402_escrow/src/verify.rs::compute_binding` in the
// spt-txn-x402-escrow repository. Any change to a tag, a width, or an order is a
// breaking change on both sides at once, and must be re-pinned against the
// shared known-answer vector in binding_test.go.
//
// # Deliberately dependency-free
//
// This package imports only the Go standard library. The Solana SDK appears only
// in cmd/escrowdevnet, behind a `devnet` build tag. That keeps the bytes that
// authorize a release testable in isolation, on any machine, with no network and
// no toolchain beyond `go test`.
package escrow

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	// DomainTagEscrow separates the escrow payment binding from every other
	// SHA-256 construction in SPT-Txn. It is deliberately NOT the same tag as
	// gate.DomainTagX402: the gate binding and the escrow binding cover
	// different fields and must never be substitutable for one another
	// (SPEC §4).
	DomainTagEscrow = "spt-txn/x402-escrow-binding/v1"

	// LayoutVersion is hashed into the binding and into the attestation. Bump
	// it on any change to the byte layouts in this package, so an attestation
	// minted under an old layout can never be replayed against a new one.
	LayoutVersion = 1
)

// BindingParams is the complete set of fields covered by an escrow binding.
//
// The binding is *instance-unique*: Payer and Nonce are included precisely so
// that one issuer attestation authorizes exactly one escrow. An earlier design
// omitted them, which meant a single attestation could release every escrow that
// happened to share (Mint, Amount, Recipient, ResourceID) — a cross-escrow
// replay and fund sweep found in adversarial review (THREAT-MODEL T4). Do not
// remove a field from this struct without re-reading that finding.
//
// Recipient is a *wallet* address, not a token account. The program enforces
// `recipient_ata.owner == escrow.recipient`. This differs from the gate's PayTo,
// which is an associated token account; see link.go for the mapping and why the
// two values are not interchangeable.
type BindingParams struct {
	Payer      [32]byte // funding wallet; makes the binding instance-unique
	Mint       [32]byte // SPL mint being escrowed
	Amount     uint64   // atomic units of Mint
	Recipient  [32]byte // destination WALLET (not its token account)
	ResourceID [32]byte // SHA-256 of the x402 resource identifier
	Nonce      [32]byte // per-authorization nonce; the token jti
}

// ComputeBinding returns the 32-byte escrow binding.
//
// Layout, authoritative:
//
//	DOMAIN_TAG_ESCROW  (30 bytes, no terminator)
//	0x00               (1 byte, separator)
//	LAYOUT_VERSION     (1 byte)
//	payer              (32 bytes)
//	mint               (32 bytes)
//	amount             (8 bytes, little-endian u64)
//	recipient          (32 bytes)
//	resource_id        (32 bytes)
//	nonce              (32 bytes)
//
// Note the width difference from gate.ComputeBinding, which encodes its amount
// as a 16-byte little-endian u128. That is not an inconsistency to "fix": the
// gate binds an x402 protocol amount of arbitrary precision, while the escrow
// binds an SPL token amount, which the SPL token program itself defines as u64.
// Widening this field would desynchronize it from the Rust verifier.
func ComputeBinding(p BindingParams) [32]byte {
	var amt [8]byte
	binary.LittleEndian.PutUint64(amt[:], p.Amount)

	h := sha256.New()
	h.Write([]byte(DomainTagEscrow))
	h.Write([]byte{0x00})
	h.Write([]byte{LayoutVersion})
	h.Write(p.Payer[:])
	h.Write(p.Mint[:])
	h.Write(amt[:])
	h.Write(p.Recipient[:])
	h.Write(p.ResourceID[:])
	h.Write(p.Nonce[:])

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// ResourceID hashes an x402 resource identifier to the 32 bytes the program
// stores. The resource string is bound byte-exact — no trimming, no case
// folding, no URL normalization — because any normalization step is somewhere
// the issuer and a verifier can disagree.
func ResourceID(resource string) [32]byte {
	return sha256.Sum256([]byte(resource))
}
