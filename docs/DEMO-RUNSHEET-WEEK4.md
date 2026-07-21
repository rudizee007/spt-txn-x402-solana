# Demo run-sheet — Week 4: The product & the vision (~2:00)

This one is less live-terminal and more narrative — the problem, everything
that's built, why now, and where it goes. Use short cuts of the earlier demos (or
the live demo page) as B-roll while you talk. Say **devnet**, never mainnet, and
keep it to what's actually built.

## Before you record

- Have the demo page (`foss.violetskysecurity.com/demo.html`) open, and the
  terminal outputs from Weeks 1–3 ready as quick cutaways.
- Optional: the two devnet Explorer tabs (settlement + anchor) open.

## Beats

| Time | On screen | Say |
|------|-----------|-----|
| 0:00–0:20 | Title, then the x402 gap (a diagram or just talking head) | "AI agents are starting to pay for things on their own with x402 — a payment standard built on HTTP. But x402 only answers one question: did the money move? It never checks whether the agent was *allowed* to. So a hijacked or prompt-injected agent will happily pay an attacker — and that's an unbounded liability. SPT-Txn closes that gap." |
| 0:20–0:55 | Quick montage: Week-1 terminal, the devnet tx, the tamper refusal, the gateway output, the escrow test | "Across these demos you've seen the whole thing work: a token bound to one exact payment, verified offline with no call home; a guard that refuses to sign a transaction that doesn't match; real USDC settling on Solana devnet; a tampered payment refused before signing; on-chain escrow enforcement; a drop-in gateway; and tamper-evident receipts anchored on-chain. All open source, all reproducible, all running on devnet today." |
| 0:55–1:25 | Standards / market slide or talking head | "And the timing is the moat. x402 deliberately left authorization, delegation, and revocation out of scope — and the whole ecosystem is now hitting that gap. NIST and NCCoE are standardizing AI-agent authorization, and the construct they're converging toward is transaction-scoped authorization — this one. Regulated money movement — the FATF Travel Rule, MiCA — needs per-transaction, provable, PII-free authorization. That's exactly what we emit as a byproduct of enforcement." |
| 1:25–1:50 | Model + what's next | "It's open core. The spec and the engine are open — that's the distribution and the standards credibility. Revenue comes from compliance receipts and a hosted transparency log, jurisdiction policy packs, and the gateway. And we consume the customer's identity and policy engine — OPA, Sumsub, in-house — so we sit *on top* of them, not against them. Next up: the hosted transparency log and the first jurisdiction packs — the shortest path to revenue." |
| 1:50–2:00 | End card: repo + demo page + IETF/Zenodo links | "x402 moves the money. SPT-Txn proves the agent was *allowed* to. It's the authorization layer the agent economy is missing — and it runs today. Everything's linked below." |

## Notes

- No new commands — this is the recap/pitch. If you want one live moment, run
  `go run ./cmd/x402demo` under beat 2 for a quick "it's real" cut.
- Keep the ask evergreen (building toward the transparency log + policy packs);
  don't tie it to any single program.
- §0: nothing here references unpublished or patent-potential work — only the
  published product.
