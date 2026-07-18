# M2 — Real USDC settlement on Solana devnet

**What it proves:** the §6.4 settle guard runs against a *real* Solana USDC
`TransferChecked` before signing. Only a transfer that pays the **bound**
recipient / asset / amount under the **payer's authority** is signed and sent —
otherwise the tool refuses to sign and no funds move.

## Prerequisites

- Go, plus the Solana SDK: `go get github.com/gagliardetto/solana-go`
- A devnet keypair (e.g. `~/.config/solana/id.json` from `solana-keygen new`),
  with a little devnet SOL for fees: `solana airdrop 1 --url devnet`
- Devnet USDC in that wallet — Circle faucet: https://faucet.circle.com
  (select **Solana devnet**; 20 USDC per 2h). Mint:
  `4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU`, 6 decimals.

## Run

```sh
# 0.10 USDC to yourself (simplest reproducible check)
go run -tags devnet ./cmd/paydevnet -amount 100000

# or to a specific merchant wallet
go run -tags devnet ./cmd/paydevnet -to <merchant_wallet> -amount 100000
```

The `devnet` build tag keeps this key/network path out of the default
`go test ./...`, so the core packages stay buildable and green without the SDK.

## What happens

1. Loads your keypair (the key stays in the file — never an env var, never
   printed, never committed).
2. Derives your USDC associated token account (source) and the merchant's (dest).
3. Builds the real `TransferChecked` (source → dest, micro-USDC, 6 decimals).
4. **`settle.AssertMatches` decodes the *actual* instruction and refuses to sign**
   unless it pays the bound recipient / asset / amount under your authority.
5. Signs, sends to devnet, prints the explorer link.

## Notes

- USDC mint + SPL program-id bytes are frozen and unit-tested
  (`settle/spl_test.go`), cross-checked against an independent base58 decode.
- The signing key never leaves the keypair file. Mainnet is intentionally not
  wired here.
- To see the guard refuse a bad transfer, the unit tests in `settle/` cover every
  mismatch (wrong recipient/asset/amount/authority); the same `AssertMatches`
  runs on the real instruction here.
