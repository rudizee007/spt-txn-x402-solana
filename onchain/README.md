# `onchain/` — on-chain enforcement (release-on-proof escrow)

**Built and proven on devnet.** The Anchor program lives in its own repository,
[`spt-txn-x402-escrow`](https://github.com/rudizee007/spt-txn-x402-escrow), and is
deployed at
[`C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ`](https://explorer.solana.com/address/C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ?cluster=devnet).
The Go half of the integration is in this repository: the `escrow/` package
(bindings, attestations, PDA derivation, Ed25519 precompile encoding — nothing
beyond the standard library) and `cmd/escrowdevnet`, which drives the whole path
end to end on devnet.

This directory is the map between the two.

## Proven on devnet

Every claim below is a confirmed transaction, not a test fixture.

| Step | What it establishes | Transaction |
|---|---|---|
| `init_config` | Config created with an **empty** allowlist — deny-by-default, on chain. The upgrade authority signs, but names a *different* key as admin; the account constraints reject the transaction if one key would hold both roles | [tx](https://explorer.solana.com/tx/3Xx65mAHMC3Cu7YrPbvmUrCxHzSXpCQYbNqJ693t33XUWM78GKLkXyPuFynijapKoovpNsJgeuijsck655U9Y4Fv?cluster=devnet) |
| `add_issuer` | The issuer is authorized in a *separate* transaction, so the deny-by-default state existed on its own | [tx](https://explorer.solana.com/tx/5zY1CUym2A9eYnaGuz7f43SEJY6FQZ15cj9iuk23MGfTSvf1eXzjvSSLzNNAF1UUriAVwxygaPfk3ksNyT7vcBxZ?cluster=devnet) |
| `init_escrow` | Funds move into the vault PDA and the payer **pins** the one issuer permitted to release this escrow; the config account is not even touched, so custody setup asserts no authorization | [tx](https://explorer.solana.com/tx/2L4HCrVkh1yRrKtASvXntzWhWFX8Q4Bqy6JSzaRwXPCgkHTHhgVdBykFgtbkfLrQ34Uw1d1Y7ZC58NEAT4BhRJLU?cluster=devnet) |
| `release_with_proof` | **DENIED `6105 BindingMismatch`** — genuine signature, authorized issuer, but the attestation is bound to a different payment | [tx](https://explorer.solana.com/tx/2LhQ55BWtLTF4LrBF7Qm9vsCPReXr8Bk6RUqWHM7SJ6iwJhCF2xWM2cxJ4u99XUrdbBiT26ssVFDBNo9otTzJLmm?cluster=devnet) |
| `release_with_proof` | **DENIED `6102 IssuerNotAuthorized`** — genuine signature, correct binding, issuer not on the allowlist | [tx](https://explorer.solana.com/tx/2PdFXekEZHf6SD3NLzEY7FGcBZB8484yF2FA9u9A1qBNBuRzfYKz68tAPxvwfKYrKSxegvZ9RNqVLf7ia8LstBZc?cluster=devnet) |
| `add_issuer` (rogue) | **SUCCEEDS.** Run with the admin key in hand: a compromised admin really can allow-list an issuer it controls, and here it lands on chain | [tx](https://explorer.solana.com/tx/576zuHiQqgc8jNTGpgpAKzfLV5Ry7ZihJF9g9Ffj3CVYKXVzNc2F8zFrvBvcB5FFhH56PtUGswBKT5WuboLXYgBs?cluster=devnet) |
| `release_with_proof` | **DENIED `6108 IssuerNotPinned`** — that newly allow-listed rogue issuer signs a valid, correctly bound, fresh attestation and still cannot release, because it is not the issuer the payer pinned at deposit. This is the compromised-admin result | [tx](https://explorer.solana.com/tx/5tVnrNwESCNawdLEB7Q6r2wVys2rEvpRMZrTQUaWEwz5roE2W6fypJ2V8yx1WtJTKFMr3UzVcJqsAEKpy6jGsC6Y?cluster=devnet) |
| `remove_issuer` | The rogue issuer is revoked; allow-list changes take effect on the next block | [tx](https://explorer.solana.com/tx/5q9joyuykk3xTBzqj3EPhHyJthDmjL4d98eCRzEPTCEpXYRmvU8YQLqtdgwXJfSfW9E7uXfrcxjdGcXLUCtkH3ib?cluster=devnet) |
| `release_with_proof` | **RELEASED** — funds to the recipient, escrow and vault closed, spent marker created | [tx](https://explorer.solana.com/tx/3wAaBFu2mRJtGq2MZJ5RQFf9iwaKyTyVrETpLySYrMb9uxbQ74bt6bdhSNS7uuMakuY1eddx5Tu5EC4kheWugiVw?cluster=devnet) |

These ran against [this program upgrade](https://explorer.solana.com/tx/53G8LuvdfBucEKEkqSEVvxAvYwbzBCogQsFaAf2BiZvCt5ma7bnUfMX8tsaGLnwwdawyRUEK6s5aPRw4pcFqY7QZ?cluster=devnet), which is the hardened
build carrying issuer pinning and role separation.

Reproduce with `go run -tags devnet ./cmd/escrowdevnet -mode all`, or the
compromised-admin case on its own with `-mode deny-unpinned`. The tool fails
loudly in both directions: if a transaction expected to revert instead succeeds,
it stops and reports that the enforcement property does not hold. Note that the
`add_issuer` (rogue) step is expected to **succeed** — a run in which it failed
would prove nothing about what the pin protects.

## What changes when enforcement moves on-chain

The gate (`gate/`) decides ALLOW or DENY before anything is signed. The pre-send
guard (`settle/`) asserts what a transaction pays before it is signed. Both are
the right control for a cooperating client, and both are ultimately the payer's
own code refusing to act — nothing stops a payer who deletes the guard.

The escrow closes that gap. Funds move into a vault owned by a program-derived
address, and the only exit to the recipient is `release_with_proof`, which the
program executes solely against a fresh, allowlisted, correctly-bound issuer
attestation. The payer's cooperation stops being part of the trust model.

## The program

1. Holds the x402 payment in an escrow vault (`init_escrow`). Custody setup
   asserts *no* authorization — anyone may fund an escrow; it is the release that
   has to prove itself.
2. Verifies no signature itself. Verification is done by Solana's native
   **Ed25519 precompile**; the program reads the Instructions sysvar, locates the
   precompile instruction, and extracts the `(pubkey, message)` pair the runtime
   has *already* verified — rejecting any instruction whose offsets point outside
   itself.
3. Reconstructs the **escrow binding** — a domain-separated hash over payer, mint,
   amount, recipient, resource and nonce — on chain, at deposit time. The caller
   never supplies it.
4. Releases **only** on a fresh, in-scope proof from an allowlisted issuer;
   otherwise fails closed. A permanent spent marker makes every binding
   single-use. If no valid proof ever arrives, `refund_expired` returns the funds
   to the stored payer and to no one else.

Deny-by-default throughout: the issuer allowlist starts empty, and `init_config`
can only be run by the program's recorded upgrade authority.

## Two bindings, deliberately not substitutable

| | domain tag | amount width | recipient field |
|---|---|---|---|
| gate | `spt-txn/x402-payment/v1` | u128 (16-byte LE) | `payTo` — a **token account** |
| escrow | `spt-txn/x402-escrow-binding/v1` | u64 (8-byte LE) | `recipient` — a **wallet** |

They share only the nonce (the token's `jti`), which is what ties one gate
decision to one escrow release. The distinct domain tags mean neither value can
ever be presented in place of the other. `escrow.FromGate` is the only supported
way to get from one to the other, and it *verifies* rather than trusts the claim
that the recipient wallet owns the gate's `payTo` token account.

## Highest-risk step (do not skip)

The on-chain canonicalization **must** match the off-chain implementation
byte-for-byte. A mismatch between issuer and verifier canonicalization is the #1
authorization-bypass class in this design.

It is handled by differential tests rather than by inspection: `escrow/` is tested
against the Rust program's own fixtures, including the exact precompile header
bytes and a pinned binding known-answer test. `cmd/escrowdevnet` additionally
derives every PDA twice — once with this repository's dependency-free
implementation, once with `solana-go` — and refuses to sign if the two disagree.

## Running it

```sh
go run -tags devnet ./cmd/escrowdevnet -gen-issuer   # once: create the issuer key (0600)
go run -tags devnet ./cmd/escrowdevnet -mode setup   # once per deployment: config + allowlist
go run -tags devnet ./cmd/escrowdevnet -mode all     # deposit, two denials, release
```

`-mode all` lands four transactions: the deposit, a release attempt with a
*validly signed* attestation over the wrong binding (reverts `6105
BindingMismatch`), a release attempt from a *validly signed* but non-allowlisted
issuer (reverts `6102 IssuerNotAuthorized`), and the real release.

Both denials sign for real on purpose. Tampering with the *signature* makes the
precompile fail during transaction verification, so the transaction is dropped by
the validator and never lands — and a dropped transaction is not evidence of
anything. To produce an on-chain DENY the signature must be genuine and the
failure must happen inside the program.

Needs devnet SOL for fees and rent, and devnet USDC from
<https://faucet.circle.com>. Key material stays in files outside the repository
and is never printed, logged, or committed.

## The cost, stated plainly

On-chain verification adds latency and compute, and duplicates the canonicalizer
in a second language — the opposite of SPT-Txn's offline, sub-millisecond thesis.
That is why the off-chain gate ships as the default and the escrow is the
settlement path you reach for when the payer is not trusted. Both are real; they
are not alternatives to each other.
