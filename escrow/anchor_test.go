package escrow

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// TestDiscriminators pins all eight instruction discriminators the escrow program
// exposes. These were computed independently (SHA-256 of "global:"+name, first 8
// bytes) and checked against the program's IDL.
func TestDiscriminators(t *testing.T) {
	want := map[string]string{
		IxInitConfig:       "17eb73e8a86001e7",
		IxProposeAdmin:     "79d6c7d4572775ea",
		IxAcceptAdmin:      "702a2d5a74b50daa",
		IxAddIssuer:        "fc6103dd41a2b120",
		IxRemoveIssuer:     "004b58e1049fa777",
		IxInitEscrow:       "462e2817060b518b",
		IxReleaseWithProof: "7f2311f9fea0f0c1",
		IxRefundExpired:    "7699a4f42880f2fa",
	}
	for name, w := range want {
		d := Discriminator(name)
		if got := hex.EncodeToString(d[:]); got != w {
			t.Errorf("%s: got %s, want %s", name, got, w)
		}
	}
}

// TestAccountDiscriminatorDiffersFromInstruction guards against using the wrong
// prefix — "account:" and "global:" namespaces are distinct and mixing them
// produces a valid-looking but meaningless 8 bytes.
func TestAccountDiscriminatorDiffersFromInstruction(t *testing.T) {
	if AccountDiscriminator("Escrow") == Discriminator("escrow") {
		t.Fatal("account and instruction discriminators collided")
	}
}

func TestInitEscrowData(t *testing.T) {
	res, nonce, issuer := pk(0x44), pk(0x55), pk(0x66)
	data := InitEscrowData(1_000_000, res, nonce, issuer)

	if len(data) != 8+8+32+32+32 {
		t.Fatalf("length = %d, want 112", len(data))
	}
	d := Discriminator(IxInitEscrow)
	if string(data[:8]) != string(d[:]) {
		t.Error("discriminator prefix is wrong")
	}
	if got := binary.LittleEndian.Uint64(data[8:16]); got != 1_000_000 {
		t.Errorf("amount = %d, want 1000000", got)
	}
	if string(data[16:48]) != string(res[:]) {
		t.Error("resource_id is misplaced")
	}
	if string(data[48:80]) != string(nonce[:]) {
		t.Error("nonce is misplaced")
	}
	// The pinned issuer is appended LAST, matching the Rust argument order
	// (amount, resource_id, nonce, issuer). Borsh has no field names on the wire:
	// swap two 32-byte arguments and the program happily pins the wrong key.
	if string(data[80:112]) != string(issuer[:]) {
		t.Error("pinned issuer is misplaced")
	}
}

// TestInitEscrowDataDistinguishesArguments is the guard the length check cannot
// give: three 32-byte arguments in a row means a transposition is invisible to a
// size assertion. Pinning the wrong key here is silent on-chain — the escrow
// simply becomes releasable by a key the payer never chose.
func TestInitEscrowDataDistinguishesArguments(t *testing.T) {
	a := InitEscrowData(1, pk(0x01), pk(0x02), pk(0x03))
	b := InitEscrowData(1, pk(0x01), pk(0x03), pk(0x02)) // nonce/issuer swapped
	if string(a) == string(b) {
		t.Fatal("nonce and issuer are interchangeable in the encoding")
	}
}

func TestProposeAdminData(t *testing.T) {
	newAdmin := pk(0xAB)
	data := ProposeAdminData(newAdmin)
	if len(data) != 40 {
		t.Fatalf("length = %d, want 40", len(data))
	}
	d := Discriminator(IxProposeAdmin)
	if string(data[:8]) != string(d[:]) {
		t.Error("discriminator prefix is wrong")
	}
	if string(data[8:]) != string(newAdmin[:]) {
		t.Error("new_admin pubkey is misplaced")
	}
}

// TestArgumentlessInstructions states the property plainly: release_with_proof
// takes no caller-controlled input at all. Every input to the release decision
// is either already on chain or comes from the precompile.
func TestArgumentlessInstructions(t *testing.T) {
	for name, data := range map[string][]byte{
		IxReleaseWithProof: ReleaseWithProofData(),
		IxRefundExpired:    RefundExpiredData(),
		IxInitConfig:       InitConfigData(),
		IxAcceptAdmin:      AcceptAdminData(),
	} {
		if len(data) != 8 {
			t.Errorf("%s: length = %d, want 8 (discriminator only)", name, len(data))
		}
		d := Discriminator(name)
		if string(data) != string(d[:]) {
			t.Errorf("%s: payload is not the bare discriminator", name)
		}
	}
}

func TestIssuerAdminData(t *testing.T) {
	issuer := pk(0xEE)
	for name, data := range map[string][]byte{
		IxAddIssuer:    AddIssuerData(issuer),
		IxRemoveIssuer: RemoveIssuerData(issuer),
	} {
		if len(data) != 40 {
			t.Errorf("%s: length = %d, want 40", name, len(data))
		}
		if string(data[8:]) != string(issuer[:]) {
			t.Errorf("%s: issuer pubkey is misplaced", name)
		}
	}
}
