# SPT-Txn × x402 on Solana

**The authorization layer for the agent economy.**

x402 answers *"did the money move?"* It says nothing about *"was this agent
**allowed** to move it — on whose behalf, within what limit, under what policy,
and can that authority be delegated and revoked?"* That gap is where a
compromised or prompt-injected agent does real damage.

**SPT-Txn fills that gap.** It is a scoped, provable, transaction-bound
authorization token. Authority exists only inside a short-lived token bound to
**one declared payment**, on **one resource**, for **one accountable human**,
verified **offline in well under a millisecond with no call home**. A compromised
agent holds a token that is cryptographically useless for any payment other than
the one it declared.

This repository is the **Solana + x402 integration** of that engine, built on the
open [SPT-Txn reference implementation](https://github.com/rudizee007/spt-txn-poc).

---

## The one-line pitch

> **x402 moves the money. SPT-Txn proves the agent was allowed to — offline,
> per-transaction, with a human anchor and a tamper-evident receipt.**

---

## What it does

An AI agent hits a resource, gets an **HTTP 402 Payment Required**, and *before*
it signs anything on Solana:

1. The **SPT-Txn gate** computes a fixed-width intent binding over the exact x402
   payment (scheme, network, asset, pay-to, amount, resource) and runs the
   pluggable policy. In scope → **ALLOW**; over scope, expired, or replayed →
   **DENY**, with a distinct *violation* vs *unavailable* class so an operator can
   tell an attack from an outage.
2. On ALLOW, the settler builds the real USDC `TransferChecked` and a **pre-sign
   guard** refuses to sign unless the transaction pays *exactly* the bound
   recipient / asset / amount under the payer's authority. Only then does it sign
   and settle on devnet.
3. Every decision emits a **signed, hash-chained receipt**; the log's RFC 6962
   Merkle root is anchored on-chain via the SPL Memo program — a tamper-evident
   evidence trail, with no PII on the ledger.

Compliance evidence is a byproduct of enforcement, not an after-the-fact audit.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and the diagram
[`docs/architecture.svg`](docs/architecture.svg).

---

## Status

| Piece | State |
|---|---|
| Offline authorization engine (verifier, attenuating delegation) | **Built** — reused from the SPT-Txn reference implementation |
| x402 intent binding + payer-gate (ALLOW / two DENY classes) | **Built & tested** — differential Go/Python KAT |
| HTTP x402 flow (real `402` + `X-PAYMENT` round-trip) | **Built & tested** |
| Pre-sign `TransferChecked` guard | **Built & tested** |
| **USDC** settlement on **devnet** | **Built & proven on devnet** |
| Signed receipts + RFC 6962 Merkle log | **Built & tested** |
| On-chain evidence anchor (Merkle root via SPL Memo) | **Built & proven on devnet** |
| Gateway / PEP middleware (drop-in x402 authorization) | **Built & tested** |
| Transparency-log service (receipts as an HTTP API) | **Built & tested** |
| Option 2: on-chain enforcement (Anchor program) | **Published separately** — `spt-txn-x402-escrow`, devnet-deployed |
| Mainnet | Gated; devnet-first by default |

Nothing here is overstated: what's marked *Built* runs today (`go test ./...`),
and *proven on devnet* means a real confirmed transaction. See
[`docs/BUILD-PLAN.md`](docs/BUILD-PLAN.md) for what ships and in what order.

---

## Quickstart (reproducible in ~5 minutes)

Everything except the two devnet commands runs **offline, no accounts**:

```sh
go test ./...          # gate + settle + receipt + demo, incl. differential KATs
go run ./cmd/demo      # 4 scenarios + the evidence Merkle root, in process
go run ./cmd/x402demo  # the same, over a real HTTP 402 / X-PAYMENT exchange
go run ./cmd/gateway   # drop-in PEP middleware + transparency-log endpoints
```

You should see: an authorized payment released, a replay refused, an over-scope
payment denied, a tampered payment **refused before signing**, and a signed-receipt
Merkle root.

**On devnet (real USDC, your own keypair):**

```sh
go get github.com/gagliardetto/solana-go
# fund ~/.config/solana/id.json with devnet SOL (fees) + devnet USDC:
#   SOL  -> https://faucet.solana.com      USDC -> https://faucet.circle.com
go run -tags devnet ./cmd/paydevnet -amount 100000          # settles 0.10 USDC
go run -tags devnet ./cmd/paydevnet -amount 100000 -tamper  # guard REFUSES TO SIGN
go run -tags devnet ./cmd/anchordevnet                      # anchors the receipt root
```

The `devnet` build tag keeps the key/network path out of the default build; your
signing key stays in the keypair file and is never read into the process env.

---

## Repository layout

```
gate/       Off-chain x402 authorization gate (Go): fixed-width intent binding +
            ALLOW/DENY decision. No Solana SDK in this path.
settle/     Pre-sign TransferChecked guard + the real SPL/USDC transfer builder.
receipt/    Signed receipts + RFC 6962 Merkle log (tamper-evident evidence).
gateway/    Drop-in x402 authorization (PEP) middleware + transparency-log service.
demo/       In-process and HTTP x402 loops used by the demos.
cmd/        demo, x402demo, gateway (offline); paydevnet, anchordevnet (devnet).
docs/       Spec (SPEC-X402), architecture, build plan, monetization, sprint plan.
```

## Design invariants (non-negotiable)

- **Deny by default, fail closed.** Timeout, malformed token, unreachable
  revocation, over-scope → **DENY** with an evidence record explaining why.
- **No ambient authority.** Authority lives only in the short-lived,
  transaction-scoped token — never in a role, a network location, or a
  long-lived key.
- **Attenuation is monotonic and offline-verifiable.** Every delegation step can
  only *narrow* authority. Child TTL strictly less than parent.
- **No custom cryptography.** Standard, audited primitives only (`crypto/ed25519`,
  `crypto/sha256`, an RFC 6962 Merkle tree, SPL Memo). No hand-rolled
  constructions; the binding and Merkle layouts are differential-tested against an
  independent Python implementation.
- **No PII on-chain.** Only hashes touch the ledger — the receipt Merkle root —
  never an identity or a cleartext amount.

## Security note

No key material lives in the repo. The devnet tools read your signing key from a
Solana keypair file (`~/.config/solana/id.json` by default) — never an env var,
never a flag, never committed. `*keypair*.json`, `.env`, and build artifacts are
git-ignored. Mainnet is intentionally not wired.

## License

Apache-2.0. See [`LICENSE`](LICENSE). Built on the open SPT-Txn reference
implementation and the IETF `draft-coetzee-oauth-spt-txn-tokens`.
