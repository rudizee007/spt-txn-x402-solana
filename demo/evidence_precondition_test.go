package demo

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rudizee007/spt-txn-pep/gate"
	"github.com/rudizee007/spt-txn-pep/translog"
)

// unwritableLog returns a log that cannot accept an append: it verifies against
// a public key unrelated to the key the client signs with, so Append's
// self-verify fails on every call. This is the observable shape of a
// misconfigured or mis-rotated receipt key.
func unwritableLog(t *testing.T) *translog.Log {
	t.Helper()
	otherPub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return translog.NewLog(otherPub)
}

// An in-scope payment whose decision cannot be recorded must not settle. The
// signed chain is the product's evidence; paying without it produces a transfer
// no receipt accounts for.
func TestPay_UnrecordableDecisionDoesNotPay(t *testing.T) {
	server, c, now := fixture(1_000_000)
	c.receipts = unwritableLog(t)

	out := c.Pay(server, tok(now, 0x5A), now, false)

	if out.Paid {
		t.Error("a payment whose decision could not be recorded must not be made")
	}
	if out.Released != "" {
		t.Errorf("resource released (%q) for a decision that was never recorded", out.Released)
	}
	if out.Decision != gate.DenyUnavailable {
		t.Errorf("Decision = %v, want DenyUnavailable (the evidence path, not the policy, failed)", out.Decision)
	}
}

// Same guarantee over the real HTTP transport.
func TestHTTPPay_UnrecordableDecisionDoesNotPay(t *testing.T) {
	ts, c, now := httpFixture(1_000_000)
	defer ts.Close()
	c.receipts = unwritableLog(t)

	out, err := c.Pay(ts.URL+"/resource", tok(now, 0x5B), now, false)
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if out.Paid {
		t.Error("a payment whose decision could not be recorded must not be made")
	}
	if out.Decision != gate.DenyUnavailable {
		t.Errorf("Decision = %v, want DenyUnavailable", out.Decision)
	}
}

// A policy denial stays a DenyViolation when its evidence cannot be recorded, so
// the evidence path can never be broken to turn a violation into an outage (or to
// stop the client refusing at all).
//
// Asserting the CLASS, not just !Paid, is what gives this test teeth: Paid is
// false either way, so a test that checked only Paid would still pass if the
// `&& d.Class == gate.Allow` guard were deleted.
func TestPay_UnrecordableDecisionPreservesDenialClass(t *testing.T) {
	server, c, now := fixture(9_000_000) // price above the 5 USDC ceiling
	c.receipts = unwritableLog(t)

	out := c.Pay(server, tok(now, 0x5C), now, false)
	if out.Paid {
		t.Fatal("an out-of-scope payment must stay denied when evidence cannot be recorded")
	}
	if out.Decision != gate.DenyViolation {
		t.Errorf("Decision = %v, want DenyViolation: a policy violation must not be relabelled as an outage", out.Decision)
	}
}

// Same guarantee over the HTTP transport, which had no denial-side test at all.
func TestHTTPPay_UnrecordableDecisionPreservesDenialClass(t *testing.T) {
	ts, c, now := httpFixture(9_000_000)
	defer ts.Close()
	c.receipts = unwritableLog(t)

	out, err := c.Pay(ts.URL+"/resource", tok(now, 0x5D), now, false)
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if out.Paid {
		t.Fatal("an out-of-scope payment must stay denied when evidence cannot be recorded")
	}
	if out.Decision != gate.DenyViolation {
		t.Errorf("Decision = %v, want DenyViolation", out.Decision)
	}
}

// countingServer wraps the resource handler and counts the settlements the SERVER
// actually performed — an oracle independent of the Outcome the client builds for
// itself. A payment carries an X-PAYMENT header; a settled response carries
// X-PAYMENT-RESPONSE: settled.
type countingServer struct {
	inner    http.Handler
	attempts int // requests that presented a payment
	settled  int // responses in which the server released the resource
}

func (c *countingServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-PAYMENT") != "" {
		c.attempts++
	}
	rec := httptest.NewRecorder()
	c.inner.ServeHTTP(rec, r)
	if rec.Header().Get("X-PAYMENT-RESPONSE") == "settled" {
		c.settled++
	}
	for k, v := range rec.Header() {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.Code)
	_, _ = w.Write(rec.Body.Bytes())
}

// The evidence entry must be recorded BEFORE the payment goes out, not merely
// reported as a refusal afterwards.
//
// out.Paid and out.Released are not evidence that nothing settled — they are what
// the same return statement says. This asserts against the server instead, so a
// refusal placed after the paid round-trip fails here even though the Outcome
// still reads DenyUnavailable / Paid=false.
func TestHTTPPay_UnrecordableDecisionNeverReachesTheMerchant(t *testing.T) {
	acc := NewAccounts()
	oracle := &countingServer{inner: NewHTTPHandler(NewResourceServer(acc, 1_000_000, "https://api.example.com/premium"))}
	ts := httptest.NewServer(oracle)
	defer ts.Close()

	scope := Scope{Ceiling: 5_000_000, Asset: acc.Asset, Payees: map[[32]byte]bool{acc.Merchant: true}}
	c := NewHTTPClient(acc, scope)
	c.receipts = unwritableLog(t)
	now := time.Unix(1_700_000_000, 0)

	out, err := c.Pay(ts.URL+"/resource", tok(now, 0x5E), now, false)
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}

	if oracle.attempts != 0 {
		t.Errorf("the client sent %d payment(s) for a decision it could not record", oracle.attempts)
	}
	if oracle.settled != 0 {
		t.Errorf("THE MERCHANT WAS PAID %d time(s) while the client reported %v", oracle.settled, out.Decision)
	}
}

// The oracle must be able to see a real settlement, or the assertions above would
// pass for the wrong reason.
func TestHTTPPay_OracleObservesARealSettlement(t *testing.T) {
	acc := NewAccounts()
	oracle := &countingServer{inner: NewHTTPHandler(NewResourceServer(acc, 1_000_000, "https://api.example.com/premium"))}
	ts := httptest.NewServer(oracle)
	defer ts.Close()

	scope := Scope{Ceiling: 5_000_000, Asset: acc.Asset, Payees: map[[32]byte]bool{acc.Merchant: true}}
	c := NewHTTPClient(acc, scope)
	now := time.Unix(1_700_000_000, 0)

	if _, err := c.Pay(ts.URL+"/resource", tok(now, 0x5F), now, false); err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if oracle.settled != 1 {
		t.Fatalf("oracle counted %d settlements on a healthy path, want 1 — the oracle is blind", oracle.settled)
	}
}
