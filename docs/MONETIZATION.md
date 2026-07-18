# SPT-Txn × x402 — Monetization One-Pager

**Violet Sky Security SEZC · authorization for the agent economy**
*Audience: Colosseum judges & pre-seed investors*

---

## The problem, in one line

The agent economy is being built on **payment rails with no authorization layer.**
x402 lets an AI agent pay autonomously — but nothing checks whether the agent was
*allowed* to make that payment, on whose behalf, within what limit, or under which
regulation. When agents are compromised or prompt-injected — and they are — the
blast radius is every dollar they can reach.

## What we sell

**SPT-Txn: a scoped, provable, transaction-bound authorization token.** Authority
exists only inside a short-lived token bound to one payment, one resource, one
accountable human — verified **offline, sub-millisecond, no call home.** A
compromised agent holds a token that is cryptographically useless for any payment
it didn't declare. Every decision emits a signed receipt, so compliance evidence
is a byproduct of enforcement, not an audit.

We are **not** an identity provider and **not** a payment rail. We are the
**authorization layer that sits between them** — we consume identity, we gate the
payment. That makes every identity vendor and every payment rail a partner, not a
competitor.

## Why now

- **x402 deliberately leaves authorization, delegation, policy, and revocation out
  of scope** — a gap the whole ecosystem is now hitting. The Solana Foundation,
  Visa, Coinbase, and Phantom are all pushing agentic payments in 2025–26.
- **NIST/CAISI and NCCoE are standardizing AI-agent authorization right now**, and
  the construct they are converging toward is transaction-scoped authorization —
  ours. Standards position is the moat.
- Regulated money movement (FATF **Travel Rule**, EU **DORA/MiCA**) needs
  per-transaction, provable, PII-free authorization. That is exactly what we emit.

## How we make money

| Line | Model | Who pays | Timing |
|---|---|---|---|
| **Compliance receipts + transparency log** | SaaS / usage — priced per verified transaction | VASPs, fintechs, banks moving regulated value through agents | **Fastest to revenue** |
| **Jurisdictional policy packs** (Travel Rule, DORA, MiCA, SEC/VARA/CIMA) | Subscription per jurisdiction profile — **optional; bring-your-own engine supported** | Regulated operators who want the floor turnkey rather than coded | Near-term |
| **Gateway / PEP form factor** (x402 middleware, Envoy `ext_authz`, OPA, MCP) | Per-seat / per-node platform license | Platform & infra teams adopting agentic payments | Adoption multiplier |
| **NHI attested issuance** (SPIFFE, cloud workload identity, RFC 8693) | Embedded / OEM into identity & agent platforms | Identity vendors who want the layer *on* them | Acquisition-critical |
| **Open core** (spec, reference engine, this repo) | Apache-2.0, free | Everyone | Credibility & funnel |

**Open core, paid edges.** The spec and reference engine are open — that is the
distribution and the standards credibility. Revenue comes from the
jurisdiction-specific policy packs, the hosted transparency log, and the gateway
licenses that regulated operators cannot responsibly self-assemble.

**We consume the customer's policy engine — we don't dictate the ruleset.** Policy
is evaluated at one pluggable decision point (the ABAC → TBAC boundary), and the
reference engine ships an OPA-compatible decision API. A customer who already runs
Sumsub, an OPA/Rego ruleset, or a bespoke in-house engine keeps it — SPT-Txn
consumes their ALLOW/DENY and binds it into a provable, per-transaction token,
adding the enforcement layer their engine lacks. Our jurisdiction packs are a
**turnkey convenience for operators who don't want to author the rules themselves,
not a lock-in.** This lowers the adoption barrier (no rip-and-replace of an existing
compliance investment) and keeps us positioned *on top of* the RegTech vendors,
not competing with them — the same "consume, don't replace" posture we hold with
identity providers. It also keeps us on the right side of the regulatory line: the
customer is the licensed/obligated entity and names the tools in their control
framework; we sell a compliance-support tool that runs entirely in their infra,
never touching PII or holding funds.

## Traction / assets (real, all citable)

- IETF Internet-Draft `draft-coetzee-oauth-spt-txn-tokens`; formal game-based
  security proofs; Zenodo DOI `10.5281/zenodo.19299787`.
- Working reference implementation (Apache-2.0): offline 8-step verifier,
  attenuating delegation, format-agnostic policy engine.
- **Proven on Solana devnet today**: an agent's over-scope payment is refused
  before signing; an in-scope payment settles on devnet with a zero-knowledge
  human anchor written on-chain via SPL Memo — no PII on the ledger.
- Live integration demonstrated against enterprise identity (PingOne AI Agent,
  Auth0, Keycloak) over RFC 8693.
- NIST SP 800-133r3 public comments acknowledged; NCCoE Letter of Interest.

## The Solana / x402 wedge (this hackathon)

x402 gives us a fast-growing, standards-shaped payment surface with a named
authorization gap and Foundation-level attention. Winning here puts the
authorization layer in front of exactly the builders defining agentic payments —
and turns "we have a spec" into "it runs on mainnet."

## The ask

Non-dilutive prize + accelerator entry (Colosseum), converting to a pre-seed
round to fund the hosted transparency log, the first two jurisdictional policy
packs (Travel Rule + MiCA), and the x402 gateway form factor — the three lines
above with the shortest path to recurring revenue.

---

*Public-goods scope only. Proprietary jurisdictional policy packs and unpublished
IP are developed separately and are not part of any open-source deliverable.*
