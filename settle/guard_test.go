package settle

import (
	"encoding/binary"
	"math/big"
	"testing"
)

func pk(b byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = b
	}
	return a
}

// Named actors for readability.
var (
	tokenProg = pk(0x99) // stands in for the SPL token program id
	srcAcct   = pk(0x01)
	mintAcct  = pk(0x02)
	dstAcct   = pk(0x03)
	payer     = pk(0x04)
	feePayer  = pk(0xFE) // the facilitator's gas sponsor — must be irrelevant here
)

func mkTransferIx(prog [32]byte, amount uint64, dec uint8, src, mint, dst, auth [32]byte) Instruction {
	data := make([]byte, 10)
	data[0] = TransferCheckedDiscriminator
	binary.LittleEndian.PutUint64(data[1:9], amount)
	data[9] = dec
	return Instruction{ProgramID: prog, Data: data, Accounts: [][32]byte{src, mint, dst, auth}}
}

func decoder() Decoder { return Decoder{TokenPrograms: [][32]byte{tokenProg}} }

func boundOK() BoundPayment {
	return BoundPayment{PayTo: dstAcct, Asset: mintAcct, Payer: payer, Amount: big.NewInt(1_000_000)}
}

// The happy path: a transfer that pays the bound recipient/asset/amount under
// the payer's authority passes.
func TestAssertMatches_OK(t *testing.T) {
	tr := mkTransferIx(tokenProg, 1_000_000, 6, srcAcct, mintAcct, dstAcct, payer)
	d := decoder()
	dec, err := d.Decode(tr)
	if err != nil {
		t.Fatal(err)
	}
	if err := AssertMatches(dec, boundOK()); err != nil {
		t.Fatalf("well-formed matching transfer should pass: %v", err)
	}
}

// Every field mismatch must refuse to sign.
func TestAssertMatches_Mismatches(t *testing.T) {
	base := DecodedTransfer{Source: srcAcct, Mint: mintAcct, Destination: dstAcct, Authority: payer, Amount: 1_000_000}

	cases := []struct {
		name string
		mut  func(*DecodedTransfer)
		want error
	}{
		{"wrong destination", func(x *DecodedTransfer) { x.Destination = pk(0x33) }, ErrDestinationMismatch},
		{"wrong mint", func(x *DecodedTransfer) { x.Mint = pk(0x22) }, ErrMintMismatch},
		{"wrong authority (attacker)", func(x *DecodedTransfer) { x.Authority = pk(0x44) }, ErrAuthorityMismatch},
		{"amount inflated", func(x *DecodedTransfer) { x.Amount = 1_000_001 }, ErrAmountMismatch},
	}
	for _, c := range cases {
		tr := base
		c.mut(&tr)
		if err := AssertMatches(tr, boundOK()); err != c.want {
			t.Errorf("%s: got %v want %v", c.name, err, c.want)
		}
	}
}

// A bound amount that does not fit u64 cannot be a Solana payment → refuse.
func TestAssertMatches_AmountRange(t *testing.T) {
	tr := DecodedTransfer{Destination: dstAcct, Mint: mintAcct, Authority: payer, Amount: 0}
	over := boundOK()
	over.Amount = new(big.Int).Lsh(big.NewInt(1), 64) // 2^64, one past u64
	if err := AssertMatches(tr, over); err != ErrAmountRange {
		t.Fatalf("expected ErrAmountRange, got %v", err)
	}
	nilAmt := boundOK()
	nilAmt.Amount = nil
	if err := AssertMatches(tr, nilAmt); err != ErrAmountRange {
		t.Fatalf("nil amount: expected ErrAmountRange, got %v", err)
	}
}

// The point of §6.4: a hostile fee payer cannot subvert the payment. The fee
// payer is not part of the transfer instruction, so a correct transfer still
// passes regardless of who sponsors gas; and if an attacker tries to make the
// fee payer the transfer AUTHORITY, the authority check rejects it.
func TestFeePayerCannotSubvert(t *testing.T) {
	d := decoder()

	// Correct transfer; a separate (hostile) feePayer sponsors gas elsewhere in
	// the tx. The guard only inspects the transfer, which is correct → pass.
	good := mkTransferIx(tokenProg, 1_000_000, 6, srcAcct, mintAcct, dstAcct, payer)
	if err := AssertTransactionPays(d, []Instruction{good}, boundOK()); err != nil {
		t.Fatalf("correct transfer must pass regardless of fee payer: %v", err)
	}

	// Attacker makes the feePayer the transfer authority → rejected.
	evil := mkTransferIx(tokenProg, 1_000_000, 6, srcAcct, mintAcct, dstAcct, feePayer)
	if err := AssertTransactionPays(d, []Instruction{evil}, boundOK()); err != ErrAuthorityMismatch {
		t.Fatalf("feePayer-as-authority must be rejected, got %v", err)
	}
}

// Exactly one transfer is required: zero and duplicate both fail closed.
func TestFindTransfer_Cardinality(t *testing.T) {
	d := decoder()
	tr := mkTransferIx(tokenProg, 1_000_000, 6, srcAcct, mintAcct, dstAcct, payer)
	other := Instruction{ProgramID: pk(0x77), Data: []byte{1, 2, 3}, Accounts: nil} // non-token noise

	if _, err := d.FindTransfer([]Instruction{other}); err != ErrNoTransfer {
		t.Fatalf("no transfer: got %v", err)
	}
	if _, err := d.FindTransfer([]Instruction{tr, tr}); err != ErrMultipleTransfers {
		t.Fatalf("duplicate transfer: got %v", err)
	}
	if _, err := d.FindTransfer([]Instruction{other, tr}); err != nil {
		t.Fatalf("exactly one transfer should decode: %v", err)
	}
}

// A foreign program or malformed data must not be accepted as a transfer.
func TestDecode_FailClosed(t *testing.T) {
	d := decoder()
	foreign := mkTransferIx(pk(0x88), 1, 6, srcAcct, mintAcct, dstAcct, payer) // not in TokenPrograms
	if _, err := d.Decode(foreign); err != ErrForeignProgram {
		t.Fatalf("foreign program: got %v", err)
	}
	short := Instruction{ProgramID: tokenProg, Data: []byte{TransferCheckedDiscriminator, 0x00}, Accounts: [][32]byte{srcAcct, mintAcct, dstAcct, payer}}
	if _, err := d.Decode(short); err != ErrShortData {
		t.Fatalf("short data: got %v", err)
	}
	wrongDisc := mkTransferIx(tokenProg, 1, 6, srcAcct, mintAcct, dstAcct, payer)
	wrongDisc.Data[0] = 3 // Transfer (unchecked), not TransferChecked
	if _, err := d.Decode(wrongDisc); err != ErrNotTransferChecked {
		t.Fatalf("wrong discriminator: got %v", err)
	}
}
