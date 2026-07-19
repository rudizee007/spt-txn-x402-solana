# Colosseum Eternal — 4-week sprint plan

Track framing: **Best Trustless Agent / x402 authorization** — the per-transaction
authorization layer x402 leaves out.

## Mechanics (confirm exact fields on colosseum.com/eternal)

- Hit the **stopwatch** on the Eternal dashboard to start a 4-week sprint.
- Post a **brief weekly update** with a Loom/YouTube video: what you prioritized
  and shipped that week.
- **Submit** through the portal at the end of week 4.
- Eligible for the **$25k Eternal Award** (semi-annual) plus **$250k pre-seed +
  accelerator cohort** consideration.

Start the clock only once this plan is set, so each weekly update shows genuine
week-over-week shipping.

## Week 0 — already built (the foundation you demo in Update 1)

Offline x402 gate (differential-tested intent binding), HTTP `402`/`X-PAYMENT`
flow, pre-sign `TransferChecked` guard, **real USDC settlement on devnet**, the
tamper→refuse-before-signing moment, and signed receipts with an RFC 6962 Merkle
root **anchored on devnet**. `go test ./...` green, govulncheck clean.

## The 4 weeks

| Week | Ship | Weekly-update video focus |
|---|---|---|
| **1** | Merchant-pay (`CreateAssociatedTokenAccountIdempotent`, pay a real second wallet); flip the repo **public** after a novelty scan of the new Option-1 content; polish the quickstart. | The working loop end-to-end on devnet: 402 → gate → guard → USDC settle → on-chain evidence root. |
| **2** | Public **Option 2 on-chain escrow enforcement** (already novelty-cleared and devnet-deployed) wired in as the "trustless" settlement path: funds released only on a valid on-chain proof, fails closed. | On-chain enforcement — the stronger trustless story; escrow release gated by the SPT-Txn proof. |
| **3** | **Gateway / PEP form factor**: a drop-in x402 middleware a resource server puts in front of its endpoint, plus a compliance-receipt/transparency-log service (the receipts productized). | Adoption — how any x402 server adds authorization with one middleware and gets audit evidence for free. |
| **4** | Polish, docs, the 3-minute demo video (script ready in `DEMO-SCRIPT.md`), pitch, and submission. | The full product + the ask: standards position (IETF draft, NIST/NCCoE), what the accelerator funds. |

## Submission checklist

- [ ] Public repo, novelty-scanned and §0-clean (Option 1 content is cleared;
      keep any unpublished on-chain-enforcement work out of it).
- [ ] Demo video (per `DEMO-SCRIPT.md`) uploaded and linked.
- [ ] Product description / one-pager (reuse `MONETIZATION.md`; tighten to the
      Eternal product-page format).
- [ ] Live devnet artifacts: a settlement tx link and the anchor (memo) tx link.
- [ ] Standards/credibility links: IETF `draft-coetzee-oauth-spt-txn-tokens`,
      Zenodo DOI, ORCID.
- [ ] Weekly update videos 1–4 posted.

## §0 guardrail (every week)

Only published-primitive, novelty-cleared work goes public. The private
on-chain-enforcement research and jurisdictional policy packs stay in their own
repos and are never referenced in commits, updates, or the submission.

## Immediate actions

1. Decide a start date and hit the stopwatch (kicks off the 4-week clock).
2. Build Week 1: merchant-pay + repo public prep.
3. Draft the Eternal product description from `MONETIZATION.md`.
