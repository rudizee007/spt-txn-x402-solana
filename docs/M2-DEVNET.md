# M2 — Real USDC settlement on Solana devnet

**What it proves:** the §6.4 settle guard runs against a *real* Solana USDC
`TransferChecked` before signing. Only a transfer that pays the **bound**
recipient / asset / amount under the **payer's authority** is signed and sent —
otherwise the tool refuses to sign and no funds move.

## Prerequisites

- Go, plus the Solana SDK: `go get github.com/gagliardetto/solana-go`
- A devnet keypair (e.g. `~/.config/solana/id.json` from `solana-keygen new`),
  with a little devnet SOL for fees: `solana airdrop 1 --url devnet`
- Devnet USDC in that wallet — Circle faucet: https://faucet.circle.com
  (select **Solana devnet**; 20 USDC per 2h). Mint:
  `4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU`, 6 decimals.

## Run

```sh
# 0.10 USDC to yourself (simplest reproducible check)
go run -tags devnet ./cmd/paydevnet -amount 100000

# or to a specific merchant wallet
go run -tags devnet ./cmd/paydevnet -to <merchant_wallet> -amount 100000
```

The `devnet` build tag keeps this key/network path out of the default
`go test ./...`, so the core packages stay buildable and green without the SDK.

## What happens

1. Loads your keypair (the key stays in the file — never an env var, never
   printed, never committed).
2. Derives your USDC associated token account (source) and the merchant's (dest).
3. Builds the real `TransferChecked` (source → dest, micro-USDC, 6 decimals).
4. **`settle.AssertMatches` decodes the *actual* instruction and refuses to sign**
   unless it pays the bound recipient / asset / amount under your authority.
5. Signs, sends to devnet, prints the explorer link.

## M2b — On-chain escrow enforcement (`cmd/escrowdevnet`)

Where `paydevnet` proves the *off-chain* guard refuses to sign a bad transfer,
`escrowdevnet` proves the *on-chain* program refuses to release one. Same devnet,
real USDC, no off-chain service in the custody path.

### One-time setup — three keys, not two

The program enforces separation of duties, so the demo needs three distinct
keypairs. Generate the two it manages for you (mode 0600 under
`~/.config/spt-txn/`):

```sh
go run -tags devnet ./cmd/escrowdevnet -gen-issuer   # the SPT-Txn signing key
go run -tags devnet ./cmd/escrowdevnet -gen-admin    # the issuer-allowlist admin
```

The third is your existing wallet (`SPT_KEYPAIR` or `~/.config/solana/id.json`),
which is the payer **and** the program's upgrade authority. `init_config` refuses
the transaction if the admin key equals the upgrade authority — that is the point
of generating a separate one rather than reusing the wallet.

Fund the admin key with a little devnet SOL for fees; it never holds USDC and
never touches a vault.

### Run

```sh
# everything in one take: setup (init_config + add_issuer) -> deposit ->
# deny-binding -> deny-issuer -> deny-unpinned -> release
go run -tags devnet ./cmd/escrowdevnet -mode all

# or one at a time
go run -tags devnet ./cmd/escrowdevnet -mode setup
go run -tags devnet ./cmd/escrowdevnet -mode deposit
go run -tags devnet ./cmd/escrowdevnet -mode deny-unpinned    # the one that matters
go run -tags devnet ./cmd/escrowdevnet -mode release
go run -tags devnet ./cmd/escrowdevnet -mode refund           # after 15 min expiry
```

`deposit` writes `escrow-ticket.json` with that escrow's public parameters —
including the pinned issuer — and the later `release` / `refund` modes read it.
Note `-mode release` does **not** load the admin key at all: the admin has no
part in the release path, and requiring its signature there would misrepresent
who can release.

### What `deny-unpinned` proves

It runs the admin-compromise attack **with the admin key in hand**:

1. A payer deposits an escrow pinning issuer **A**.
2. The admin allowlists a rogue issuer **B**. *This succeeds* — the admin really
   does have that power, and the run shows it landing on chain.
3. **B** signs a perfectly valid, correctly-bound, fresh attestation for that
   escrow and calls `release_with_proof`.
4. The program reverts with **6108 `IssuerNotPinned`**. No funds move.

The claim "a compromised admin cannot release customer funds" is therefore an
explorer link, not a paragraph. Hold the failing transaction on screen — the
custom error code is the evidence.

To pin a specific issuer rather than the generated one, pass `-issuer-pub <base58>`;
`init_escrow` carries it as the fourth Borsh argument.

## Notes

- USDC mint + SPL program-id bytes are frozen and unit-tested
  (`settle/spl_test.go`), cross-checked against an independent base58 decode.
- The signing key never leaves the keypair file. Mainnet is intentionally not
  wired here.
- To see the guard refuse a bad transfer, the unit tests in `settle/` cover every
  mismatch (wrong recipient/asset/amount/authority); the same `AssertMatches`
  runs on the real instruction here.
