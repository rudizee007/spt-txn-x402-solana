package demo

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rudizee007/spt-txn-pep/gate"
	"github.com/rudizee007/spt-txn-pep/receipt"
)

// httptestServer starts an x402 resource server over the given accounts at the
// given price and returns its base URL. Unlike httpFixture it takes the accounts
// from the caller, so several servers can share one merchant/asset set — which is
// what the over-scope decision needs (same payee, higher price).
func httptestServer(t *testing.T, acc Accounts, price uint64) string {
	t.Helper()
	ts := httptest.NewServer(NewHTTPHandler(NewResourceServer(acc, price, "https://api.example.com/premium")))
	t.Cleanup(ts.Close)
	return ts.URL
}

// fourDecisions drives the same four moments cmd/x402demo shows: in-scope
// release, replay refused, over-scope denied, tamper aborted before signing.
func fourDecisions(t *testing.T) *HTTPClient {
	t.Helper()
	acc := NewAccounts()
	now := time.Unix(1_700_000_000, 0)
	scope := Scope{Ceiling: 5_000_000, Asset: acc.Asset, Payees: map[[32]byte]bool{acc.Merchant: true}}

	ts := httptestServer(t, acc, 1_000_000)
	pricey := httptestServer(t, acc, 10_000_000)

	c := NewHTTPClient(acc, scope)
	tok := gate.Token{Nonce: fill(0x5A), Expiry: now.Add(time.Minute)}
	if _, err := c.Pay(ts+"/resource", tok, now, false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Pay(ts+"/resource", tok, now, false); err != nil { // replay
		t.Fatal(err)
	}
	if _, err := c.Pay(pricey+"/resource", gate.Token{Nonce: fill(0x6B), Expiry: now.Add(time.Minute)}, now, false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Pay(ts+"/resource", gate.Token{Nonce: fill(0x7C), Expiry: now.Add(time.Minute)}, now, true); err != nil {
		t.Fatal(err)
	}
	if c.ReceiptCount() != 4 {
		t.Fatalf("expected 4 receipts, got %d", c.ReceiptCount())
	}
	return c
}

// The root the demo prints and the root a later process reads back off disk must
// be the same value. This is the whole point of persisting the log: the on-chain
// anchor commits to the decisions that actually happened, not to a stand-in.
func TestSavedReceiptsCarryTheSameRoot(t *testing.T) {
	c := fourDecisions(t)
	path := filepath.Join(t.TempDir(), "receipts.json")
	if err := c.SaveReceipts(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := receipt.LoadLog(path)
	if err != nil {
		t.Fatalf("LoadLog: %v", err)
	}
	if loaded.Len() != c.ReceiptCount() {
		t.Fatalf("loaded %d receipts, demo made %d", loaded.Len(), c.ReceiptCount())
	}
	if loaded.Root() != c.ReceiptRoot() {
		t.Fatalf("anchored root %x != demo root %x", loaded.Root(), c.ReceiptRoot())
	}
}

// The Merkle root commits to the decisions, not to the key that signed them: two
// independent runs use different receipt-signing keys and must still produce the
// same root. Without this, "the root you saw is the root we anchored" would only
// hold within a single process.
func TestReceiptRootIsDeterministicAcrossRuns(t *testing.T) {
	a := fourDecisions(t)
	b := fourDecisions(t)
	if a.ReceiptRoot() != b.ReceiptRoot() {
		t.Fatalf("roots differ across runs: %x vs %x", a.ReceiptRoot(), b.ReceiptRoot())
	}
	// Different signing keys, same root.
	pa, pb := loadedPubKey(t, a), loadedPubKey(t, b)
	if string(pa) == string(pb) {
		t.Fatal("the two runs shared a receipt-signing key; the test proves nothing")
	}
}

func loadedPubKey(t *testing.T, c *HTTPClient) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "receipts.json")
	if err := c.SaveReceipts(path); err != nil {
		t.Fatal(err)
	}
	l, err := receipt.LoadLog(path)
	if err != nil {
		t.Fatal(err)
	}
	return l.PublicKey()
}

// A saved log is evidence, not scratch: it must not be world-readable.
func TestSavedReceiptsArePrivate(t *testing.T) {
	c := fourDecisions(t)
	path := filepath.Join(t.TempDir(), "receipts.json")
	if err := c.SaveReceipts(path); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %o, want 600", perm)
	}
}
