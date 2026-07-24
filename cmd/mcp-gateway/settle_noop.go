//go:build !devnet

package main

// Default build: the server authorizes but moves no funds. Build with
// -tags devnet to perform a real Solana devnet USDC transfer on ALLOW.

// settlePayment is a no-op here; it returns an empty signature so the caller
// reports "authorized (no funds moved)".
func settlePayment(_ string, _ uint64) (string, error) { return "", nil }

// demoMerchant is the approved recipient in the default build — a placeholder
// address used for authorization only (no real on-chain account).
func demoMerchant() string { return addr(0x11) }
