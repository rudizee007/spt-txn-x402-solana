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
- Have two things ready from the `spt-txn-pep` module: its `gateway/README.md`
  one-liner, and its `go.mod`.

## Beats

| Time | Command / on screen | Say |
|------|---------------------|-----|
| 0:00–0:15 | Title, or `spt-txn-pep` `README.md` on screen | "An authorization layer only matters if people can actually adopt it. So this week the enforcement core moved into its own module — any x402 server wraps one middleware and gets per-transaction authorization plus a tamper-evident audit trail." |
| 0:15–0:30 | The one-liner in `gateway/README.md`, then `cat go.mod` — three lines, no `require` block | "This is the whole integration — you wrap your existing handler. And this is why the split mattered: the module's go.mod has no dependencies at all. Not 'few' — none, and CI fails the build if one appears. Before the split, wrapping one handler pulled a blockchain SDK and a Mongo driver into your dependency graph." |
| 0:30–1:00 | `go run ./cmd/gateway` *(five request lines print)* | "Here it is enforcing on every request. Authorized — served. The same token replayed — refused, single-use. Over budget — denied. No authorization at all — rejected. A fresh token — served again." |
| 1:00–1:20 | `/transparency/root`, then `/transparency/entry/0` — point at the matching root and at `"verified":true` | "Every decision that carried an identity emitted a Transaction Receipt — the record the spec requires — and a signed entry in a Merkle transparency log. The auditor fetches the root, then proves any single decision belongs to it — same root, inclusion proof, verified — without seeing the others. Note the count is four, not five: a request presenting no token is refused without being written, so anonymous traffic can't drive unbounded signing." |
| 1:20–1:30 | Close card | "One middleware, zero dependencies. Authorization on every request, and audit evidence as a byproduct. Proof of concept — not audited, not in production — but that's how this reaches every x402 server, not just ours." |

## Commands, in order

```sh
clear
go run ./cmd/gateway
```

Everything else is two files on screen from the `spt-txn-pep` module.

## Why the go.mod shot is the strongest fifteen seconds

Every project in this hackathon will claim to be easy to adopt. Almost none can
show it in one frame. A three-line `go.mod` with no `require` block, next to a CI
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
