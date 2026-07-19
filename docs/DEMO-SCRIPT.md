# Demo video — storyboard & script (target 3:00)

The hook is one real setup showing **two outcomes**: an authorized USDC payment
settles on devnet, and a tampered one is **refused before signing**. Everything on
screen is live and reproducible from the README quickstart — no slideware.

Tone: calm, technical, confident. Let the terminal do the talking. Total spoken
words ≈ 430 (≈150 wpm).

---

## Pre-flight checklist (before you hit record)

- Terminal font large (≈18–20pt), dark theme, wide enough that no line wraps.
- `cd spt-txn-x402-solana`; clear scrollback (`clear`).
- Devnet wallet funded: a little SOL (fees) + USDC (faucet.circle.com). Do a
  throwaway `go run -tags devnet ./cmd/paydevnet -amount 100000` off-camera so the
  ATA exists and the on-camera run is fast.
- Browser open to a blank Solana Explorer tab (devnet cluster preselected).
- Pre-build so there's no compile lag on camera: `go build ./... >/dev/null`.
- Close notifications / hide anything private (the payer address is fine to show).

---

## Beat sheet

| # | Time | On screen | Say (verbatim) |
|---|------|-----------|----------------|
| 1 | 0:00–0:22 | Title card: **"SPT-Txn × x402 — authorization for agent payments"**, then cut to terminal | "x402 lets an AI agent pay for things over HTTP. It answers *did the money move* — but nothing checks whether the agent was *allowed* to move it. When an agent is prompt-injected or hijacked, x402 will faithfully pay the attacker. That gap is what we close." |
| 2 | 0:22–0:40 | Terminal, `README.md` open or the one-line pitch on screen | "SPT-Txn is a per-transaction authorization layer. Authority exists only inside a short-lived token bound to one exact payment — one asset, one amount, one recipient — verified offline, with no call home. A hijacked agent holds a token that's cryptographically useless for any other payment." |
| 3 | 0:40–1:05 | Run `go test ./...` — all packages `ok` | "It's all here and tested. The intent binding is differential-tested against an independent Python implementation, the Merkle log is RFC 6962, and every fail-closed path has a negative test. No custom cryptography." |
| 4 | 1:05–1:35 | Run `go run ./cmd/x402demo` — the four lines print, then the evidence root | "Here's the real HTTP flow. A 402 payment request comes back; the gate decides. In-scope: allowed, resource released. A replayed token: refused. An over-budget payment: denied before signing. And a *tampered* payment — the gate allowed the legitimate request, but the pre-sign guard sees the money is being redirected and refuses to sign. Every decision drops a signed receipt; here's the Merkle root over all of them." |
| 5 | 1:35–2:15 | Run `go run -tags devnet ./cmd/paydevnet -amount 100000`; when it prints the tx link, **cut to browser**, paste link, show the confirmed USDC transfer | "Now on Solana devnet, with real USDC. The guard checks the actual transaction pays exactly the bound recipient and amount, under my authority — then signs and settles. There it is on-chain: a real, authorization-gated USDC transfer." |
| 6 | 2:15–2:38 | Back to terminal, run `go run -tags devnet ./cmd/paydevnet -amount 100000 -tamper` → **`REFUSING TO SIGN`**, exit 1 | "Same wallet, same USDC — but now the built transfer points at a different recipient. The bound payTo hasn't changed, so the guard refuses to sign. Nothing touches the chain. One setup: the authorized payment settled; the redirected one never left the machine." |
| 7 | 2:38–2:50 | Run `go run -tags devnet ./cmd/anchordevnet`; show root + memo tx link (optional quick browser cut to the memo) | "And the evidence is a byproduct: the receipt Merkle root is anchored on-chain via SPL Memo. Any single decision can be proven to belong to that batch — tamper-evident, no PII on the ledger." |
| 7b *(optional)* | 2:50–3:05 | Run `go run ./cmd/gateway` — the enforcement lines, then `/transparency/root` and `"verified":true` | "And it drops in: any x402 server wraps one middleware to get this on every request — served, replay refused, over-budget denied — plus a live transparency log. Fetch the anchored root, prove any single decision belongs to it, without seeing the others." |
| 8 | 3:05–3:12 | Title/end card: repo + links (IETF draft, Zenodo DOI) | "x402 moves the money. SPT-Txn proves the agent was allowed to. Repo and spec in the description." |

---

## Exact on-camera command order

```sh
clear
go test ./...
go run ./cmd/x402demo
go run -tags devnet ./cmd/paydevnet -amount 100000          # settles -> browser cut
go run -tags devnet ./cmd/paydevnet -amount 100000 -tamper  # REFUSING TO SIGN
go run -tags devnet ./cmd/anchordevnet                      # anchors the root
go run ./cmd/gateway                                        # optional beat 7b — PEP + transparency
```

Core cut runs a tight 3:00 (beats 1–8); including the optional gateway beat 7b
takes it to ~3:12.

If a devnet call is slow to confirm on camera, cut the dead air in edit; do not
re-run mid-shot.

## Lower-thirds (optional captions)

- Beat 3: `differential-tested · RFC 6962 · fail-closed`
- Beat 4: `402 → gate → guard → settle, all enforced`
- Beat 5: `real USDC · Solana devnet`
- Beat 6: `refused before signing — funds never move`
- Beat 7: `evidence = byproduct of enforcement`
- Beat 7b: `drop-in middleware · live transparency log`

## 60-second cut (if a short version is needed)

Beats 1 (trimmed to the gap), 4 (the four-line demo), 6 (the tamper refusal), 8.
That alone tells the whole story: the problem, the enforcement, the on-chain
refusal, the ask.

## Accuracy guardrails (do not overclaim)

- Say **devnet**, never mainnet. Mainnet is intentionally not wired.
- Don't call the receipt anchor a "human anchor" — it's a Merkle root of decision
  receipts. Only hashes go on-chain.
- Everything shown is Option 1 (off-chain gate + settlement guard + receipts). Do
  not mention or hint at any unpublished on-chain-enforcement work.
- The numbers on screen are the truth; let them stand without embellishment.
