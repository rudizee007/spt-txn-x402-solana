# Colosseum Eternal — submission packet

Copy-paste reference for the Eternal portal. Items still to supply are marked
**TODO**. All three repos below were confirmed publicly reachable, signed out,
on 2026-08-04.

Colosseum asks for **two** videos at submission, not one: a **pitch
presentation (max 3 minutes)** covering team, problem, audience, any user
validation and the vision, and a **technical walkthrough (under 3 minutes)**
covering implementation, design choices, stack and the Solana integration. The
weekly updates are separate and go in the Eternal dashboard. A repo may be
private if judges are granted access — that is an explicitly supported route,
and forgetting to grant it is on their list of common mistakes.

## One-liner

> x402 moves the money. SPT-Txn proves the agent was allowed to — per transaction, with a  tamper-evident receipt, verifiable offline at the edge or online for instant revocation.


## Short description

SPT-Txn is the per-transaction authorization layer x402 is missing. x402 moves the
money; it never checks whether the *actor* was allowed to. It authorizes any actor
initiating a payment — a person acting through an interface, a non-human workload,
or an AI agent (including agents acting over MCP), with the autonomous-agent case
the most urgent. SPT-Txn puts authority inside a short-lived token bound to one
exact payment — one asset, one amount, one recipient — verified offline with no
call home, so a hijacked or prompt-injected agent holds a token that's useless for
any other payment. A pre-sign guard refuses
any transaction that doesn't match; a real USDC transfer settles on Solana devnet;
and every decision emits a signed receipt whose log's Merkle root is anchored
on-chain. Drop-in middleware adds it to any x402 server in one line.

## Links

- **Main repo:** https://github.com/rudizee007/spt-txn-x402-solana
- **On-chain escrow:** https://github.com/rudizee007/spt-txn-x402-escrow
- **Reference engine:** https://github.com/rudizee007/spt-txn-poc
- **Week 1 update video:** https://youtu.be/R_f7R0nkM1M
- **Week 2 update video:** https://youtu.be/gKuELvPePS8
- **Week 3 update video:** https://youtu.be/7hG7lTwe7i4
- **Pitch video (max 3 min):** *(TODO: record + paste URL)*
- **Technical walkthrough (under 3 min):** *(TODO: record + paste URL)*
- **Devnet settlement tx (authorization-gated USDC, shown in the demo):** https://explorer.solana.com/tx/376oVo5dNc8tVgJiXB6eVpckNhTNchxbrgs19ShZmcmNx1ZxkN6v8Hvw6TjFVRxo2Xzs1w1RDPFT6BdxbsPDU1u2?cluster=devnet
- **Devnet settlement tx (payer → merchant, earlier run):** https://explorer.solana.com/tx/3H4MfiYrsZ66pK23VkCFeKPpN18u2YiJQvWDnqTBNp4Hy541kMKtDWuVV9xnBN9Kp9R8WBiRN6m4uaBrCm76rNkX?cluster=devnet
- **Devnet evidence anchor tx (receipt root via Memo):** https://explorer.solana.com/tx/2CQpKfHvfMTd2bDp5mYAFB5giaiqLKWdAHroE74CRVf271n9VEmdbrRne6m5M4DyeKNjw9TEwxoqVBuH7YVAU1m9?cluster=devnet
- **Escrow program (deployed, devnet):** https://explorer.solana.com/address/C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ?cluster=devnet
- **Escrow — program upgraded to the hardened build** (issuer pinning + role separation): https://explorer.solana.com/tx/53G8LuvdfBucEKEkqSEVvxAvYwbzBCogQsFaAf2BiZvCt5ma7bnUfMX8tsaGLnwwdawyRUEK6s5aPRw4pcFqY7QZ?cluster=devnet
- **Escrow — deny-by-default config created** (empty allowlist; the upgrade authority signs but names a *different* key as admin — the program rejects the transaction if one key would hold both): https://explorer.solana.com/tx/3Xx65mAHMC3Cu7YrPbvmUrCxHzSXpCQYbNqJ693t33XUWM78GKLkXyPuFynijapKoovpNsJgeuijsck655U9Y4Fv?cluster=devnet
- **Escrow — issuer authorized (`add_issuer`):** https://explorer.solana.com/tx/5zY1CUym2A9eYnaGuz7f43SEJY6FQZ15cj9iuk23MGfTSvf1eXzjvSSLzNNAF1UUriAVwxygaPfk3ksNyT7vcBxZ?cluster=devnet
- **Escrow — deposit into the vault PDA** (the payer pins the one issuer permitted to release *this* escrow): https://explorer.solana.com/tx/2L4HCrVkh1yRrKtASvXntzWhWFX8Q4Bqy6JSzaRwXPCgkHTHhgVdBykFgtbkfLrQ34Uw1d1Y7ZC58NEAT4BhRJLU?cluster=devnet
- **Escrow — on-chain DENY `6105 BindingMismatch`** (validly signed, wrong escrow): https://explorer.solana.com/tx/2LhQ55BWtLTF4LrBF7Qm9vsCPReXr8Bk6RUqWHM7SJ6iwJhCF2xWM2cxJ4u99XUrdbBiT26ssVFDBNo9otTzJLmm?cluster=devnet
- **Escrow — on-chain DENY `6102 IssuerNotAuthorized`** (validly signed, issuer not allow-listed): https://explorer.solana.com/tx/2PdFXekEZHf6SD3NLzEY7FGcBZB8484yF2FA9u9A1qBNBuRzfYKz68tAPxvwfKYrKSxegvZ9RNqVLf7ia8LstBZc?cluster=devnet
- **Escrow — a compromised admin allow-lists a rogue issuer, and it SUCCEEDS** (the admin genuinely has this power; the run does not simulate it): https://explorer.solana.com/tx/576zuHiQqgc8jNTGpgpAKzfLV5Ry7ZihJF9g9Ffj3CVYKXVzNc2F8zFrvBvcB5FFhH56PtUGswBKT5WuboLXYgBs?cluster=devnet
- **Escrow — on-chain DENY `6108 IssuerNotPinned`** — *the custody claim.* The rogue issuer is now on the allowlist and signs a valid, correctly bound, fresh attestation. The release still fails, because it is not the issuer this payer pinned at deposit: https://explorer.solana.com/tx/5tVnrNwESCNawdLEB7Q6r2wVys2rEvpRMZrTQUaWEwz5roE2W6fypJ2V8yx1WtJTKFMr3UzVcJqsAEKpy6jGsC6Y?cluster=devnet
- **Escrow — rogue issuer revoked (`remove_issuer`):** https://explorer.solana.com/tx/5q9joyuykk3xTBzqj3EPhHyJthDmjL4d98eCRzEPTCEpXYRmvU8YQLqtdgwXJfSfW9E7uXfrcxjdGcXLUCtkH3ib?cluster=devnet
- **Escrow — RELEASE against a valid proof** (funds move, escrow closes, spent marker created): https://explorer.solana.com/tx/3wAaBFu2mRJtGq2MZJ5RQFf9iwaKyTyVrETpLySYrMb9uxbQ74bt6bdhSNS7uuMakuY1eddx5Tu5EC4kheWugiVw?cluster=devnet
- **IETF Internet-Draft:** https://datatracker.ietf.org/doc/draft-coetzee-oauth-spt-txn-tokens/
- **Zenodo DOI:** `10.5281/zenodo.19299787`
- **ORCID:** `0009-0009-6557-8843`

## Reproduce in ~5 minutes

```sh
git clone <main repo> && cd spt-txn-x402-solana
go test ./...          # gate + settle + receipt + gateway, incl. differential KATs
go run ./cmd/x402demo  # 402 → gate → guard → settle, over real HTTP, + evidence root
go run ./cmd/gateway   # drop-in PEP middleware + transparency-log endpoints
```

Devnet (real USDC, your keypair): fund at faucet.solana.com + faucet.circle.com,
then `go run -tags devnet ./cmd/paydevnet -amount 100000` (settles), `… -tamper`
(refuses before signing), `go run -tags devnet ./cmd/anchordevnet` (anchors root).

## The ask

Eternal award + accelerator entry, converting to a pre-seed round to fund the
hosted transparency log, the first two jurisdiction policy packs (Travel Rule +
MiCA), and the x402 gateway — the shortest paths to recurring revenue.

## Pre-submission checklist

- [x] `go test ./...` green; `govulncheck ./...` **and** `govulncheck -tags devnet ./...`
      both clean (2026-08-03). One module-level, unreachable finding
      (`GO-2026-5932`, unimported `x/crypto/openpgp`) documented in
      [`SECURITY.md`](../SECURITY.md); Rust `cargo audit` findings are dev-dependency
      only and documented in the escrow repo's `SECURITY.md`.
- [ ] Main repo public + §0-clean (final grep: no hook / private / disclosure refs)
- [x] Escrow repo published public (2026-07-19)
- [x] Release-on-proof escrow proven on devnet: deny 6105, deny 6102, release
- [ ] Pitch video (max 3 min) recorded, uploaded, and linked here
- [ ] Technical walkthrough (under 3 min) recorded, uploaded, and linked here
- [ ] Product page filled (from `ETERNAL-PRODUCT.md`); all links above resolve
- [x] Week 1 update video posted (https://youtu.be/R_f7R0nkM1M)
- [x] Week 2 update video posted (https://youtu.be/gKuELvPePS8)
- [x] Week 3 update video posted (https://youtu.be/7hG7lTwe7i4)
- [ ] Week 3 update submitted in the Eternal dashboard
- [ ] Week 4 update video posted and submitted
- [ ] Devnet tx links open on the explorer
