package demo

import (
	"testing"
	"time"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
)

func fixture(price uint64) (*ResourceServer, *Client, time.Time) {
	acc := NewAccounts()
	server := NewResourceServer(acc, price, "https://api.example.com/data")
	scope := Scope{
		Ceiling: 5_000_000,
		Asset:   acc.Asset,
		Payees:  map[[32]byte]bool{acc.Merchant: true},
	}
	return server, NewClient(acc, scope), time.Unix(1_700_000_000, 0)
}

func tok(now time.Time, n byte) gate.Token {
	return gate.Token{Nonce: fill(n), Expiry: now.Add(time.Minute)}
}

// In-scope payment: gate ALLOWs, settle passes, server releases the resource.
func TestEndToEnd_AllowReleases(t *testing.T) {
	server, c, now := fixture(1_000_000)
	out := c.Pay(server, tok(now, 0x5A), now, false)
	if out.Decision != gate.Allow || !out.Paid || out.Released == "" {
		t.Fatalf("expected ALLOW + paid + released, got %+v", out)
	}
}

// Over-scope payment: gate DENYs (violation) before any signing, no payment.
func TestEndToEnd_OverScopeDenied(t *testing.T) {
	server, c, now := fixture(10_000_000) // above the 5,000,000 ceiling
	out := c.Pay(server, tok(now, 0x6B), now, false)
	if out.Decision != gate.DenyViolation || out.Paid {
		t.Fatalf("expected DENY_VIOLATION and no payment, got %+v", out)
	}
}

// Tampered destination: gate ALLOWs the legitimate requirement, but the client's
// constructed transfer pays the attacker — the §6.4 settle guard aborts BEFORE
// signing, so nothing is sent.
func TestEndToEnd_TamperCaughtBeforeSign(t *testing.T) {
	server, c, now := fixture(1_000_000)
	out := c.Pay(server, tok(now, 0x7C), now, true)
	if !out.AbortedBeforeSign || out.Paid {
		t.Fatalf("expected abort-before-sign and no payment, got %+v", out)
	}
}

// Replay: the same token cannot pay twice (nonce spent on the first ALLOW).
func TestEndToEnd_ReplayDenied(t *testing.T) {
	server, c, now := fixture(1_000_000)
	first := c.Pay(server, tok(now, 0x5A), now, false)
	if first.Decision != gate.Allow {
		t.Fatalf("first payment should ALLOW, got %+v", first)
	}
	second := c.Pay(server, tok(now, 0x5A), now, false)
	if second.Decision != gate.DenyViolation || second.Paid {
		t.Fatalf("replay should be DENY_VIOLATION and no payment, got %+v", second)
	}
}
