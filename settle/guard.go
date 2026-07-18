package settle

import (
	"errors"
	"math/big"
)

// BoundPayment is what the gate authorized: the recipient/asset/amount taken
// from the intent binding (SPEC-X402 §4), plus the authorized payer taken from
// the SPT-Txn token context. The amount is the binding's u128; a real Solana
// transfer is u64, so a bound amount that does not fit u64 cannot correspond to
// any Solana payment and is refused.
type BoundPayment struct {
	PayTo  [32]byte // bound recipient
	Asset  [32]byte // bound SPL mint
	Payer  [32]byte // authorized transfer authority (from the token)
	Amount *big.Int // bound amount (u128)
}

var (
	ErrDestinationMismatch = errors.New("settle: transfer destination != bound payTo")
	ErrMintMismatch        = errors.New("settle: transfer mint != bound asset")
	ErrAuthorityMismatch   = errors.New("settle: transfer authority != authorized payer")
	ErrAmountMismatch      = errors.New("settle: transfer amount != bound amount")
	ErrAmountRange         = errors.New("settle: bound amount is nil, negative, or exceeds u64")
)

// AssertMatches is the SPEC-X402 §6.4 pre-send gate. Call it on the decoded
// TransferChecked immediately before signing; a non-nil error means DO NOT SIGN
// (abort, DENY_VIOLATION). It pins the value-moving instruction to exactly what
// the gate authorized:
//
//   - destination == bound payTo   (funds go where authorized)
//   - mint        == bound asset   (the right token)
//   - amount      == bound amount  (no inflation, no truncation)
//   - authority   == authorized payer
//
// The authority check is the one that defeats fee-payer abuse: the transaction
// fee payer (extra.feePayer) sponsors gas but is NOT the transfer authority, and
// is not even present in the transfer instruction, so a hostile feePayer cannot
// move the payer's funds. Because the authority must equal the authorized payer,
// the source token account is necessarily payer-controlled — SPL TransferChecked
// only lets the account's owner/delegate move it.
func AssertMatches(t DecodedTransfer, b BoundPayment) error {
	if t.Destination != b.PayTo {
		return ErrDestinationMismatch
	}
	if t.Mint != b.Asset {
		return ErrMintMismatch
	}
	if t.Authority != b.Payer {
		return ErrAuthorityMismatch
	}
	if b.Amount == nil || b.Amount.Sign() < 0 || b.Amount.BitLen() > 64 {
		return ErrAmountRange
	}
	if t.Amount != b.Amount.Uint64() {
		return ErrAmountMismatch
	}
	return nil
}

// AssertTransactionPays is the convenience entry point: find the single
// TransferChecked in the transaction and assert it matches the bound payment.
// Any decode problem, a missing/duplicate transfer, or a field mismatch is a
// refuse-to-sign error.
func AssertTransactionPays(d Decoder, ixs []Instruction, b BoundPayment) error {
	t, err := d.FindTransfer(ixs)
	if err != nil {
		return err
	}
	return AssertMatches(t, b)
}
