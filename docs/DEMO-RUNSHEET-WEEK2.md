# Demo run-sheet — Week 2: Trustless on-chain enforcement (~90s)

The Week-1 clip showed SPT-Txn refuse an unauthorized payment **off-chain**,
before signing. This one shows the **chain itself** enforce it: a Solana program
that holds the payment in escrow and releases it only against a valid,
in-scope authorization proof. Say **devnet**, never mainnet.

## Before you record

- `cd spt-txn-x402-escrow`
- Pre-build so `cargo test` is fast on camera (no minute-long compile):
  `cargo build --tests -p spt_x402_escrow` (off-camera). Then `clear`.
- Large terminal font, dark theme.

## Beats

| Time | Command / on screen | Say |
|------|---------------------|-----|
| 0:00–0:15 | Title, or the `spt-txn-x402-escrow` repo on screen | "Last time, SPT-Txn refused to sign an unauthorized payment off-chain — before it ever touched the chain. But sometimes you want the chain *itself* to enforce it. That's the escrow: a Solana program that holds the payment and releases it only against a valid, in-scope authorization proof." |
| 0:15–0:45 | `cargo test -p spt_x402_escrow` *(runs fast — pre-built; 7 unit tests + the integration test pass)* | "Here's the program's own test suite. The intent binding is checked against a known-answer vector, secret-adjacent bytes are compared in constant time, freshness is bounded, and there's a full on-chain integration test that runs the happy path and then proves a *replayed* proof is blocked. All green." |
| 0:45–1:10 | `docs/THREAT-MODEL.md` (or SPEC.md) on screen, scroll the invariants | "And it fails closed by construction. A missing or duplicate attestation, an issuer that isn't on the allowlist, a binding mismatch, stale freshness, or a replayed nonce — all revert. There's no canonicalization on-chain, so the classic issuer-versus-verifier bypass is designed out. Memory-safe Rust, no custom cryptography." |
| 1:10–1:20 | *(optional, only if deployed)* browser → Explorer for the program id | "And it's live on Solana devnet." |
| 1:20–1:30 | Close card | "So authorization isn't only enforced by the client — it's enforced by the chain. Off-chain speed when you want it; on-chain trustlessness when you need it." |

## Commands, in order

```sh
clear
cargo test -p spt_x402_escrow     # pre-build first so this is fast on camera
# then just scroll docs/THREAT-MODEL.md and docs/SPEC.md
```

If you deploy first (`anchor deploy`), add an Explorer cut for the program id
`C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ`; otherwise the passing test +
spec is the evidence. Don't run `anchor build` on camera — the compile is long;
show `cargo test` from a warm cache instead.
