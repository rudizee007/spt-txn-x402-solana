package escrow

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// TestDiscriminators pins all six instruction discriminators the escrow program
// exposes. These were computed independently (SHA-256 of "global:"+name, first 8
// bytes) and checked against the program's IDL.
func TestDiscriminators(t *testing.T) {
	want := map[string]string{
		IxInitConfig:       "17eb73e8a86001e7",
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
	res, nonce := pk(0x44), pk(0x55)
	data := InitEscrowData(1_000_000, res, nonce)

	if len(data) != 8+8+32+32 {
		t.Fatalf("length = %d, want 80", len(data))
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
}

// TestArgumentlessInstructions states the property plainly: release_with_proof
// takes no caller-controlled input at all. Every input to the release decision
// is either already on chain or comes from the precompile.
func TestArgumentlessInstructions(t *testing.T) {
	for name, data := range map[string][]byte{
		IxReleaseWithProof: ReleaseWithProofData(),
		IxRefundExpired:    RefundExpiredData(),
		IxInitConfig:       InitConfigData(),
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
