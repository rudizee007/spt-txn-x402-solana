# SPT-Txn — authorization for agentic payments

**x402 moves the money. SPT-Txn proves the agent was *allowed* to — per
transaction, verified offline, with a tamper-evident receipt.**

## The problem

The agent economy is being built on payment rails with no authorization layer.
x402 lets an AI agent pay autonomously, but nothing checks whether the agent was
*allowed* to make that payment — on whose behalf, within what limit, under which
policy. When an agent is prompt-injected or hijacked (and they are), x402 will
faithfully pay the attacker. The blast radius is every dollar the agent can reach.

## The solution

SPT-Txn is a scoped, provable, transaction-bound authorization token. Authority
exists only inside a short-lived token bound to **one exact payment** — one asset,
one amount, one recipient — verified **offline, sub-millisecond, with no call
home**. A compromised agent holds a token that is cryptographically useless for
any payment it didn't declare. Every decision emits a signed receipt, so
compliance evidence is a byproduct of enforcement, not a later audit.

We are **not** an identity provider and **not** a payment rail. We are the
authorization layer between them — consuming identity, gating the payment — which
makes every identity vendor and payment rail a partner, not a competitor.

## What's built and proven on Solana devnet

Reproducible from the repo in ~5 minutes (`go test ./...`, `go run ./cmd/x402demo`):

- A real HTTP **402 → gate → settle → X-PAYMENT** round-trip. The gate binds the
  exact x402 payment fields and returns ALLOW, or DENY with a distinct
  *violation* vs *unavailable* class. The binding is differential-tested against
  an independent Python implementation.
- A **pre-sign guard** that refuses to sign unless the on-chain transaction pays
  exactly the bound recipient/asset/amount under the payer's authority.
- A **real USDC payment settled on devnet** — payer → merchant, with the
  merchant's USDC token account created *idempotently* in the same transaction
  when it does not already exist, so a brand-new recipient just works — and, from
  the same wallet, a *tampered* payment **refused before signing**, so nothing
  touches the chain.
  [Settlement tx](https://explorer.solana.com/tx/3H4MfiYrsZ66pK23VkCFeKPpN18u2YiJQvWDnqTBNp4Hy541kMKtDWuVV9xnBN9Kp9R8WBiRN6m4uaBrCm76rNkX?cluster=devnet)
  (the refusal never becomes a transaction).
- **Signed, hash-chained receipts** with an RFC 6962 Merkle root **anchored
  on-chain via SPL Memo** — any single decision is provably in the batch,
  tamper-evident, no PII on the ledger.
  [Anchor tx](https://explorer.solana.com/tx/2CQpKfHvfMTd2bDp5mYAFB5giaiqLKWdAHroE74CRVf271n9VEmdbrRne6m5M4DyeKNjw9TEwxoqVBuH7YVAU1m9?cluster=devnet)
- **Trustless release-on-proof escrow — the program itself is the enforcement
  point.** Funds sit in a vault PDA and move only against an Ed25519 attestation
  from an allow-listed issuer, bound to *that exact escrow*. The config is
  created with an **empty** allowlist — deny-by-default, on chain, in its own
  transaction — and the issuer is authorized separately. Two **validly signed**
  proofs are then refused *inside the program*: one bound to a different payment
  (`6105 BindingMismatch`), one from a non-allowlisted issuer
  (`6102 IssuerNotAuthorized`). Only the real proof releases, after which a
  spent-marker PDA makes that binding unreleasable forever.
  [Config, empty allowlist](https://explorer.solana.com/tx/2hKJngUeAdg3CGJ6p1RzCzc7T5cyaQBuk82X1oocBtxUXY9T9DbJsiKEBqAH2jc8pHbaWrrGSH6XGgP9Ph7qTaCQ?cluster=devnet) ·
  [Issuer authorized](https://explorer.solana.com/tx/3TpeXa9N6oeQoimyfxWC7BEBFDpFLy2VAqKEKvZukhCfNnePqiBp19KqXtYB5sS2AgdpdrxVUTBeQkyUopfo3gpz?cluster=devnet) ·
  [Deposit](https://explorer.solana.com/tx/23uwVCXj7ZWgXzdYHQRn9YrUmPeMVXBTJ4847ZxsZdrUkwaFen5NgqYXUsvuExTwJ9MiJpwZBMaqyKnAYPSXg3AW?cluster=devnet) ·
  [DENY 6105](https://explorer.solana.com/tx/62NBEFfhkuXacPuUWZaFUBmpiPR6NivnDKkwTEQQDiPL5RDD3uq74G1gMSoJrtHb8QcesGyhyeh4GZpv1ChsZQz4?cluster=devnet) ·
  [DENY 6102](https://explorer.solana.com/tx/67VjPLQprswyR2wS6RaS4k6xFbkNxddrz4sUVu11b2c96A1kPg8i58vPW9ag3YMEB872EkC6eh6rTZA28LDmuuqy?cluster=devnet) ·
  [RELEASE](https://explorer.solana.com/tx/2zeKqbfirZ9U7VwbL2ngdRm9phRDLobAq1oUtvSz9Jk2A6HUhKviNL2Q8YvAjHLbUjCoWrMWKN6ykEaYTFshe43f?cluster=devnet)

  Both denials are genuinely signed. Tampering with the *signature* instead
  would fail the Ed25519 precompile at transaction verification, the validator
  would drop the transaction, and nothing would land — a dropped transaction
  proves nothing. An on-chain DENY requires a real signature failing a real
  policy check.

No custom cryptography; `go test ./...` green; `govulncheck` clean.

## Scope — what's in, what's out

**In scope (this deliverable):**

1. **Off-chain authorization gate** — intent binding + pluggable policy, ALLOW /
   two DENY classes.
2. **HTTP x402 flow** — real `402` + `X-PAYMENT` retry.
3. **Pre-sign settlement guard** — refuses to sign unless the transaction pays
   exactly what was authorized.
4. **USDC settlement on devnet** — including merchant-pay and the tamper refusal.
5. **Signed receipts + RFC 6962 Merkle log + on-chain anchor.**
6. **On-chain trustless escrow** — release-on-proof enforcement (Anchor program,
   devnet-*proven*: two validly-signed proofs refused on chain, then a release).
7. **Gateway / PEP middleware** — drop-in x402 authorization *(built this sprint)*.
8. **Transparency-log / receipts service** — receipts productized *(built this
   sprint)*.

All open (Apache-2.0) and reproducible; 1–6 are devnet-proven today, 7–8 are the
sprint's net-new build.

**Out of scope (not part of this deliverable):**

- **Mainnet** — intentionally not wired; everything is devnet-first.
- **Proprietary jurisdiction policy packs** — a separate commercial product; the
  open framework consumes any policy engine (OPA, Sumsub, in-house) instead.
- **Other proprietary / unpublished work** — developed separately, never part of
  this open deliverable.

## Why now

x402 deliberately leaves authorization, delegation, and revocation out of scope —
a gap the whole ecosystem is now hitting. NIST/CAISI and NCCoE are standardizing
AI-agent authorization, and the construct they are converging toward is
transaction-scoped authorization — this one. Regulated money movement (FATF
Travel Rule, EU MiCA/DORA) needs per-transaction, provable, PII-free
authorization. Standards position is the moat.

## Business model

Open core, paid edges. The spec and reference engine are open (Apache-2.0) — the
distribution and the standards credibility. Revenue comes from compliance
receipts + a hosted transparency log, jurisdiction policy packs, and a gateway /
policy-enforcement-point form factor. We **consume the customer's policy engine**
(OPA, Sumsub, in-house), we don't dictate the ruleset — so adoption is additive,
and the customer remains the licensed entity while we sell a compliance-support
tool that runs in their infra and never touches PII or holds funds.

## Traction & assets (all citable)

- IETF Internet-Draft `draft-coetzee-oauth-spt-txn-tokens`; formal game-based
  security proofs; Zenodo DOI `10.5281/zenodo.19299787`; ORCID
  `0009-0009-6557-8843`.
- Working open-source reference implementation: offline verifier, attenuating
  delegation, format-agnostic policy engine.
- This Solana x402 integration, proven on devnet (above).
- NIST SP 800-133r3 public comments; NCCoE engagement.

## The ask

Eternal award + accelerator entry, converting to a pre-seed round to fund the
hosted transparency log, the first two jurisdiction policy packs (Travel Rule +
MiCA), and the x402 gateway — the shortest paths to recurring revenue.

---

*Open, published-primitive scope only. Proprietary policy packs and unpublished
research are developed separately and are not part of this deliverable.*
