package escrow

import (
	"crypto/sha256"
	"encoding/binary"
)

// Anchor prefixes every instruction payload with an 8-byte discriminator derived
// from the instruction's snake_case name, and every account with one derived
// from its type name. Getting a discriminator wrong is not a security failure —
// the program simply does not recognise the instruction — but it is a cheap way
// to waste a devnet round trip, so the eight the escrow program exposes are
// pinned by known-answer test in anchor_test.go.
const (
	IxInitConfig       = "init_config"
	IxProposeAdmin     = "propose_admin"
	IxAcceptAdmin      = "accept_admin"
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
//
// `issuer` IS sent, and it is the point of this instruction: it pins the one
// issuer whose attestation can ever release this escrow. The program stores it
// immutably and ANDs it with the allowlist at release, so an issuer added after
// this deposit — including one added by a compromised admin — cannot touch it.
// Pass the issuer the payer actually trusts, not whatever the allowlist happens
// to contain at deposit time.
func InitEscrowData(amount uint64, resourceID, nonce, issuer [32]byte) []byte {
	d := Discriminator(IxInitEscrow)
	out := make([]byte, 0, 8+8+32+32+32)
	out = append(out, d[:]...)
	var amt [8]byte
	binary.LittleEndian.PutUint64(amt[:], amount)
	out = append(out, amt[:]...)
	out = append(out, resourceID[:]...)
	out = append(out, nonce[:]...)
	out = append(out, issuer[:]...)
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

// InitConfigData encodes init_config(), which takes no arguments. The admin is
// named by ACCOUNT, not by argument, and must be a different key from the signing
// upgrade authority — the program rejects the transaction if they match.
func InitConfigData() []byte {
	d := Discriminator(IxInitConfig)
	return d[:]
}

// ProposeAdminData encodes propose_admin(new_admin: Pubkey), step 1 of the
// two-step admin handover. Nothing changes until the nominee calls accept_admin.
func ProposeAdminData(newAdmin [32]byte) []byte {
	d := Discriminator(IxProposeAdmin)
	out := make([]byte, 0, 8+32)
	out = append(out, d[:]...)
	return append(out, newAdmin[:]...)
}

// AcceptAdminData encodes accept_admin(), step 2. It takes no arguments: the
// nominee is identified by the signing account, which is what makes the handover
// proof-of-possession rather than an assertion by the outgoing admin.
func AcceptAdminData() []byte {
	d := Discriminator(IxAcceptAdmin)
	return d[:]
}
