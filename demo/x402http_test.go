package demo

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
)

func httpFixture(price uint64) (*httptest.Server, *HTTPClient, time.Time) {
	acc := NewAccounts()
	server := NewResourceServer(acc, price, "https://api.example.com/premium")
	ts := httptest.NewServer(NewHTTPHandler(server))
	scope := Scope{Ceiling: 5_000_000, Asset: acc.Asset, Payees: map[[32]byte]bool{acc.Merchant: true}}
	return ts, NewHTTPClient(acc, scope), time.Unix(1_700_000_000, 0)
}

// Real HTTP round-trip: 402 → gate ALLOW → settle → X-PAYMENT → 200 released.
func TestHTTP_AllowReleases(t *testing.T) {
	ts, c, now := httpFixture(1_000_000)
	defer ts.Close()
	out, err := c.Pay(ts.URL+"/resource", gate.Token{Nonce: fill(0x5A), Expiry: now.Add(time.Minute)}, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Decision != gate.Allow || !out.Paid || out.Released == "" {
		t.Fatalf("expected ALLOW + paid + released, got %+v", out)
	}
}

// Over-scope: gate DENYs after reading the 402; no X-PAYMENT is ever sent.
func TestHTTP_OverScopeDenied(t *testing.T) {
	ts, c, now := httpFixture(10_000_000)
	defer ts.Close()
	out, err := c.Pay(ts.URL+"/resource", gate.Token{Nonce: fill(0x6B), Expiry: now.Add(time.Minute)}, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Decision != gate.DenyViolation || out.Paid {
		t.Fatalf("expected DENY_VIOLATION and no payment, got %+v", out)
	}
}

// Tamper: gate allows the legit requirement, but the settle guard aborts before
// signing, so no X-PAYMENT is sent.
func TestHTTP_TamperAbortsBeforeSign(t *testing.T) {
	ts, c, now := httpFixture(1_000_000)
	defer ts.Close()
	out, err := c.Pay(ts.URL+"/resource", gate.Token{Nonce: fill(0x7C), Expiry: now.Add(time.Minute)}, now, true)
	if err != nil {
		t.Fatal(err)
	}
	if !out.AbortedBeforeSign || out.Paid {
		t.Fatalf("expected abort-before-sign and no payment, got %+v", out)
	}
}

// Replay: the same token cannot pay twice over HTTP either.
func TestHTTP_ReplayDenied(t *testing.T) {
	ts, c, now := httpFixture(1_000_000)
	defer ts.Close()
	tok := gate.Token{Nonce: fill(0x5A), Expiry: now.Add(time.Minute)}
	if out, _ := c.Pay(ts.URL+"/resource", tok, now, false); out.Decision != gate.Allow {
		t.Fatalf("first payment should ALLOW")
	}
	out, _ := c.Pay(ts.URL+"/resource", tok, now, false)
	if out.Decision != gate.DenyViolation || out.Paid {
		t.Fatalf("replay should be DENY_VIOLATION and no payment, got %+v", out)
	}
}

// Defense in depth: even if a payment reaches the server (bypassing the client
// gate), the server's own facilitator check rejects a transfer that does not pay
// its demand — here, a payment to the attacker → 402, resource withheld.
func TestHTTP_ServerRejectsTamperedPaymentIndependently(t *testing.T) {
	acc := NewAccounts()
	server := NewResourceServer(acc, 1_000_000, "https://api.example.com/premium")
	ts := httptest.NewServer(NewHTTPHandler(server))
	defer ts.Close()

	ixs := buildTransfer(acc.Source, acc.Asset, acc.Attacker, acc.Payer, 1_000_000)
	payload, _ := json.Marshal(paymentJSON{
		X402Version:  1,
		Scheme:       "exact",
		Network:      "solana:devnet",
		Instructions: ixsToJSON(ixs),
		Signed:       true,
	})
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/resource", nil)
	req.Header.Set("X-PAYMENT", base64.StdEncoding.EncodeToString(payload))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("server must reject a tampered payment with 402, got %d", resp.StatusCode)
	}
}
