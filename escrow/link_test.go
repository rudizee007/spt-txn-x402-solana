package escrow

import (
	"testing"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
)

func b58(k [32]byte) string { return gate.EncodeBase58(k[:]) }

// fixture builds a self-consistent gate PaymentRequirements: PayTo is the real
// associated token account of recipientWallet for mint, which is the invariant
// FromGate checks.
func fixture(t *testing.T) (al gate.Allowlist, pr gate.PaymentRequirements, payer, recipient, mint [32]byte) {
	t.Helper()
	payer, recipient, mint = pk(0x11), pk(0x22), pk(0x33)

	ata, _, err := AssociatedTokenAddress(recipient, mint)
	if err != nil {
		t.Fatal(err)
	}
	al = gate.Allowlist{
		Schemes:  map[string]byte{"exact": 1},
		Networks: map[string]byte{"solana:devnet": 1},
	}
	pr = gate.PaymentRequirements{
		Scheme:            "exact",
		Network:           "solana:devnet",
		Asset:             b58(mint),
		PayTo:             b58(ata),
		MaxAmountRequired: "1000000",
		Resource:          "https://api.example.com/v1/quote",
	}
	return
}

func TestFromGateHappyPath(t *testing.T) {
	al, pr, payer, recipient, mint := fixture(t)
	nonce := pk(0x55)

	p, err := FromGate(al, pr, payer, recipient, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if p.Payer != payer || p.Recipient != recipient || p.Mint != mint {
		t.Error("parties or mint were mapped incorrectly")
	}
	if p.Amount != 1_000_000 {
		t.Errorf("amount = %d, want 1000000", p.Amount)
	}
	if p.ResourceID != ResourceID(pr.Resource) {
		t.Error("resource id does not match the gate's resource string")
	}
	if p.Nonce != nonce {
		t.Error("nonce was not carried through")
	}
	if p.Binding != ComputeBinding(p.BindingParams) {
		t.Error("cached binding disagrees with its own parameters")
	}
	if b58(p.RecipientATA) != pr.PayTo {
		t.Error("derived recipient ATA is not the gate's PayTo")
	}
}

// TestGateAndEscrowBindingsDiffer is the point of having two domain tags: the
// same payment produces two different 32-byte values, so neither construction
// can ever be presented in place of the other.
func TestGateAndEscrowBindingsDiffer(t *testing.T) {
	al, pr, payer, recipient, _ := fixture(t)
	nonce := pk(0x55)

	gb, err := gate.ComputeBinding(al, pr, nonce)
	if err != nil {
		t.Fatal(err)
	}
	p, err := FromGate(al, pr, payer, recipient, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if gb == p.Binding {
		t.Fatal("gate and escrow bindings collided — domain separation is broken")
	}
}

// TestFromGateRejectsWrongRecipient is the guard against the most likely
// integration mistake: passing a wallet that does not own the PayTo token
// account. Left unchecked it produces an escrow whose recipient constraint can
// never be satisfied, stranding real funds until refund_expired.
func TestFromGateRejectsWrongRecipient(t *testing.T) {
	al, pr, payer, _, _ := fixture(t)
	if _, err := FromGate(al, pr, payer, pk(0x99), pk(0x55)); err != ErrRecipientMismatch {
		t.Fatalf("got %v, want ErrRecipientMismatch", err)
	}
}

// TestFromGateRejectsWalletAsPayTo covers the inverse confusion: the gate's
// PayTo holding a wallet address rather than that wallet's token account.
func TestFromGateRejectsWalletAsPayTo(t *testing.T) {
	al, pr, payer, recipient, _ := fixture(t)
	pr.PayTo = b58(recipient) // the wallet, not its ATA
	if _, err := FromGate(al, pr, payer, recipient, pk(0x55)); err != ErrRecipientMismatch {
		t.Fatalf("got %v, want ErrRecipientMismatch", err)
	}
}

// TestFromGateRejectsAmountAboveU64 is where the two amount widths meet. The
// gate binds a u128 because x402 permits it; SPL token amounts are u64.
// Truncating rather than refusing would move the wrong amount of money.
func TestFromGateRejectsAmountAboveU64(t *testing.T) {
	al, pr, payer, recipient, _ := fixture(t)
	pr.MaxAmountRequired = "18446744073709551616" // 2^64
	if _, err := FromGate(al, pr, payer, recipient, pk(0x55)); err != ErrAmountNotU64 {
		t.Fatalf("got %v, want ErrAmountNotU64", err)
	}

	pr.MaxAmountRequired = "18446744073709551615" // 2^64 - 1, the maximum
	p, err := FromGate(al, pr, payer, recipient, pk(0x55))
	if err != nil {
		t.Fatalf("u64 max should be accepted: %v", err)
	}
	if p.Amount != 1<<64-1 {
		t.Errorf("amount = %d, want 2^64-1", p.Amount)
	}
}

func TestFromGateRejectsZeroAmount(t *testing.T) {
	al, pr, payer, recipient, _ := fixture(t)
	pr.MaxAmountRequired = "0"
	if _, err := FromGate(al, pr, payer, recipient, pk(0x55)); err != ErrAmountZero {
		t.Fatalf("got %v, want ErrAmountZero", err)
	}
}

func TestDeriveIsStableAndBindingDriven(t *testing.T) {
	al, pr, payer, recipient, _ := fixture(t)

	p1, err := FromGate(al, pr, payer, recipient, pk(0x55))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := FromGate(al, pr, payer, recipient, pk(0x56)) // different nonce only
	if err != nil {
		t.Fatal(err)
	}

	a1, err := p1.Derive(EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := p2.Derive(EscrowProgramID)
	if err != nil {
		t.Fatal(err)
	}

	if a1.Config != a2.Config {
		t.Error("config PDA should be a singleton, independent of the escrow")
	}
	if a1.Escrow == a2.Escrow || a1.Vault == a2.Vault || a1.Spent == a2.Spent {
		t.Error("a different nonce must move the escrow, vault and spent marker")
	}
}

// TestAttestCarriesTheEscrowBinding closes the loop: the only attestation this
// package will build for a set of params is the one bound to those params.
func TestAttestCarriesTheEscrowBinding(t *testing.T) {
	al, pr, payer, recipient, _ := fixture(t)
	p, err := FromGate(al, pr, payer, recipient, pk(0x55))
	if err != nil {
		t.Fatal(err)
	}
	att := p.Attest(1_700_000_000)
	if att.Binding != p.Binding {
		t.Fatal("attestation is not bound to the escrow it was built from")
	}
	parsed, err := ParseAttestation(att.Marshal())
	if err != nil || parsed.Binding != p.Binding {
		t.Fatalf("attestation did not survive a round trip: %v", err)
	}
}
