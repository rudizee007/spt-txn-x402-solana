# Demo run-sheet — Week 3: Adoption — drop-in gateway + transparency (~90s)

An authorization layer only matters if people can adopt it. This clip shows the
**drop-in middleware**: any x402 server wraps one line and gets per-transaction
authorization *plus* a tamper-evident audit trail — and the transparency service
that turns those decisions into verifiable evidence.

The enforcement core now lives in its own module, `spt-txn-pep`. That is the
Week 3 story, not an erratum: the split is what makes adoption cheap.

## Before you record

- Open the editor **on this repository**, never on its parent. `cd` off camera.
- `clear`, and clear the scrollback — the buffer is part of the recording.
- `go build ./... > /dev/null` so there is no compile lag on the take.
- Do one full rehearsal run of `go run ./cmd/gateway` and read the numbers. The
  receipt count is **4, not 5** — see the 1:00–1:20 beat.
- **Show the two files from GitHub, in a signed-out browser window.** A
  signed-out window *cannot* display a private repository — no repo switcher, no
  notifications, no profile page enumerating private repos. That removes the
  disclosure risk structurally instead of relying on you not clicking the wrong
  thing while narrating. It also proves to the viewer that these are the
  published artifacts rather than local files.
- Fresh private/incognito window, bookmarks bar hidden, exactly two tabs:
  - `github.com/rudizee007/spt-txn-pep/blob/main/gateway/README.md`
  - `github.com/rudizee007/spt-txn-pep/blob/main/go.mod`
- **Flip once.** Browser for both file shots (0:00–0:30), then to the terminal
  for the run and stay there through the close. Size and `clear` the terminal
  before you start so the flip lands on a clean screen.

## Beats

| Time | Command / on screen | Say |
|------|---------------------|-----|
| 0:00–0:15 | Title card, or browser tab 1 already open on `gateway/README.md` | "An authorization layer only matters if people can actually adopt it. So this week the enforcement core moved into its own module — any x402 server wraps one middleware and gets per-transaction authorization plus a tamper-evident audit trail." |
| 0:15–0:30 | Browser tab 1: the complete `gateway/README.md` page, scrolling past the `NewPEP` example. Then tab 2: `go.mod` — **two lines**, a module path and a Go version, no `require` block | "This is the whole integration — you wrap your existing handler. And this is why the split mattered: the module's go.mod has no dependencies at all. Not 'few' — none, and CI fails the build if one appears. Before the split, wrapping one handler pulled a blockchain SDK and a Mongo driver into your dependency graph." |
| 0:30–1:00 | `go run ./cmd/gateway` *(five request lines print)* | "Here it is enforcing on every request. Authorized — served. The same token replayed — refused, single-use. Over budget — denied. No authorization at all — rejected. A fresh token — served again." |
| 1:00–1:20 | `/transparency/root`, then `/transparency/entry/0` — point at the matching root and at `"verified":true` | "Every decision that presented a token emitted a Transaction Receipt — the record the spec requires — and a signed entry in a Merkle transparency log. The auditor fetches the root, then proves any single decision belongs to it — same root, inclusion proof, verified — without seeing the others. Note the count is four, not five: a request presenting no token is refused without being written, so anonymous traffic can't drive unbounded signing." |
| 1:20–1:30 | Close card | "One middleware, zero dependencies. Authorization on every request, and audit evidence as a byproduct. Proof of concept, not audited, not in production — but that's how this reaches every x402 server, not just ours." |

## Commands, in order

```sh
clear
go run ./cmd/gateway
```

Everything else is the two GitHub tabs above — no local commands for those.

## Why the go.mod shot is the strongest fifteen seconds

Every project in this hackathon will claim to be easy to adopt. Almost none can
show it in one frame. A two-line `go.mod` with no `require` block, next to a CI
job named *zero-dependency invariant* that fails the build if one is added, is a
claim a viewer verifies rather than believes. Same move as the escrow admin in
Week 2 and `Conformant()` this week: don't ask to be trusted, make the property
structural.

## Accuracy guardrails

- **This demo touches no chain.** It runs an in-process `httptest` server. Don't
  say devnet here, and don't imply the Merkle root was anchored during the run —
  it is the value that *gets* anchored, which is a different claim.
- **Four receipts, five requests.** A request with no parseable token is refused
  before anything is signed or chained, deliberately, so unauthenticated traffic
  cannot grow the log. Narrate it as a feature rather than hoping nobody counts.
- **"Not externally audited, not in production"** belongs in the close.
- **§0** — nothing about the private on-chain-enforcement research or the
  jurisdiction policy packs.
- The receipts here come from a worked-example emitter in `cmd/gateway`. It
  deliberately does not sign or canonicalize — that belongs next to the verifier
  in the engine. If you say "signed", mean the transparency-log entries, which
  are.
- **The demo's tokens are not signed credentials.** `gateway.EncodeToken` emits
  base64 of `{nonce, expiry}` — no signature, no CAT, no delegation chain. What
  this demo proves is the enforcement *shape*: deny-by-default, single-use
  nonces, an amount ceiling, fail-closed, and evidence on every decision. The
  eight-step cryptographic verifier lives in the engine and is not wired to this
  edge path yet — see `docs/PRESENTATION-SEAM.md` in the `spt-txn-pep` module. Do not say or imply that a token
  signature is being checked here; a judge who opens `EncodeToken` sees it in
  five seconds, and the honest version is a better story anyway.

## Video description

```
Week 3 of the Colosseum Eternal sprint — adoption.

The SPT-Txn enforcement core is now its own Go module with zero dependencies —
no require block, no go.sum, enforced by CI. Any x402 resource server wraps one
middleware and gets per-transaction authorization, a Transaction Receipt at
every decision, and an RFC 6962 Merkle transparency log with inclusion proofs.

Enforcement points: https://github.com/rudizee007/spt-txn-pep
x402 reference:     https://github.com/rudizee007/spt-txn-x402-solana
Escrow program:     https://github.com/rudizee007/spt-txn-x402-escrow
IETF draft: https://datatracker.ietf.org/doc/draft-coetzee-oauth-spt-txn-tokens
Paper: https://doi.org/10.5281/zenodo.19299787

Proof of concept. Not externally audited, not for production use.
```
