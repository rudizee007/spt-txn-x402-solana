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
  transaction — by the upgrade authority, which must name a *different* key as
  admin; the program rejects the transaction if one key would hold both roles.
  At deposit the payer **pins** the one issuer whose attestation can ever
  release that escrow, and the program stores it immutably. Three **validly
  signed** proofs are then refused *inside the program*: one bound to a
  different payment (`6105 BindingMismatch`), one from a non-allowlisted issuer
  (`6102 IssuerNotAuthorized`), and — the one that answers the custody
  question — one from a rogue issuer that a **compromised admin successfully
  added to the allowlist**, which still fails `6108 IssuerNotPinned`. Only the
  real proof releases, after which a spent-marker PDA makes that binding
  unreleasable forever.
  [Program upgrade](https://explorer.solana.com/tx/53G8LuvdfBucEKEkqSEVvxAvYwbzBCogQsFaAf2BiZvCt5ma7bnUfMX8tsaGLnwwdawyRUEK6s5aPRw4pcFqY7QZ?cluster=devnet) ·
  [Config, empty allowlist, admin separated](https://explorer.solana.com/tx/3Xx65mAHMC3Cu7YrPbvmUrCxHzSXpCQYbNqJ693t33XUWM78GKLkXyPuFynijapKoovpNsJgeuijsck655U9Y4Fv?cluster=devnet) ·
  [Issuer authorized](https://explorer.solana.com/tx/5zY1CUym2A9eYnaGuz7f43SEJY6FQZ15cj9iuk23MGfTSvf1eXzjvSSLzNNAF1UUriAVwxygaPfk3ksNyT7vcBxZ?cluster=devnet) ·
  [Deposit, issuer pinned](https://explorer.solana.com/tx/2L4HCrVkh1yRrKtASvXntzWhWFX8Q4Bqy6JSzaRwXPCgkHTHhgVdBykFgtbkfLrQ34Uw1d1Y7ZC58NEAT4BhRJLU?cluster=devnet) ·
  [DENY 6105](https://explorer.solana.com/tx/2LhQ55BWtLTF4LrBF7Qm9vsCPReXr8Bk6RUqWHM7SJ6iwJhCF2xWM2cxJ4u99XUrdbBiT26ssVFDBNo9otTzJLmm?cluster=devnet) ·
  [DENY 6102](https://explorer.solana.com/tx/2PdFXekEZHf6SD3NLzEY7FGcBZB8484yF2FA9u9A1qBNBuRzfYKz68tAPxvwfKYrKSxegvZ9RNqVLf7ia8LstBZc?cluster=devnet) ·
  [Rogue issuer allow-listed — SUCCEEDS](https://explorer.solana.com/tx/576zuHiQqgc8jNTGpgpAKzfLV5Ry7ZihJF9g9Ffj3CVYKXVzNc2F8zFrvBvcB5FFhH56PtUGswBKT5WuboLXYgBs?cluster=devnet) ·
  [DENY 6108 IssuerNotPinned](https://explorer.solana.com/tx/5tVnrNwESCNawdLEB7Q6r2wVys2rEvpRMZrTQUaWEwz5roE2W6fypJ2V8yx1WtJTKFMr3UzVcJqsAEKpy6jGsC6Y?cluster=devnet) ·
  [Rogue revoked](https://explorer.solana.com/tx/5q9joyuykk3xTBzqj3EPhHyJthDmjL4d98eCRzEPTCEpXYRmvU8YQLqtdgwXJfSfW9E7uXfrcxjdGcXLUCtkH3ib?cluster=devnet) ·
  [RELEASE](https://explorer.solana.com/tx/3wAaBFu2mRJtGq2MZJ5RQFf9iwaKyTyVrETpLySYrMb9uxbQ74bt6bdhSNS7uuMakuY1eddx5Tu5EC4kheWugiVw?cluster=devnet)

  The `add_issuer` step for the rogue issuer is not simulated and does not
  fail — a compromised admin really does hold that power, and the run shows it
  landing on chain. What it cannot do is make the resulting key able to release
  an escrow pinned to someone else. The admin is a denial-of-service role, not
  a custody role, and that distinction is the whole claim.

  All three denials are genuinely signed. Tampering with the *signature* instead
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
a gap the whole ecosystem is now hitting. The same gap is open one level up: in
February 2026 NIST's CAISI opened an AI Agent Standards Initiative, and the NCCoE
published a concept paper on software and AI-agent identity and authorization
that names authorization, auditing and non-repudiation of agents as unsolved
without settling on a mechanism. Both are at concept and request-for-input stage.
No standard has been written, nothing has been endorsed, and that is precisely
the opening: the construct is still undecided, and what gets read at this stage
is a working implementation with a published security argument behind it.

Regulated money movement pushes from the demand side. The FATF Travel Rule, MiCA
and DORA oblige operators to attribute and retain records for individual
transfers. None of them mandates this construct — and the Travel Rule in fact
requires originator and beneficiary data to move *between* institutions, so
"PII-free" describes the ledger, not the compliance flow. What an operator gets
here is per-transaction authorization evidence as a byproduct of enforcement,
with no PII written on a public chain.

## Business model

Open core, paid edges. The spec and reference engine are open (Apache-2.0) — the
distribution and the standards credibility. Revenue comes from compliance
receipts + a hosted transparency log, jurisdiction policy packs, and a gateway /
policy-enforcement-point form factor. We **consume the customer's policy engine**
(OPA, Sumsub, in-house), we don't dictate the ruleset — so adoption is additive,
and the customer remains the licensed entity while we sell a compliance-support
tool that runs in their infra and never touches PII or holds funds.

## Traction & assets (all citable)

- IETF Internet-Draft `draft-coetzee-oauth-spt-txn-tokens` — an individual
  submission, not working-group adopted; formal game-based security proofs;
  Zenodo DOI `10.5281/zenodo.19299787`; ORCID `0009-0009-6557-8843`.
- Working open-source reference implementation: offline verifier, attenuating
  delegation, format-agnostic policy engine.
- This Solana x402 integration, proven on devnet (above).
- Public comments submitted on the NIST SP 800-133r3 initial public draft
  (released for comment April 2026), and on the NCCoE concept paper *Accelerating
  the Adoption of Software and AI Agent Identity and Authorization* (comments
  closed 2 April 2026). Submitted and on the record — not endorsed; NIST has
  published no response to either.

## The ask

Eternal award + accelerator entry, converting to a pre-seed round to fund the
hosted transparency log, the first two jurisdiction policy packs (Travel Rule +
MiCA), and the x402 gateway — the shortest paths to recurring revenue.

---

*Open, published-primitive scope only. Proprietary policy packs and unpublished
research are developed separately and are not part of this deliverable.*
