# `settle/` — Solana settlement client (isolated Go module)

Submits the Solana transfer from a gate **ALLOW** decision and writes the human
anchor **on-chain via the SPL Memo program** — visible to any explorer, with no
PII on the ledger.

This is an **isolated Go module** so the Solana SDK never enters the offline
authorization core. Starting point: `clients/sol-pay` from the reference
implementation, which already does this for **SOL on devnet**.

Hackathon delta:
- Extend from SOL → **USDC (SPL)**, the currency agents actually transact in.
- Wire the retry-with-proof leg of the real x402 protocol flow.

## Security
- Key material is read from `$SOL_OPERATOR_KEY` only (base58, JSON array, or a
  path to a `solana-keygen` file) — **never** a flag, **never** committed.
- **Devnet by default.** Mainnet requires `-network mainnet` **and** an explicit
  confirmation prompt.
- `-dry-run` derives the address and prints the transfer with no network call.

```
# devnet, dry run
SOL_OPERATOR_KEY=... go run . -to <merchant> -amount 1000000 -memo <humanAnchor> -dry-run
```
