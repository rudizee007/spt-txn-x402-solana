// Command mcp-demo scripts an AI agent making tool-calls through the SPT-Txn MCP
// policy-enforcement point. The human approved the agent for exactly one payment;
// the enforcer allows that one call, refuses every hijacked / prompt-injected
// variant, refuses a replay, and refuses an expired authorization — emitting a
// signed receipt on every decision, then printing the transparency-log root.
//
// It uses the SAME gate + receipt core as the HTTP x402 PEP (package gateway).
//
//	go run ./cmd/mcp-demo
package main

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
	"github.com/rudizee007/spt-txn-x402-solana/mcpgate"
	"github.com/rudizee007/spt-txn-x402-solana/receipt"
)

func b32(x byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = x
	}
	return a
}

func addr(x byte) string {
	a := b32(x)
	return gate.EncodeBase58(a[:])
}

func main() {
	now := time.Unix(1_700_000_000, 0)
	_, rk, _ := ed25519.GenerateKey(nil)
	log := receipt.NewLog(rk.Public().(ed25519.PublicKey))

	usdc := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	merchant := addr(0x11)
	attacker := addr(0xEE)

	e := &mcpgate.Enforcer{
		Scheme:    "exact",
		Network:   "solana:devnet",
		Allowlist: gate.Allowlist{Schemes: map[string]byte{"exact": 1}, Networks: map[string]byte{"solana:devnet": 2}},
		Policy:    mcpgate.ExactPayment{Asset: usdc, PayTo: merchant, Resource: "invoice:42", MaxAmount: 1_000_000},
		Spend:     gate.NewMemSpendLog(),
		Log:       log,
		RKey:      rk,
		Now:       func() time.Time { return now },
	}

	fmt.Println("SPT-Txn — MCP payment enforcement demo (public reference)")
	fmt.Println()
	fmt.Println("An AI agent has a make_payment tool. The human approved it for exactly:")
	fmt.Println("  pay  <= 1.000000 USDC  to the merchant  for  invoice:42")
	fmt.Println("Every tool-call is enforced by the same gate as the HTTP x402 PEP.")
	fmt.Println()

	call := func(label string, c mcpgate.ToolCall) {
		r := e.Authorize(c)
		if r.Allowed() {
			fmt.Printf("  ALLOW   %-46s  receipt %s\n", label, r.Receipt)
		} else {
			fmt.Printf("  DENY    %-46s  %s\n", label, r.Reason)
		}
	}

	legit := func(nonce byte) mcpgate.ToolCall {
		return mcpgate.ToolCall{
			To: merchant, Asset: usdc, Amount: "1000000", Resource: "invoice:42",
			Nonce: b32(nonce), Expiry: now.Add(time.Minute),
		}
	}

	fmt.Println("The agent makes tool-calls; the enforcement point decides:")
	fmt.Println()

	call("make_payment(merchant, 1 USDC, invoice:42)", legit(0x01))

	h1 := legit(0x02)
	h1.To = attacker
	call("make_payment(ATTACKER, 1 USDC, invoice:42)", h1)

	h2 := legit(0x03)
	h2.Amount = "1000000000"
	call("make_payment(merchant, 1000 USDC, invoice:42)", h2)

	h3 := legit(0x04)
	h3.Resource = "invoice:999"
	call("make_payment(merchant, 1 USDC, invoice:999)", h3)

	call("make_payment replay (reused authorization)", legit(0x01))

	ex := legit(0x05)
	ex.Expiry = now.Add(-time.Minute)
	call("make_payment after the approved window expired", ex)

	root := log.Root()
	fmt.Println()
	fmt.Printf("Every decision emitted a signed receipt. Transparency-log Merkle root:\n  %x\n", root[:])
	fmt.Println()
	fmt.Println("The agent can make exactly one payment — the one the human approved.")
	fmt.Println("A prompt-injected or hijacked tool-call is cryptographically refused,")
	fmt.Println("with a tamper-evident audit trail as a byproduct.")
}
