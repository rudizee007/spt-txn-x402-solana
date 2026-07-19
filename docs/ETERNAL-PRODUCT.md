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
- A **real USDC payment settled on devnet** — payer → a brand-new merchant, whose
  token account is created in the same transaction — and, from the same wallet, a
  *tampered* payment **refused before signing**, so nothing touches the chain.
  [Settlement tx](https://explorer.solana.com/tx/3H4MfiYrsZ66pK23VkCFeKPpN18u2YiJQvWDnqTBNp4Hy541kMKtDWuVV9xnBN9Kp9R8WBiRN6m4uaBrCm76rNkX?cluster=devnet)
  (the refusal never becomes a transaction).
- **Signed, hash-chained receipts** with an RFC 6962 Merkle root **anchored
  on-chain via SPL Memo** — any single decision is provably in the batch,
  tamper-evident, no PII on the ledger.
  [Anchor tx](https://explorer.solana.com/tx/2CQpKfHvfMTd2bDp5mYAFB5giaiqLKWdAHroE74CRVf271n9VEmdbrRne6m5M4DyeKNjw9TEwxoqVBuH7YVAU1m9?cluster=devnet)

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
   devnet-deployed).
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
