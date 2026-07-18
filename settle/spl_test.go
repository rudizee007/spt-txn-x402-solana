package settle

import (
	"encoding/hex"
	"testing"
)

// Program-id bytes are frozen and cross-checked against an independent base58
// decode (see the M2 KAT). A wrong program id would send the transfer to the
// wrong program entirely.
func TestProgramIDBytes(t *testing.T) {
	if got := hex.EncodeToString(SPLTokenProgramID[:]); got != "06ddf6e1d765a193d9cbe146ceeb79ac1cb485ed5f5b37913a8cf5857eff00a9" {
		t.Fatalf("SPL Token program id = %s", got)
	}
	if got := hex.EncodeToString(Token2022ProgramID[:]); got != "06ddf6e1ee758fde18425dbce46ccddab61afc4d83b90d27febdf928d8a18bfc" {
		t.Fatalf("Token-2022 program id = %s", got)
	}
	if got := hex.EncodeToString(USDCDevnetMint[:]); got != "3b442cb3912157f13a933d0134282d032b5ffecd01a2dbf1b7790608df002ea7" {
		t.Fatalf("USDC devnet mint = %s", got)
	}
}

// The built instruction must have the exact on-chain bytes and round-trip
// through the decoder + guard.
func TestBuildTransferChecked_KAT(t *testing.T) {
	ix := BuildTransferChecked(SPLTokenProgramID, pk(0x01), pk(0x02), pk(0x03), pk(0x04), 1_500_000, 6)

	if ix.ProgramID != SPLTokenProgramID {
		t.Fatal("program id mismatch")
	}
	// data = [12] ++ u64_le(1_500_000) ++ [6]
	if got := hex.EncodeToString(ix.Data); got != "0c60e316000000000006" {
		t.Fatalf("TransferChecked data = %s", got)
	}
	if len(ix.Accounts) != 4 || ix.Accounts[0] != pk(0x01) || ix.Accounts[1] != pk(0x02) ||
		ix.Accounts[2] != pk(0x03) || ix.Accounts[3] != pk(0x04) {
		t.Fatal("account order wrong")
	}

	d := Decoder{TokenPrograms: [][32]byte{SPLTokenProgramID}}
	dec, err := d.Decode(ix)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dec.Amount != 1_500_000 || dec.Decimals != 6 || dec.Source != pk(0x01) || dec.Destination != pk(0x03) {
		t.Fatalf("decoded wrong: %+v", dec)
	}
}
