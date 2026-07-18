# `demo/` — 3-minute submission video storyboard

Colosseum requires a demo video (max 3 min) and a reproducible repo. The story:
**show the gap, then show us close it — live, on devnet.**

## Storyboard (target ~2:45)

**0:00–0:25 — The gap.** "An AI agent with a funded wallet and x402 can pay
autonomously. Nothing checks whether it was *allowed* to. Watch what happens when
it's manipulated into an over-limit payment." (Show the architecture diagram; land
the one-liner: *x402 moves the money; SPT-Txn proves the agent was allowed to.*)

**0:25–1:10 — DENY.** An agent tries to pay **over its capability ceiling** (the
prompt-injection case). The gate refuses to mint the SPT-Txn → **DENY** before any
signature. Show the signed evidence record: *denied — over scope*, with the
violation-vs-outage decision class. **No payment leaves the wallet.**

**1:10–2:10 — ALLOW.** An **in-scope** payment: the gate mints the
transaction-bound token, the 8-step verifier passes offline in < 1ms, `settle/`
submits the **USDC/SOL transfer on devnet**, and the **human anchor appears
on-chain via SPL Memo**. Open the Solana explorer on the live signature — anchor
visible, **no PII**.

**2:10–2:40 — Evidence & pitch.** Show the signed receipt and (stretch) its Merkle
root anchored on-chain. Close: offline, per-transaction, human-anchored
authorization — the missing layer for the agent economy — open source, built on an
IETF draft, running on Solana.

## Reproduce it (fill commands as M0–M2 land)

```
make demo-devnet     # DENY over-scope, then ALLOW in-scope with on-chain anchor
```

Keep it real: every number on screen comes from an actual devnet transaction, and
the explorer links are live.
