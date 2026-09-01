# Week 4 — weekly update (1 minute)

**RECORDED AND PUBLISHED 2026-08-22 — https://youtu.be/cYbHxNoZHGI**

Before submitting the link, confirm two things in YouTube Studio:

1. **Visibility is Public or Unlisted, never Private.** A Private link is
   invisible to judges and is the single most common submission failure.
2. **Duration is at or under 1:00**, per Colosseum's *"1 minute project update"*.

Note: the Eternal dashboard lists a video-update checkbox for weeks 1, 2 and 3
but **not** for week 4 — week 4's only item is *"Review & submit your project."*
Confirm where this link is meant to go before assuming a slot exists for it. It
is good material for the product page and YouTube regardless.

Colosseum's Eternal page: *"At the end of each week, provide a **1 minute**
project update in the dashboard."* This script is **129 words ≈ 52 seconds** at
150 wpm, leaving margin for pauses. Do not let it grow.

The longer narrative cut lives in `PITCH-VIDEO-3MIN-DRAFT.md` — use that if the
submission form asks for a separate pitch video.

Voiceover over B-roll. Four blocks. Record each separately, then cut visuals
underneath. Say **devnet**, never mainnet.

---

# 1 — Prioritized
**0:00–0:07 · 18 words**

**Visual:** `video-assets/01-title.png`

**Read:**

```
This week I prioritized packaging and submitting: the demo video, the product
page, and the final Eternal submission.
```

---

# 2 — Shipped
**0:07–0:27 · 50 words**

**Visual:** open on `SPT-TXN-MCP-live-demo.mp4` (the agent being refused), then
cut fast through the terminal captures — settlement, tamper refusal, escrow,
gateway. Roughly 4 seconds each.

**Read:**

```
And the whole thing is now demonstrable end to end. A real AI agent — Claude
Desktop — paying over MCP, with a hijacked recipient refused. Real USDC settling
on Solana devnet. On-chain escrow that releases only against a valid proof and
fails closed. A drop-in gateway, and signed receipts anchored on-chain.
```

---

# 3 — Proof
**0:27–0:39 · 30 words**

**Visual:** the settlement transaction on Solana Explorer (devnet cluster), then
`video-assets/03-gap-closed.png`.

**Read:**

```
All of it open source, runnable from the README, and on devnet today.
Self-custody or a custodian — we bind the transfer authority, whoever holds it,
and we never hold keys.
```

---

# 4 — Why it matters, and next
**0:39–0:52 · 31 words**

**Visual:** `video-assets/04-end-card.png`

**Read:**

```
x402 moves the money. SPT-Txn proves the agent was allowed to — the authorization
layer the agent economy is missing. Next: the hosted transparency log and the
first jurisdiction packs. Links below.
```

---

## What got cut, and why it's safe

Removed from the long version: the standards section (NIST/NCCoE), the regulated
pull (FATF/MiCA/DORA), and the open-core business model. All three belong in a
pitch video, not a one-minute shipping update — and the template this update
follows is *prioritized / shipped / proof / next*, which is what judges are
looking for week to week.

The custody line survived because it is one sentence and it pre-empts the
first objection anyone has about agent payments.

## Accuracy guardrails

- **devnet**, never mainnet.
- Never "audited", "validated" or "certified" about this product.
- No bare "reproducible" — always "runnable from the README".
- No custodian-integration claim. The design accommodates custody; that is all.
- Nothing about work outside the public repositories.

## B-roll needed

One continuous silent screen recording covers everything:

```

go run ./cmd/x402demo

go run -tags devnet ./cmd/paydevnet -to $MERCHANT -amount 100000

go run -tags devnet ./cmd/paydevnet -to $MERCHANT -amount 100000 -tamper

go run -tags devnet ./cmd/anchordevnet

go run ./cmd/gateway

go run -tags devnet ./cmd/escrowdevnet -mode all -to $MERCHANT -amount 100000

```

Then the browser: the settlement tx, the anchor memo tx, and the escrow program
`C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ` on Solana Explorer, devnet
cluster. Pause 2–3 seconds on a still screen after each command so the editor has
clean cut points.

At one minute you only need ~4 seconds of each clip, so a fumbled take costs
nothing — trim it out.
