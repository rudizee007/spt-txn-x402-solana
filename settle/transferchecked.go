// Package settle contains the payer-side settlement guard for the SPT-Txn x402
// flow. Its job is the SPEC-X402 §6.4 assertion: after the gate says ALLOW, and
// before anything is signed, prove the constructed transaction actually pays the
// bound recipient/asset/amount under the authorized payer — no more, no less.
//
// This file decodes the SPL Token / Token-2022 `TransferChecked` instruction. It
// deliberately does NOT import a Solana SDK: it operates on already-resolved
// instruction fields (program id, data, ordered account pubkeys), so it is
// dependency-free and unit-testable offline. The transaction builder/signer that
// produces those fields plugs in at demo time.
package settle

import (
	"encoding/binary"
	"errors"
)

// TransferCheckedDiscriminator is the SPL Token instruction tag for
// TransferChecked. Data layout: [tag u8][amount u64 LE][decimals u8] = 10 bytes.
const TransferCheckedDiscriminator = 12

var (
	ErrNotTransferChecked = errors.New("settle: instruction is not TransferChecked")
	ErrForeignProgram     = errors.New("settle: instruction not from an accepted token program")
	ErrShortData          = errors.New("settle: TransferChecked data too short")
	ErrFewAccounts        = errors.New("settle: TransferChecked has too few accounts")
	ErrNoTransfer         = errors.New("settle: no TransferChecked instruction found")
	ErrMultipleTransfers  = errors.New("settle: more than one TransferChecked instruction (refusing to guess)")
)

// Instruction is one instruction of a to-be-signed transaction, with its account
// references already resolved to ordered pubkeys.
type Instruction struct {
	ProgramID [32]byte
	Data      []byte
	Accounts  [][32]byte
}

// DecodedTransfer is the meaningful content of a TransferChecked: who moves how
// much of what, to whom, under whose authority. Note there is no fee-payer here
// — the fee payer is a transaction-level account, not part of the transfer, so
// it can never satisfy or subvert the §6.4 assertion.
type DecodedTransfer struct {
	Source      [32]byte
	Mint        [32]byte
	Destination [32]byte
	Authority   [32]byte
	Amount      uint64
	Decimals    uint8
}

// Decoder knows which program ids count as SPL token programs (SPL Token and/or
// Token-2022). Supplying them as config keeps this package free of base58 and of
// hardcoded ids.
type Decoder struct {
	TokenPrograms [][32]byte
}

func (d Decoder) isTokenProgram(p [32]byte) bool {
	for _, tp := range d.TokenPrograms {
		if tp == p {
			return true
		}
	}
	return false
}

func isTransferChecked(d Decoder, ix Instruction) bool {
	return d.isTokenProgram(ix.ProgramID) && len(ix.Data) >= 1 && ix.Data[0] == TransferCheckedDiscriminator
}

// Decode parses a single TransferChecked instruction. Any structural problem is
// an error (fail closed) — a transfer we cannot fully decode is never asserted
// as matching.
func (d Decoder) Decode(ix Instruction) (DecodedTransfer, error) {
	var t DecodedTransfer
	if !d.isTokenProgram(ix.ProgramID) {
		return t, ErrForeignProgram
	}
	if len(ix.Data) < 10 {
		return t, ErrShortData
	}
	if ix.Data[0] != TransferCheckedDiscriminator {
		return t, ErrNotTransferChecked
	}
	if len(ix.Accounts) < 4 {
		return t, ErrFewAccounts
	}
	t.Source = ix.Accounts[0]
	t.Mint = ix.Accounts[1]
	t.Destination = ix.Accounts[2]
	t.Authority = ix.Accounts[3]
	t.Amount = binary.LittleEndian.Uint64(ix.Data[1:9])
	t.Decimals = ix.Data[9]
	return t, nil
}

// FindTransfer requires exactly one TransferChecked across the transaction's
// instructions and returns it. Zero → ErrNoTransfer; more than one →
// ErrMultipleTransfers. Requiring exactly one blocks a decoy second transfer
// that an attacker might slip in alongside the intended one.
func (d Decoder) FindTransfer(ixs []Instruction) (DecodedTransfer, error) {
	var found DecodedTransfer
	count := 0
	for _, ix := range ixs {
		if !isTransferChecked(d, ix) {
			continue
		}
		dec, err := d.Decode(ix)
		if err != nil {
			return DecodedTransfer{}, err
		}
		found = dec
		count++
	}
	switch count {
	case 0:
		return DecodedTransfer{}, ErrNoTransfer
	case 1:
		return found, nil
	default:
		return DecodedTransfer{}, ErrMultipleTransfers
	}
}
