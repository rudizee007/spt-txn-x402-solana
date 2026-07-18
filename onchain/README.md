# `onchain/` — Option 2: on-chain enforcement (Anchor / Rust)

**Not built yet.** This is the near-path target after Option 1's off-chain flow
lands (see `../docs/BUILD-PLAN.md`).

An Anchor program that:

1. Holds the x402 payment in escrow.
2. Verifies the SPT-Txn signature on-chain using Solana's **Ed25519 precompile**.
3. Reconstructs and checks the **intent digest** against the payment (amount,
   pay-to, resource).
4. Releases the payment **only** on a valid, in-scope proof; otherwise fails
   closed and the escrow is returned.

The existing `spt_anchor/` in the reference repo is the default Anchor **counter
scaffold** (initialize / increment) — it proves the toolchain and a devnet deploy,
not SPT-Txn verification. That logic is what gets built here.

## Highest-risk step (do not skip)

The on-chain canonicalization of the request **must** match the off-chain
verifier's byte-for-byte. A mismatch between issuer and verifier canonicalization
is the #1 authorization-bypass class in this design. Differential-test the
on-chain path against the off-chain verifier before any mainnet use.

## Why this is second, not first

On-chain verification adds latency and compute and duplicates the canonicalizer in
a second language — the opposite of SPT-Txn's offline, sub-millisecond thesis.
Online verification is a real market trend, so it's on the near path — but Option 1
ships first because it's true to the product and already runs.
