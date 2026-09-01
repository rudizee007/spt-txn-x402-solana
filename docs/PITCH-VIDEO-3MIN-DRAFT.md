# Pitch video (~3 min) — DRAFT, repurposed from the Week 4 long cut

The Week 4 weekly update is capped at **1 minute** by Colosseum, so this longer
narrative was moved here. If the Eternal submission form asks for a separate
pitch video, this is the script for it — problem, proof, market, model, ask.

**Confirm the required length in the submission form before recording.**

Runtime ~2:52 at 150 wpm (430 words). Drop section 7 to land near 2:25.

Say **devnet**, never mainnet. Nothing here references unpublished or
patent-track work.

---

# 1 — The gap
**0:00–0:24 · 60 words**

**Capture:** none. Editor-made title card, then a simple diagram of the x402 gap
or a talking head.

**Read:**

```
AI agents are starting to pay for things on their own, using x402 — a payment
standard built on HTTP. But x402 answers only one question: did the money move?
It never checks whether the agent was allowed to move it. A hijacked or
prompt-injected agent will pay an attacker, faithfully. That's an unbounded
liability, and it's the gap SPT-Txn closes.
```

---

# 2 — A real agent
**0:24–0:39 · 37 words**

**Capture:** none. Use the existing `SPT-TXN-MCP-live-demo.mp4` — Claude Desktop
calls the payment tool, the approved payment settles, the hijacked recipient is
refused.

Lead with this. Every other clip uses a CLI standing in for an agent; this is the
only one where a real agent is refused on camera, and the track is *Best
Trustless Agent*.

**Read:**

```
Start with a real one. This is Claude Desktop, calling a payment tool over MCP.
The payment a human approved settles on Solana devnet. The hijacked recipient is
refused by the enforcement point — before anything is signed.
```

---

# 3 — What's built
**0:39–1:03 · 59 words**

**Capture:** harvested from the submission-demo shoot — no re-run needed. Cut a
few seconds from each:

```
go run ./cmd/x402demo
go run -tags devnet ./cmd/paydevnet -to $MERCHANT -amount 100000
go run -tags devnet ./cmd/paydevnet -to $MERCHANT -amount 100000 -tamper
go run -tags devnet ./cmd/anchordevnet
go run ./cmd/gateway
```

**Read:**

```
Across these demos you've seen the whole thing work. A token bound to one exact
payment, verified offline with no call home. A guard that refuses to sign a
transaction that doesn't match. Real USDC settling on devnet. A drop-in gateway.
And tamper-evident receipts anchored on-chain. All open source, all runnable
yourself from the README, all on devnet today.
```

*"Runnable yourself from the README", never a bare "reproducible" — a security
audience hears that as reproducible builds, a claim not yet earned.*

---

# 4 — Trustless enforcement
**1:03–1:18 · 37 words**

**Capture: THE ONE NEW RECORDING.** Silent screen capture.

First time only:

```
go run -tags devnet ./cmd/escrowdevnet -gen-admin
go run -tags devnet ./cmd/escrowdevnet -gen-issuer
go run -tags devnet ./cmd/escrowdevnet -mode setup
```

The shot — deposit, three refusals, release, refund in sequence:

```
go run -tags devnet ./cmd/escrowdevnet -mode all -to $MERCHANT -amount 100000
```

If `-mode all` runs long, capture just the refusal instead:

```
go run -tags devnet ./cmd/escrowdevnet -mode deny-issuer -to $MERCHANT -amount 100000
```

*The admin key must differ from `-keypair` — the program enforces it and the CLI
refuses the collapsed-roles setup locally, before spending a devnet round trip.*

Also grab the program on Explorer, devnet cluster:
`C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ`

**Read:**

```
And the strongest version doesn't trust our gate at all. The escrow program on
Solana devnet releases funds only against a valid proof, and fails closed if it
doesn't get one. Enforcement on-chain, not just alongside it.
```

---

# 5 — Custody
**1:18–1:28 · 25 words**

**Capture:** none new. The `paydevnet` settlement tx on Solana Explorer (devnet
cluster) from the submission demo, as a still or slow scroll. Or an editor-made
payer → custodian → chain diagram.

**Read:**

```
And it doesn't care who holds the keys. Self-custody or a custodian — we bind the
transfer authority, whoever holds it. We never hold keys.
```

*Backed by `settle.BoundPayment.Payer`, documented as "authorized transfer
authority (from the token)". If anyone presses, that field is the evidence. Do
**not** claim a custodian integration exists — none has been built or tested.*

---

# 6 — Why now
**1:28–1:44 · 41 words**

**Capture:** none. Editor-made standards slide, or talking head.

Deliberately short. This is a Solana audience: they need to know the gap is real,
recognised and unclaimed — not a tour of federal process.

**Read:**

```
And the timing is the moat. x402 deliberately left authorization out of scope —
and NIST and the NCCoE have both opened that exact question this year, without
picking a mechanism. The construct is still undecided. This one is built and
running.
```

*For Q&A, not the voiceover: NIST's CAISI launched its AI Agent Standards
Initiative on 17 February 2026; the NCCoE concept paper published 5 February 2026
and is now a named project, "Software and AI Agent Identity and Authorization".
It references OAuth, OIDC, SCIM and SPIFFE — as inputs, not as the answer.*

---

# 7 — The regulated pull
**1:44–2:12 · 71 words · [CUT FOR ~2:25]**

**Capture:** none new. The `gateway` transparency-log output from the submission
demo — `/transparency/root` and `"verified":true`.

**Read:**

```
Nothing is standardized and nobody has endorsed anything. That's the opening: the
construct is still undecided, and this one is built and running. Meanwhile the
FATF Travel Rule, MiCA and DORA oblige operators to attribute and keep records
for individual transfers. They don't mandate this construct — but they do make
per-transaction authorization evidence worth paying for. We emit it as a
byproduct of enforcement, with no PII on a public chain.
```

*Section 6 now carries the "undecided, and this one is running" beat, so this
whole section can go with nothing load-bearing lost.*

---

# 8 — Model and what's next
**2:12–2:41 · 73 words**

**Capture:** none. Editor-made open-core diagram, or talking head.

**Read:**

```
It's open core. The spec and the engine are open — that's the distribution and the
standards credibility. Revenue is compliance receipts and a hosted transparency
log, jurisdiction policy packs, and the gateway. And we consume the customer's
identity and policy engine — OPA, Sumsub, in-house — so we sit on top of them, not
against them. Next: the hosted transparency log and the first jurisdiction packs.
The shortest path to revenue.
```

*Keep the ask evergreen. Don't tie it to a single programme.*

---

# 9 — Close
**2:41–2:52 · 27 words**

**Capture:** none. Editor-made end card: repo, demo page, IETF draft, Zenodo DOI,
ORCID.

**Read:**

```
x402 moves the money. SPT-Txn proves the agent was allowed to. It's the
authorization layer the agent economy is missing, and it runs today. Everything's
linked below.
```

**End-card text, on screen, not spoken:**

```
Research project · not externally audited · Solana devnet
```

*Say it before anyone asks. Standing rule for external material, and on a video
making standards claims it buys more credibility than it costs.*

---

## Accuracy guardrails

- **devnet**, never mainnet. The mainnet footprints are XRPL and Ethereum and
  they do different things — don't mention them here.
- Never "audited", "validated" or "certified" about this product.
- Never "the only solution that verifies offline" — Xage and Cyolo operate
  air-gapped. The differentiator is **per-command transaction binding**.
- No bare "reproducible". Always "runnable from the README".
- No custodian-integration claim. The design accommodates custody; that is all.
- Nothing about work outside the public repositories.

## Links for the description

Both repos · the devnet settlement tx · the anchor tx · the escrow program ·
`draft-coetzee-oauth-spt-txn-tokens` on datatracker · Zenodo DOI
`10.5281/zenodo.19299787` · ORCID `0009-0009-6557-8843` · the demo page.
