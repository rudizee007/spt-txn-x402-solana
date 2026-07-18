// Command x402demo runs the SPT-Txn x402 enforcement loop over a real HTTP 402
// exchange (an in-process httptest server), printing the four moments a judge
// should see: in-scope release, replay refused, over-scope denied, and a
// tampered payment aborted before signing.
//
//	go run ./cmd/x402demo
package main

import (
	"fmt"
	"net/http/httptest"
	"time"

	"github.com/rudizee007/spt-txn-x402-solana/demo"
	"github.com/rudizee007/spt-txn-x402-solana/gate"
)

func seed(b byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = b
	}
	return a
}

func main() {
	acc := demo.NewAccounts()
	now := time.Unix(1_700_000_000, 0)
	scope := demo.Scope{
		Ceiling: 5_000_000,
		Asset:   acc.Asset,
		Payees:  map[[32]byte]bool{acc.Merchant: true},
	}

	server := demo.NewResourceServer(acc, 1_000_000, "https://api.example.com/premium")
	ts := httptest.NewServer(demo.NewHTTPHandler(server))
	defer ts.Close()
	url := ts.URL + "/resource"

	fmt.Println("SPT-Txn × x402 — real HTTP 402 flow")
	fmt.Println("server:", ts.URL, " (GET returns 402 + PaymentRequirements; pay via X-PAYMENT header)")
	fmt.Println()

	c := demo.NewHTTPClient(acc, scope)
	t1 := gate.Token{Nonce: seed(0x5A), Expiry: now.Add(time.Minute)}
	o, _ := c.Pay(url, t1, now, false)
	fmt.Printf("[1] GET in-scope     -> %-15s paid=%-5v %q\n", o.Decision, o.Paid, o.Released)

	o, _ = c.Pay(url, t1, now, false)
	fmt.Printf("[2] GET replay       -> %-15s %s\n", o.Decision, o.Reason)

	pricey := demo.NewResourceServer(acc, 10_000_000, "https://api.example.com/premium")
	tsHi := httptest.NewServer(demo.NewHTTPHandler(pricey))
	defer tsHi.Close()
	c2 := demo.NewHTTPClient(acc, scope)
	t2 := gate.Token{Nonce: seed(0x6B), Expiry: now.Add(time.Minute)}
	o, _ = c2.Pay(tsHi.URL+"/resource", t2, now, false)
	fmt.Printf("[3] GET over-scope   -> %-15s %s\n", o.Decision, o.Reason)

	c3 := demo.NewHTTPClient(acc, scope)
	t3 := gate.Token{Nonce: seed(0x7C), Expiry: now.Add(time.Minute)}
	o, _ = c3.Pay(url, t3, now, true)
	fmt.Printf("[4] GET tampered     -> %-15s aborted-before-sign=%-5v %s\n", o.Decision, o.AbortedBeforeSign, o.Reason)
}
