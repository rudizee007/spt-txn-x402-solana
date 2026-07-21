# Demo run-sheet — command + narration (Colosseum Eternal)

Sit down, share your terminal, and follow this top to bottom. Each step is: the
**command** to run, what appears, and the **exact words** to say while it runs.
Target ~3 minutes. Say **devnet**, never mainnet. Everything here is real and
reproducible.

## Before you hit record

- Terminal: large font (~18–20pt), dark theme, window wide enough that lines don't wrap.
- `cd spt-txn-x402-solana`, then `clear`.
- Devnet wallet already funded (SOL + USDC), and do one throwaway
  `go run -tags devnet ./cmd/paydevnet -amount 100000` off-camera so the on-camera
  run is fast.
- Pre-build so there's no compile lag: `go build ./... >/dev/null 2>&1`.
- A browser tab open on Solana Explorer (devnet).
- Silence notifications.

---

## [0] Open — title card or talking head (no command, ~20s)

> "AI agents are starting to pay for things on their own with x402 — a payment
> standard built on HTTP. But x402 only answers one question: *did the money
> move?* It never checks whether the agent was *allowed* to move it — on whose
> behalf, within what limit, to whom. So when an agent gets hijacked or
> prompt-injected, x402 will happily pay the attacker. SPT-Txn fixes that: it's a
> per-transaction authorization layer, where authority lives only inside a
> short-lived token bound to one exact payment, verified offline. Let me show you."

## [1] It's built and tested (~20s)

```
go test ./...
```

*(all packages print `ok`)*

> "Everything's here and tested. The intent binding — the thing that ties a token
> to one exact payment — is differential-tested against an independent Python
> implementation. The evidence log is a proper RFC 6962 Merkle tree. Every
> fail-closed path has a negative test, and there's no custom cryptography
> anywhere."

## [2] The full flow, over real HTTP (~30s)

```
go run ./cmd/x402demo
```

*(four decision lines print, then the evidence Merkle root)*

> "Here's the whole flow over real HTTP. A resource returns a 402 — payment
> required — and the gate decides before anything is signed. In-scope: authorized,
> resource released. The same token replayed: refused — it's single-use.
> Over-budget: denied. And a tampered payment, where the money's been redirected —
> the gate approved the legitimate request, but the pre-sign guard catches the swap
> and refuses to sign. Every one of those decisions dropped a signed receipt, and
> here's the Merkle root over all of them."

## [3] Real USDC on Solana devnet (~30s) — cut to the browser when the link prints

```
go run -tags devnet ./cmd/paydevnet -amount 100000
```

*(prints "settled …" and an explorer tx link → click it / cut to the browser)*

> "Now the same thing on Solana devnet, with real USDC. The guard decodes the
> actual transaction, confirms it pays exactly the bound recipient and amount under
> my authority, and only then signs and settles. There it is on-chain — a real,
> authorization-gated USDC transfer to a fresh merchant."

## [4] The adversarial case — refused before signing (~20s)

```
go run -tags devnet ./cmd/paydevnet -amount 100000 -tamper
```

*(prints the two `[tamper]` lines, then `REFUSING TO SIGN …`, exit 1)*

> "And here's the adversarial case — same wallet, same USDC, but the transfer is
> built to a different recipient than the one that was authorized. The bound
> recipient hasn't changed, so the guard refuses to sign. Nothing is sent. Nothing
> touches the chain. The authorized payment settled; the redirected one never left
> my machine."

## [5] Evidence anchored on-chain (~15s)

```
go run -tags devnet ./cmd/anchordevnet
```

*(prints the Merkle root and a memo tx link)*

> "The compliance evidence is a byproduct. The receipt log's Merkle root is
> anchored on-chain through the SPL Memo program. Any single decision can be proven
> to belong to that batch — tamper-evident, and no personal data ever touches the
> ledger."

## [6] Drop-in for any x402 server (~20s)

```
go run ./cmd/gateway
```

*(five request lines, then `/transparency/root` and a proof with `"verified":true`)*

> "And it's a drop-in. Any x402 server wraps one middleware and gets all of this on
> every request — authorized served, replay refused, over-budget denied — plus a
> live transparency log. Fetch the anchored root, prove any single decision belongs
> to it, without exposing the others."

## [7] Close — end card (~10s)

> "x402 moves the money. SPT-Txn proves the agent was *allowed* to — per
> transaction, offline, with tamper-evident evidence. It's open source, it runs on
> devnet today, and it's the authorization layer the agent economy is missing. Repo
> and spec are in the description."

---

## All commands, in order (for a dry run)

```sh
clear
go test ./...
go run ./cmd/x402demo
go run -tags devnet ./cmd/paydevnet -amount 100000          # settle -> browser cut
go run -tags devnet ./cmd/paydevnet -amount 100000 -tamper  # REFUSING TO SIGN
go run -tags devnet ./cmd/anchordevnet                      # anchor the root
go run ./cmd/gateway                                        # PEP + transparency
```

If a devnet call is slow to confirm, keep talking or trim the pause in edit —
don't re-run mid-take. Description links to paste: the two repos, this settlement
tx, the anchor tx, the IETF draft, the Zenodo DOI (all in `SUBMISSION.md`).
