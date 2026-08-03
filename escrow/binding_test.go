package escrow

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func pk(b byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = b
	}
	return out
}

// TestBindingKnownAnswer is the cross-language pin against the Rust verifier.
//
// The expected digest is copied verbatim from
// programs/spt_x402_escrow/src/verify.rs::binding_known_answer_vector in the
// spt-txn-x402-escrow repository, for the same inputs. If this test fails, the Go
// issuer and the on-chain verifier disagree about what an authorization *is* —
// which is THREAT-MODEL T1, the authorization-bypass class. Do not "fix" it by
// updating the constant; find out which side moved.
func TestBindingKnownAnswer(t *testing.T) {
	const want = "3a08cc7c2ac1c8061262c2c901b95e77fae75fc918955acceb4d4bd17b8444a4"

	got := ComputeBinding(BindingParams{
		Payer:      pk(0x11),
		Mint:       pk(0x22),
		Amount:     1_000_000,
		Recipient:  pk(0x33),
		ResourceID: pk(0x44),
		Nonce:      pk(0x55),
	})
	if h := hex.EncodeToString(got[:]); h != want {
		t.Fatalf("binding diverged from the Rust verifier\n got: %s\nwant: %s", h, want)
	}
}

// TestBindingIsInstanceUnique is the regression for the cross-escrow replay found
// in adversarial review (THREAT-MODEL T4): every field must move the binding, and
// in particular Payer and Nonce, whose absence once let a single attestation
// release every escrow sharing (mint, amount, recipient, resource).
func TestBindingIsInstanceUnique(t *testing.T) {
	base := BindingParams{
		Payer:      pk(1),
		Mint:       pk(0x22),
		Amount:     100,
		Recipient:  pk(0x33),
		ResourceID: pk(0x44),
		Nonce:      pk(0x55),
	}
	want := ComputeBinding(base)

	if ComputeBinding(base) != want {
		t.Fatal("binding is not deterministic")
	}

	mutations := map[string]func(*BindingParams){
		"payer":     func(p *BindingParams) { p.Payer = pk(2) },
		"mint":      func(p *BindingParams) { p.Mint = pk(0x23) },
		"amount":    func(p *BindingParams) { p.Amount = 101 },
		"recipient": func(p *BindingParams) { p.Recipient = pk(0x34) },
		"resource":  func(p *BindingParams) { p.ResourceID = pk(0x45) },
		"nonce":     func(p *BindingParams) { p.Nonce = pk(0x56) },
	}
	for name, mutate := range mutations {
		m := base
		mutate(&m)
		if ComputeBinding(m) == want {
			t.Errorf("changing %s did not change the binding", name)
		}
	}
}

// TestBindingAmountIsLittleEndianU64 pins the field width. A u128 here — copied
// across from the gate binding, which does use 16 bytes — would silently
// desynchronize every binding from the verifier.
func TestBindingAmountIsLittleEndianU64(t *testing.T) {
	var p BindingParams
	p.Amount = 1

	h := sha256.New()
	h.Write([]byte(DomainTagEscrow))
	h.Write([]byte{0x00, LayoutVersion})
	h.Write(p.Payer[:])
	h.Write(p.Mint[:])
	h.Write([]byte{1, 0, 0, 0, 0, 0, 0, 0}) // 8 bytes, little-endian
	h.Write(p.Recipient[:])
	h.Write(p.ResourceID[:])
	h.Write(p.Nonce[:])

	var want [32]byte
	copy(want[:], h.Sum(nil))

	if ComputeBinding(p) != want {
		t.Fatal("amount is not encoded as 8-byte little-endian u64")
	}
}

// TestResourceIDIsPlainSHA256 keeps the resource hash identical to the gate's, so
// the same resource string maps to the same 32 bytes on both sides.
func TestResourceIDIsPlainSHA256(t *testing.T) {
	const res = "https://api.example.com/v1/quote?symbol=SOL"
	if ResourceID(res) != sha256.Sum256([]byte(res)) {
		t.Fatal("ResourceID is not plain SHA-256 of the resource string")
	}
}

// TestResourceIDIsByteExact documents that no normalization happens. Any
// trimming or case folding would be a place the issuer and a verifier could
// disagree.
func TestResourceIDIsByteExact(t *testing.T) {
	if ResourceID("https://a/b") == ResourceID("https://a/b ") {
		t.Error("trailing space was normalized away")
	}
	if ResourceID("https://A/b") == ResourceID("https://a/b") {
		t.Error("case was normalized away")
	}
}

// TestDomainTagsAreDistinct guards the property that makes the two constructions
// non-substitutable.
func TestDomainTagsAreDistinct(t *testing.T) {
	if DomainTagEscrow == DomainTagAttest {
		t.Fatal("escrow binding and attestation share a domain tag")
	}
	if len(DomainTagAttest) != 30 {
		t.Fatalf("attestation tag length changed: got %d, want 30", len(DomainTagAttest))
	}
	if len(DomainTagEscrow) != 30 {
		t.Fatalf("escrow tag length changed: got %d, want 30", len(DomainTagEscrow))
	}
}
