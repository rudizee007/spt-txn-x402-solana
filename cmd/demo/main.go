// Command demo runs the SPT-Txn x402 enforcement loop end-to-end, in process,
// and prints the four moments a judge should see: an in-scope payment that is
// released, a replay that is refused, an over-scope payment denied before
// signing, and a tampered payment caught by the settle guard before signing.
//
//	go run ./cmd/demo
package main

import (
	"fmt"
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

	fmt.Println("SPT-Txn × x402 — authorization gate for agentic payments (in-process demo)")
	fmt.Println("ceiling = 5.000000 USDC, one merchant, single-use tokens")
	fmt.Println()

	// 1. In-scope payment → released.
	server := demo.NewResourceServer(acc, 1_000_000, "https://api.example.com/premium")
	c := demo.NewClient(acc, scope)
	t1 := gate.Token{Nonce: seed(0x5A), Expiry: now.Add(time.Minute)}
	o := c.Pay(server, t1, now, false)
	fmt.Printf("[1] in-scope 1.000000 USDC  -> %-15s paid=%-5v released=%q\n", o.Decision, o.Paid, o.Released)

	// 2. Replay the same token → refused.
	o = c.Pay(server, t1, now, false)
	fmt.Printf("[2] replay same token       -> %-15s %s\n", o.Decision, o.Reason)

	// 3. Over-scope payment → denied before signing.
	pricey := demo.NewResourceServer(acc, 10_000_000, "https://api.example.com/premium")
	t2 := gate.Token{Nonce: seed(0x6B), Expiry: now.Add(time.Minute)}
	o = c.Pay(pricey, t2, now, false)
	fmt.Printf("[3] over-scope 10.00 USDC   -> %-15s %s\n", o.Decision, o.Reason)

	// 4. Tampered destination → settle guard aborts before signing.
	t3 := gate.Token{Nonce: seed(0x7C), Expiry: now.Add(time.Minute)}
	o = c.Pay(server, t3, now, true)
	fmt.Printf("[4] tampered recipient      -> %-15s aborted-before-sign=%-5v %s\n", o.Decision, o.AbortedBeforeSign, o.Reason)

	// Evidence: every decision above emitted a signed, chained receipt.
	root := c.ReceiptRoot()
	fmt.Printf("\nevidence: %d signed receipts, merkle root %x\n", c.ReceiptCount(), root)
	fmt.Println("          anchor it on devnet with:  go run -tags devnet ./cmd/anchordevnet")
}
