# Pitch video — 2 minutes (Colosseum Eternal, required, PUBLIC)

**RECORDED AND PUBLISHED 2026-08-22 — https://youtu.be/-I6ZQ6C7sms**

Before pasting into the form, confirm in YouTube Studio: **visibility Public or
Unlisted (never Private)** and **duration at or under 2:00**. This field is
marked PUBLIC, so it appears in the project directory.

Colosseum's brief, verbatim:

> *Separate from the demo video — introduce yourselves, tell us what you're
> building, and tell us why you're the people to build it. Nothing fancy
> required. We're interested in how you think and communicate. Up to 2 minutes.*

**This is a talking head. Camera on you, not the screen.** "Nothing fancy
required" is a real instruction — they are assessing judgement and clarity, not
production value. No slides, no B-roll, no music. Look at the lens.

**282 words ≈ 1:53** at a natural speaking pace, leaving margin. Four blocks.
Learn the shape, don't read it word for word — reading aloud is audible and it
undercuts the one thing they said they're measuring.

---

# 1 — Who
**0:00–0:15 · 38 words**

```
I'm Rudi Coetzee, founder of Violet Sky Security, a Cayman Islands SEZC. I've
spent my career in security architecture — I hold the CISSP ISSAP, ISSMP and
CSSLP. I'm building SPT-Txn on my own, and this is week four.
```

---

# 2 — What I'm building
**0:15–0:47 · 80 words**

```
SPT-Txn is a per-transaction authorization layer for agent payments. x402 lets
an AI agent pay over HTTP, and it proves the money moved — but nothing checks
the agent was allowed to move it. A prompt-injected agent pays an attacker
faithfully. So we bind authority into a short-lived token: one asset, one
amount, one recipient, verified offline. A guard refuses anything that doesn't
match, and on-chain escrow releases only against a valid proof. Every decision
leaves a signed receipt.
```

---

# 3 — Why me
**0:47–1:23 · 90 words**

```
Why me. I wrote the specification before I wrote the code. There's an IETF
Internet-Draft, and a formal paper with five game-based security proofs
published with a DOI — because in authorization, a construct you can't reason
about formally is a construct you can't trust. I've also been deliberate about
what I don't claim: nothing here has been externally audited, nothing is in
production, and everything runs on devnet. I'd rather a judge find that on the
tin than find it in the code.
```

That last sentence is doing real work. Every submission claims to be solid; very
few volunteer their limits. Stating them is the most credible thing you can do in
a two-minute video, and it inoculates you against the diligence that follows.

---

# 4 — Why now, and what's next
**1:23–1:53 · 74 words**

```
And the timing matters. NIST and the NCCoE have both opened the question of how
AI agents get authorized, and neither has picked a mechanism. That window
doesn't stay open. Over four weeks I shipped the working loop, on-chain escrow
enforcement, a drop-in gateway, and a live agent paying over MCP — all open
source, all on devnet. Next is the hosted transparency log and the first
jurisdiction policy packs. Thanks for watching.
```

---

## Setup

- **Camera at eye level**, not below. A laptop on a stack of books beats a laptop
  on a desk.
- **Window light in front of you**, never behind.
- **Audio matters more than picture.** Earbuds with a mic beat a laptop
  microphone across a room. Record somewhere soft — a room with curtains and a
  rug, not a bare kitchen.
- One take, whole thing, then a second for safety. Pick the better one. Do not
  cut between blocks; a continuous take reads as confidence.

## Guardrails

- Never "audited", "validated" or "certified".
- Say **devnet**. Do not imply Solana mainnet.
- Do not claim to be the only solution that verifies offline.
- Nothing about work outside the public repositories.
- Don't mention the weekly-update videos or the demo video. This is a separate
  artifact and should stand alone.
