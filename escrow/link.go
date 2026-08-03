package escrow

import (
	"errors"
	"math/big"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
)

// This file is the join between the two enforcement points, and it exists mainly
// to make their differences explicit rather than to hide them.
//
// The gate (SPEC-X402 §4) decides ALLOW/DENY before anything is signed. It is
// the right control for a cooperating client and it is where policy lives, but
// it is advisory: nothing stops a client that ignores the answer from signing a
// transfer anyway. The pre-send guard in settle/ narrows that gap by asserting
// what a transaction pays before it is signed, but it still depends on the payer
// running our code.
//
// The escrow closes it. Funds move into a program-owned vault, and the only way
// out to the recipient is an instruction the program will execute solely against
// a fresh, allowlisted, correctly-bound issuer attestation. The payer's
// cooperation is no longer part of the trust model.
//
// The two bindings are NOT the same value and must never be conflated:
//
//	gate.ComputeBinding    tag spt-txn/x402-payment/v1          amount u128 (16 LE)
//	                       covers scheme, network, asset, payTo(ATA), amount,
//	                       sha256(resource), nonce
//	escrow.ComputeBinding  tag spt-txn/x402-escrow-binding/v1   amount u64 (8 LE)
//	                       covers payer, mint, amount, recipient(WALLET),
//	                       resource_id, nonce
//
// They share only the nonce (the token jti), which is what ties one gate
// decision to one escrow release. Distinct domain tags mean neither can ever be
// presented in place of the other.

var (
	ErrAmountNotU64      = errors.New("escrow: amount exceeds u64; SPL token amounts are u64")
	ErrAmountZero        = errors.New("escrow: amount must be greater than zero")
	ErrRecipientMismatch = errors.New("escrow: recipient wallet does not own the gate's PayTo token account")
)

// Params is everything needed to open and later release one escrow.
type Params struct {
	BindingParams
	Binding      [32]byte // ComputeBinding(BindingParams), cached
	RecipientATA [32]byte // derived; must equal the gate's PayTo
}

// FromGate maps an ALLOWed gate decision onto escrow parameters.
//
// recipientWallet is a separate input rather than something derived from
// pr.PayTo because the derivation only runs the other way: an ATA is a hash of
// (owner, tokenProgram, mint), so you can check a wallet against an ATA but you
// cannot recover the wallet from it. The caller must know who they are paying.
// FromGate verifies the claim rather than trusting it — if recipientWallet's ATA
// for this mint is not pr.PayTo, that is a hard error, because it would produce
// an escrow whose recipient constraint can never be satisfied and whose funds
// would sit until refund_expired.
//
// The nonce must be the same tok.Nonce the gate consumed. Reusing a nonce across
// two escrows is not merely sloppy: the spent-marker PDA is keyed on the
// binding, so the second release would fail at account init, stranding real
// funds until expiry.
func FromGate(al gate.Allowlist, pr gate.PaymentRequirements, payer, recipientWallet [32]byte, nonce [32]byte) (Params, error) {
	var p Params

	mint, err := DecodePubkey(pr.Asset)
	if err != nil {
		return p, err
	}
	payTo, err := DecodePubkey(pr.PayTo)
	if err != nil {
		return p, err
	}

	amountBig, ok := new(big.Int).SetString(pr.MaxAmountRequired, 10)
	if !ok {
		return p, gate.ErrBadAmount
	}
	if amountBig.Sign() <= 0 {
		return p, ErrAmountZero
	}
	if amountBig.BitLen() > 64 {
		// The gate binds a u128 because that is what the x402 protocol allows.
		// The escrow binds a u64 because that is what SPL tokens are. A value
		// in between is a real request the escrow path cannot carry, and
		// silently truncating it would move the wrong amount of money.
		return p, ErrAmountNotU64
	}

	ata, _, err := AssociatedTokenAddress(recipientWallet, mint)
	if err != nil {
		return p, err
	}
	if ata != payTo {
		return p, ErrRecipientMismatch
	}

	p.BindingParams = BindingParams{
		Payer:      payer,
		Mint:       mint,
		Amount:     amountBig.Uint64(),
		Recipient:  recipientWallet,
		ResourceID: ResourceID(pr.Resource),
		Nonce:      nonce,
	}
	p.Binding = ComputeBinding(p.BindingParams)
	p.RecipientATA = ata
	return p, nil
}

// Addresses is the set of account addresses one escrow occupies.
type Addresses struct {
	Config     [32]byte
	ConfigBump uint8
	Escrow     [32]byte
	EscrowBump uint8
	Vault      [32]byte
	VaultBump  uint8
	Spent      [32]byte
	SpentBump  uint8
}

// Derive computes every PDA for these params under the given program.
//
// Deriving all of them together, from the same Params, is deliberate: the escrow
// address commits to the binding, the vault address commits to the escrow
// address, and the spent marker commits to the binding again. Any parameter the
// caller gets wrong moves all of these at once, so a mismatched release fails on
// a seeds constraint rather than touching a real escrow.
func (p Params) Derive(programID [32]byte) (Addresses, error) {
	var a Addresses
	var err error

	if a.Config, a.ConfigBump, err = ConfigPDA(programID); err != nil {
		return a, err
	}
	if a.Escrow, a.EscrowBump, err = EscrowPDA(p.Payer, p.Recipient, p.Binding, programID); err != nil {
		return a, err
	}
	if a.Vault, a.VaultBump, err = VaultPDA(a.Escrow, programID); err != nil {
		return a, err
	}
	if a.Spent, a.SpentBump, err = SpentPDA(p.Binding, programID); err != nil {
		return a, err
	}
	return a, nil
}

// Attest builds the attestation that releases this escrow at time iat.
//
// There is exactly one way to produce a releasable attestation for a given
// escrow, and it runs through this function: the binding comes from Params, not
// from a caller-supplied value. That is the property the whole design rests on.
func (p Params) Attest(iat int64) Attestation {
	return Attestation{Binding: p.Binding, IAT: iat}
}
