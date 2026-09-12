# Security

## Reporting a vulnerability

Report privately to **rudi@violetskysecurity.com**. Please allow reasonable time
for remediation before public disclosure; coordinated disclosure is appreciated.

The on-chain component lives in a separate repository and has its own policy:
[`spt-txn-x402-escrow/SECURITY.md`](https://github.com/rudizee007/spt-txn-x402-escrow/blob/main/SECURITY.md).

## What this code is trusted to do

This repository is the off-chain half of the system: the SPT-Txn gate, the
settler, the receipt log, and the gateway and MCP front ends. Five properties
carry the weight.

**Authorization is decided before anything is signed.** The gate computes a
fixed-width intent binding over the exact x402 payment — scheme, network, asset,
pay-to, amount, resource — and evaluates policy against it. A DENY is returned
with a distinct *violation* versus *unavailable* class, so an operator can tell
an attack from an outage rather than having both fail closed into the same
silence.

**The pre-sign guard is the last line, and it is independent of the gate.** The
settler refuses to sign unless the constructed transaction pays exactly the
amount, asset and recipient that were bound. A gate that was wrong, bypassed, or
compromised still does not produce a signature for an unbound transfer.

**The escrow client names its own releasing issuer.** `init_escrow` carries the
issuer public key as its fourth argument, and the on-chain program stores it
immutably: the payer, not the operator, decides which key can ever release that
deposit. The encoding of those three consecutive 32-byte arguments is pinned by
a known-answer test (`escrow/anchor_test.go`) because Borsh carries no field
names — a transposition would pass a length check and silently pin a key the
payer never chose.

**Key material never enters the repository.** Solana keypairs are read from a
path supplied at runtime (`SPT_KEYPAIR`, or the `-gen-issuer` / `-gen-admin` key
files written at mode 0600 under `~/.config/spt-txn/`), never from source and
never from committed configuration. The devnet driver keeps the payer, issuer
and issuer-admin keys in three separate files precisely because the on-chain
program refuses to let one key hold two of those roles. `.gitignore` refuses `*keypair*.json`, `*-devnet.json`,
`*-mainnet.json`, `id.json`, `*.pem`, `*.key` and `.env*`. `git ls-files`
returns no JSON file and no key file in this repository. Devnet keys are treated
as secrets on the same terms as mainnet keys, because the habit is what fails,
not the network.

**Receipts are evidence, and are written as such.** The receipt store creates
its file at mode 0600. The receipt-signing private key is never serialized into
it. Per-run outputs (`receipts.json`, `escrow-ticket.json`) are gitignored:
committing them would invite anchoring a stale root.

## Dependency and vulnerability audit status

Last run **2026-09-12**, Go **1.25.14** (the `toolchain` line in `go.mod`),
`govulncheck` v1.7.0 against the Go vulnerability database. CI also runs
`govulncheck` on every push to `main`, on the newest Go 1.25 patch release.

The test suite passes and both scan configurations report **no reachable
vulnerabilities**:

```
go test ./...                       # all packages pass
govulncheck ./...                   # No vulnerabilities found.
govulncheck -tags devnet ./...      # No vulnerabilities found.
```

**That result is a statement about the toolchain the scan ran with.** Go 1.25.12 —
this repository's `toolchain` line before 2026-09-12 — has five standard-library
advisories reachable from this repository's own code: the HTTP server in
`cmd/gateway`, the HTTP client in `demo`, and the devnet drivers (GO-2026-5026,
GO-2026-5972, GO-2026-6089, GO-2026-6090, GO-2026-6218). All five are fixed in Go
1.25.13. A binary built with an older toolchain carries them regardless of what this
section says. `go run` with a local Go older than the `toolchain` line downloads that
toolchain; a newer local Go is used as it is.

The `-tags devnet` run matters and is not optional. Every code path that handles
a real key, builds a real transaction, or talks to a live RPC endpoint is behind
that build tag. A clean scan that excluded exactly those files would be a claim
about the code that does the least and says nothing about the code that does the
most.

### Module-level findings

`govulncheck -show verbose` reports four results under **Module Results**, the same
four with and without `-tags devnet`. Each means the vulnerable module is in the build
graph but no vulnerable symbol is reachable from any entry point in this repository.
All four are in `golang.org/x/crypto@v0.54.0`, which reaches the build graph
transitively through `github.com/gagliardetto/solana-go`:

| Advisory | Package | Fixed in | Status |
|---|---|---|---|
| [GO-2026-6355](https://pkg.go.dev/vuln/GO-2026-6355) | `ssh` | `v0.56.0` | Not imported, not reachable. |
| [GO-2026-6354](https://pkg.go.dev/vuln/GO-2026-6354) | `ssh` | `v0.56.0` | Not imported, not reachable. |
| [GO-2026-6303](https://pkg.go.dev/vuln/GO-2026-6303) | `ssh` | `v0.55.0` | Not imported, not reachable. |
| [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) | `openpgp` | N/A | Not imported, not reachable. |

Nothing in this repository imports `golang.org/x/crypto/ssh`. The three `ssh`
advisories have fixes in newer `golang.org/x/crypto` releases; the module version is
chosen by the dependency graph, and raising it would be a separate change that has not
been made.

`golang.org/x/crypto/openpgp` is deprecated and unmaintained — unsafe by design
rather than carrying a specific exploitable defect — and its advisory records
`Fixed in: N/A`. It will therefore never clear, at any version. It reaches the
build graph transitively through `github.com/gagliardetto/solana-go`; nothing in
this repository imports it, and `govulncheck`'s symbol analysis confirms no call
path reaches it. There is no patch to apply and no version to bump to.

Recording it here rather than omitting it is deliberate: a security document that
reports only the findings that happen to be clean is not evidence of anything.

### Reproduce

```sh
go test ./...
govulncheck ./...
govulncheck -tags devnet -show verbose ./...
```

## Rust dependencies

The Anchor program is not in this repository. `cargo audit` results for it, the
reachability argument for each finding, and the reason GitHub's Dependabot badge
overstates them are documented in
[`spt-txn-x402-escrow/SECURITY.md`](https://github.com/rudizee007/spt-txn-x402-escrow/blob/main/SECURITY.md#dependency-audit-status).

Summary: two `cargo audit` vulnerabilities, both arriving through
`[dev-dependencies]` → `litesvm`, neither compiled into the deployed program
binary.

## Scope

This is a proof of concept on Solana **devnet**. It has not been audited by a
third party and has not held mainnet value. Treat the threat model, not the
uptime, as the deliverable.
