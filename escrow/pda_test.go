package escrow

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

// TestIsOnCurveAcceptsRealKeys is the positive control: every Ed25519 public key
// produced by the standard library is by construction a valid curve point, so
// IsOnCurve must accept all of them. A decompression bug that rejected valid
// points would make FindProgramAddress return an address a real keypair could
// sign for — a silent loss of the PDA's whole security property.
func TestIsOnCurveAcceptsRealKeys(t *testing.T) {
	for i := 0; i < 256; i++ {
		pub, _, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		var b [32]byte
		copy(b[:], pub)
		if !IsOnCurve(b) {
			t.Fatalf("rejected a genuine ed25519 public key: %x", pub)
		}
	}
}

// TestIsOnCurveRejectsAboutHalf is the negative control. Roughly half of all
// 32-byte strings decode to a curve point, because x² is a quadratic residue
// about half the time. A function that always returned true (or always false)
// would pass the positive test above but fail here.
func TestIsOnCurveRejectsAboutHalf(t *testing.T) {
	const n = 1024
	on := 0
	for i := 0; i < n; i++ {
		var b [32]byte
		if _, err := rand.Read(b[:]); err != nil {
			t.Fatal(err)
		}
		if IsOnCurve(b) {
			on++
		}
	}
	// Generous bounds: this is a sanity check on the shape of the answer, not
	// a statistical test.
	if on < n/4 || on > 3*n/4 {
		t.Fatalf("on-curve rate looks wrong: %d/%d", on, n)
	}
}

func TestFindProgramAddressIsDeterministic(t *testing.T) {
	seeds := [][]byte{[]byte(SeedConfig)}
	a1, b1, err := FindProgramAddress(seeds, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	a2, b2, err := FindProgramAddress(seeds, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	if a1 != a2 || b1 != b2 {
		t.Fatal("PDA derivation is not deterministic")
	}
	if IsOnCurve(a1) {
		t.Fatal("derived PDA is on the curve — a private key could exist for it")
	}
}

// TestFindProgramAddressUsesHighestBump documents the descending search: the
// canonical PDA is the first off-curve candidate counting down from 255, and
// every candidate above the returned bump must be on the curve.
func TestFindProgramAddressUsesHighestBump(t *testing.T) {
	seed1 := pk(1)
	seeds := [][]byte{[]byte(SeedEscrow), seed1[:]}
	addr, bump, err := FindProgramAddress(seeds, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	if createProgramAddress(seeds, bump, EscrowProgramID) != addr {
		t.Fatal("returned address does not match its own bump")
	}
	for b := 255; b > int(bump); b-- {
		if !IsOnCurve(createProgramAddress(seeds, uint8(b), EscrowProgramID)) {
			t.Fatalf("bump %d was off-curve but skipped; %d was returned", b, bump)
		}
	}
}

// TestEscrowPDACommitsToBinding is the structural half of the anti-substitution
// argument: change any binding bit and the escrow lives somewhere else entirely,
// so a release aimed at the wrong parameters cannot reach a real escrow.
func TestEscrowPDACommitsToBinding(t *testing.T) {
	payer, recip := pk(1), pk(2)
	b1 := pk(3)
	b2 := pk(3)
	b2[0] ^= 1

	a1, _, err := EscrowPDA(payer, recip, b1, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	a2, _, err := EscrowPDA(payer, recip, b2, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	if a1 == a2 {
		t.Fatal("escrow address does not depend on the binding")
	}

	// Same for the payer and the recipient, both of which are seeds.
	a3, _, err := EscrowPDA(pk(9), recip, b1, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	if a3 == a1 {
		t.Fatal("escrow address does not depend on the payer")
	}
}

func TestAllDerivedAccountsAreOffCurve(t *testing.T) {
	binding := pk(0x5A)
	cfg, _, err := ConfigPDA(EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	esc, _, err := EscrowPDA(pk(1), pk(2), binding, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	vault, _, err := VaultPDA(esc, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	spent, _, err := SpentPDA(binding, EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	ata, _, err := AssociatedTokenAddress(pk(2), pk(7))
	if err != nil {
		t.Fatal(err)
	}

	for name, a := range map[string][32]byte{
		"config": cfg, "escrow": esc, "vault": vault, "spent": spent, "ata": ata,
	} {
		if IsOnCurve(a) {
			t.Errorf("%s PDA is on the curve", name)
		}
	}

	// All five must be distinct; a collision would mean two roles share an
	// account.
	seen := map[[32]byte]string{}
	for name, a := range map[string][32]byte{
		"config": cfg, "escrow": esc, "vault": vault, "spent": spent, "ata": ata,
	} {
		if prev, dup := seen[a]; dup {
			t.Errorf("%s and %s derived to the same address", name, prev)
		}
		seen[a] = name
	}
}
