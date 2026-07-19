# Eternal weekly update videos — scripts

Eternal wants a **brief** weekly video (Loom / YouTube) covering *what you
prioritized and shipped* that week. Keep each to **60–90 seconds**, founder-to-
judge tone: what you focused on, what shipped, quick proof, what's next. Screen +
voiceover is enough — no production.

Accuracy: say **devnet**, never mainnet; only claim what's built. §0 — never
mention or hint at the private on-chain-enforcement research.

---

## Week 1 — Publish & kickoff (~75s)

**Prioritized:** getting the authorization layer public and reproducible, and
starting the sprint.

| Time | On screen | Say |
|------|-----------|-----|
| 0:00–0:12 | Title: "Week 1 — SPT-Txn × x402 authorization" | "This week I prioritized getting the authorization layer public and reproducible, and kicking off the Eternal sprint." |
| 0:12–0:38 | The public repo, then `go run ./cmd/x402demo` output | "It's live. One clone and you get the whole flow: an agent hits a 402, and the gate decides before anything is signed — in-scope is served, a replay is refused, an over-budget payment is denied, and a tampered payment is refused before it touches the chain. Every decision drops a signed receipt." |
| 0:38–0:58 | Browser: the devnet USDC settlement tx, then the anchor (memo) tx | "And it's real — an authorization-gated USDC payment settled on Solana devnet, with the receipt Merkle root anchored on-chain. All reproducible from the README in about five minutes." |
| 0:58–1:12 | The `spt-txn-x402-escrow` repo | "I also published the on-chain escrow program on its own — the trustless enforcement path. Next week I wire it in: funds released only against a valid on-chain proof." |
| 1:12–1:18 | Title/end card | "That's week one: the layer is public, proven on devnet, and ready to build on." |

**Links to drop in the description:** the two repos, the settlement tx, the anchor
tx, the IETF draft, the Zenodo DOI.

---

## Reusable template (Weeks 2–4)

Four beats, ≤ 90 seconds:

1. **Prioritized (~10s)** — "This week I prioritized *[the week's theme]*."
2. **Shipped (~30s)** — screen-demo the new capability running.
3. **Proof (~15s)** — a tx link, a green test run, or a live demo output.
4. **Next (~10s)** — "Next week: *[the following theme]*."

**Week 2 — Trustless enforcement.** Shipped: the on-chain escrow wired as
release-on-proof. Proof: a devnet tx where the escrow frees funds only against a
valid proof, and fails closed otherwise. Next: the gateway form factor.

**Week 3 — Adoption surface.** Shipped: the drop-in PEP middleware (`go run
./cmd/gateway`) and the transparency-log service. Proof: authorized request
served, replay/over-budget refused, and a `/transparency/root` + inclusion proof.
Next: package and submit.

**Week 4 — Package & submit.** Shipped: the 3-minute demo video, the product
page, the final submission. Proof: the submission link. Close on the ask —
standards position and what the accelerator funds.
