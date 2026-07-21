# Demo run-sheet — Week 3: Adoption — drop-in gateway + transparency (~90s)

An authorization layer only matters if people can adopt it. This clip shows the
**drop-in middleware**: any x402 server wraps one line and gets per-transaction
authorization *plus* a tamper-evident audit trail — and the transparency service
that turns receipts into verifiable compliance evidence. Say **devnet**, never
mainnet.

## Before you record

- `cd spt-txn-x402-solana`, then `clear`.
- `go build ./... >/dev/null 2>&1` so there's no compile lag.
- Have `gateway/pep.go` (or `gateway/README.md`) ready to show the one-liner.

## Beats

| Time | Command / on screen | Say |
|------|---------------------|-----|
| 0:00–0:15 | Title, or `gateway/README.md` on screen | "The whole point of an authorization layer is that people can actually adopt it. So we made it a drop-in — any x402 server wraps one middleware and gets per-transaction authorization plus a tamper-evident audit trail, for one line of code." |
| 0:15–0:30 | Show the one line in `gateway/README.md`: `http.Handle("/premium", pep.Wrap(myHandler))` | "This is the whole integration. You wrap your existing handler — that's it. Your policy engine, OPA or Sumsub or in-house, plugs in behind it; SPT-Txn adds the per-transaction enforcement and the evidence." |
| 0:30–1:00 | `go run ./cmd/gateway` *(five request lines print)* | "Here it is enforcing on every request. An authorized request is served. The same token replayed — refused, single-use. An over-budget request — denied. No authorization — rejected. And a fresh authorized request is served again. Every one of those decisions dropped a signed receipt." |
| 1:00–1:20 | Same output — point at `/transparency/root` and `/transparency/receipt/0` with `"verified":true` | "And here's the transparency service. An auditor fetches the Merkle root — the same value we anchor on-chain — and can prove any single decision belongs to it, with its inclusion proof, without ever seeing the other receipts. Compliance evidence, as a service." |
| 1:20–1:30 | Close card | "One middleware. Authorization on every request, and audit evidence for free. That's how this reaches every x402 server, not just ours." |

## Commands, in order

```sh
clear
# show gateway/README.md — the pep.Wrap(...) one-liner
go run ./cmd/gateway
```

The `gateway` command prints the five enforced requests, then the transparency
root and a verifiable inclusion proof — all in one run, no external accounts.
