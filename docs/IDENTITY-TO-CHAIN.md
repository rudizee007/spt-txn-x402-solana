# Identity to chain — binding one issuer across both halves

**What this demonstrates.** The authority established by an enterprise identity
exchange is the *same cryptographic authority* a Solana program enforces. One
Ed25519 key signs the Compliance Attestation Token minted from an OIDC login,
and that same key is the issuer on the escrow's on-chain allowlist, pinned by
the payer at deposit, whose signature releases the funds.

**What it does not demonstrate.** The CAT is not verified on chain. The escrow
verifies an issuer signature over a fixed-width binding via the Ed25519
precompile; it has never seen a token. The binding between the two halves is at
the **key**, not the token. Say it that way — a technical reviewer will ask, and
the honest answer is still a strong one. What connecting the *tokens* would
require is specified in `PRESENTATION-SEAM.md` in the `spt-txn-pep` module.

Devnet only. Nothing here is externally audited or in production.

---

## Prerequisites

- Docker (for Keycloak), Go, and the Solana CLI.
- A devnet wallet with SOL for fees, and devnet USDC from https://faucet.circle.com.
- Three distinct keys, which the program requires and enforces: your wallet
  (payer, and the program's upgrade authority), the allowlist **admin**, and the
  **issuer**. `init_config` refuses if admin equals the upgrade authority.

## 1. Bind one identity across both sides

If you already have an escrow issuer key — you do, from the Week 2 run — derive
the bridge configuration from it rather than minting a new one. Reusing the
existing key matters: escrows pinned to the old issuer can only expire and
refund if you swap it.

```sh
go run ./cmd/issuerkey -from-key ~/.config/spt-txn/issuer-devnet.json
```

Note the **base58** public key (the on-chain allowlist entry) and the **hex**
public key (what the identity bridge will report). Then, off camera:

```sh
go run ./cmd/issuerkey -from-key ~/.config/spt-txn/issuer-devnet.json -print-seed
```

The seed is a secret. It is withheld unless you ask for it precisely so it
cannot land in a screen recording by accident.

## 2. Stand up the identity provider

In the reference-engine repository:

```sh
cd deploy/keycloak && docker compose up -d && docker compose logs -f
```

That imports the `spt` realm: one public client `spt-agent`, one user
`alice` / `alice`. Wait for "started".

## 3. Get a token from the IdP first, and read its audience

Do this before configuring the bridge — `SPT_IDP_AUDIENCE` must match the token's
`aud` claim exactly, and guessing it is the most common way to lose an hour here.

```sh
KC=$(curl -s -X POST http://localhost:8080/realms/spt/protocol/openid-connect/token \
  -d grant_type=password -d client_id=spt-agent \
  -d username=alice -d password=alice \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')

python3 -c 'import sys,base64,json;p=sys.argv[1].split(".")[1];p+="="*(-len(p)%4);c=json.loads(base64.urlsafe_b64decode(p));print("iss:",c.get("iss"));print("aud:",c.get("aud"))' "$KC"
```

**Observed with the shipped demo realm: there is no `aud` claim.** The realm
defines one client and one user and grants alice no default account roles, so
Keycloak mints no audience. The verifier checks `azp` before `aud`, and Keycloak
does set `azp` to the client id, so use:

    SPT_IDP_AUDIENCE=spt-agent

That is a weaker check than a real audience — `azp` says which client asked for
the token, not who it was minted for. An `oidc-audience-mapper` is present on the
client in `realm.json` so a *fresh* import mints `aud: spt-txn-exchange`; an
already-created realm is not re-imported by `--import-realm`, so an existing
container keeps the old behaviour. Recreating the container (`docker compose
down` then `up -d`) picks it up.

If you are demonstrating this, say it plainly: the demo realm authorizes by
client identity, and an audience-restricted token is the production hardening.
The bridge refuses to start with an empty audience either way — there is no
configuration in which the check is silently skipped.

## 4. Run the identity bridge with the bound key

```sh
export SPT_IDP_OIDC_ISSUER=http://localhost:8080/realms/spt
export SPT_IDP_AUDIENCE=<the aud claim in the Keycloak token — see below>
export SPT_IDP_PERMITTED_SCOPE='{"action":"transfer","max_amount":10000,"currency":"USD"}'
export SPT_IDP_CAT_SEED_HEX=<seed from step 1>
go run ./cmd/idp-bridge          # 127.0.0.1:8090
```

**Confirm the binding before going further:**

```sh
curl -s http://127.0.0.1:8090/issuer
```

The `public_key_hex` it returns must equal the hex public key from step 1. That
equality is the whole point of this exercise — it is the moment the identity
authority and the on-chain authority are provably the same key.

## 5. Exchange the token for a CAT

```sh
HOLDER=$(openssl rand -hex 32)

curl -s http://127.0.0.1:8090/token \
  -d grant_type=urn:ietf:params:oauth:grant-type:token-exchange \
  -d subject_token="$KC" \
  -d subject_token_type=urn:ietf:params:oauth:token-type:access_token \
  -d holder_key_hex="$HOLDER"
```

That is an unmodified Keycloak doing its normal job, and a CAT issued from it —
signed by the key the chain trusts.

## 6. Enforce on chain with the same identity

Back in this repository:

```sh
go run -tags devnet ./cmd/escrowdevnet -mode setup   # init_config + add_issuer (that key)
go run -tags devnet ./cmd/escrowdevnet -mode all     # deposit, three denies, release
```

Capture the explorer links for `add_issuer` and for the release.

## 7. The artifact

Three facts, each independently checkable:

1. A Keycloak login produced a CAT signed by key **K** (`/issuer` reports K).
2. K is on the escrow's on-chain allowlist and was pinned by the payer at deposit.
3. A release transaction on devnet succeeded against a signature by K, and
   attempts by any other issuer reverted — `IssuerNotAuthorized` (6102) and
   `IssuerNotPinned` (6108) are on chain, with explorer links.

For the pitch deck this is one slide: the IdP and the CAT issuer on the left,
the Solana explorer showing the allowlist entry and the release on the right,
and the same key rendered in both. Caption it *"the same authority, on both
sides"* — not *"the token is verified on chain"*.

## Where it will fight you

- **`SPT_IDP_AUDIENCE` is the most likely first failure.** Decode the `aud`
  claim of the Keycloak access token and use exactly that; a mismatch is
  rejected at the exchange, which is correct behaviour and an unhelpful error to
  guess at.
- **Faucets.** Devnet USDC is rate-limited (20 per 2 hours). Get it before you
  start, not halfway through.
- **Three distinct keys.** The program enforces admin ≠ upgrade authority. Fund
  the admin with a little SOL; it never holds USDC.
- **Do not swap the issuer key** if any escrow is currently pinned to the old
  one — those can only expire and refund, by design.

## Claims discipline

- Say **devnet**, never mainnet.
- Say **"the same key signs the CAT and satisfies the on-chain issuer check."**
- Do **not** say the CAT, the delegation chain, or the identity token is
  verified on chain. It is not.
- Keycloak here, and PingOne and Auth0 have been exercised through the same
  RFC 8693 flow. That is demonstrated interop, not adoption.

---

## Observed run — 2026-08-18, Solana devnet

Reproduced end to end with Keycloak as the identity provider. Every value below
is checkable by a third party.

**The binding.** `cmd/issuerkey -from-key` reported the escrow issuer key as
base58 `AmjfekEWgezUXiCgF5ZRSp7ndxYnVZAbhGyQ8A9YkBdm`, hex
`912efb218bbd0a182cb977dcab4da01a096bb016def3c35a43b72efec4eaf724`. With that
seed configured, the identity bridge's `GET /issuer` returned the same hex under
issuer name `domain-a.authorg`. Same key, both sides.

**The identity half.** An unmodified Keycloak (`spt` realm, client `spt-agent`,
user `alice`) issued an access token; RFC 8693 token exchange returned a CAT
signed EdDSA by that key, carrying `capability_scope`
`{action: transfer, currency: USD, max_amount: 10000}`, `delegation_depth_max: 3`,
a holder key, and `human_anchor: 0c80c965…` — a hash, not an email address. No
PII crosses into the payment layer.

Keycloak's minimal demo realm mints no `aud`, so the exchange authorized on
`azp` (the client id). See the note in step 3.

**The chain half.** Program `C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ`,
config PDA `6qC1B4DjpT5UaUBgUWdFRd1TQdp2VTEWbFHuZbQFQEqw`, admin
`4XXK6XvpizbjuWY2pFTiC8LfoSscyun1SN7Ya1qznvZN` — a different key from the
upgrade authority, which the program enforces. The issuer above was already on
the allowlist and was pinned by the payer at deposit.

Note the two digests are domain-separated: the off-chain gate binding
(`02b00e72…`) and the on-chain escrow binding (`e162cb06…`) are computed under
distinct tags, so neither can stand in for the other.

| Step | Result | Transaction |
|---|---|---|
| `init_escrow` — 100000 micro-USDC into the vault | moved | [3ErnWavh…qv8RK](https://explorer.solana.com/tx/3ErnWavhurU4K1LsPdaTp9PGhHiMwT3NaoWHNmAMh6BjtLJvD7pv57xJBHiaFEm7oT5gq3MZm9Dd5UrkZofqv8RK?cluster=devnet) |
| Genuine signature, wrong binding | **6105 BindingMismatch** | [2DzZtQxY…yXrSk](https://explorer.solana.com/tx/2DzZtQxYjQSXkkndLqWGy6PnigHi64Fbp7JCz9Q2Pk8FfycE9d2crBgafgj39BCdScqo8avLMrkrazPvahayXrSk?cluster=devnet) |
| Genuine signature, unlisted issuer | **6102 IssuerNotAuthorized** | [3XTdn6LE…761mj](https://explorer.solana.com/tx/3XTdn6LEgMVPkzWyGus8Y9VY4QZWUs9NDmLHBUB8DMWM3SHigwyLBFk1EckyFpY1qC6AKBJvbbBFzrEkPXT761mj?cluster=devnet) |
| Compromised admin allowlists a rogue issuer | **succeeds — the admin really can do this** | [3wqHHMUK…8Erz7](https://explorer.solana.com/tx/3wqHHMUKQ7ircM5cxq5SvBomDSQFmehEX8RfpXrLRkvfajFMNHBfY79Xt4cRCJ4rTaCCUK72zmx1RZgi6JH8Erz7?cluster=devnet) |
| That rogue issuer tries to release | **6108 IssuerNotPinned — no funds moved** | [4nD38G84…YraAf](https://explorer.solana.com/tx/4nD38G84FhuXpZ9fMs2Pzpf1kN49X6WhNLHEMEcUvyB4QWRZbYGQx5Dm6XUK7RDsRdga69nrTXtPNBYHSEUYraAf?cluster=devnet) |
| `remove_issuer` — allowlist restored | done | [3j1eYUTZ…BBXtQ](https://explorer.solana.com/tx/3j1eYUTZNu1YZGCocwe5ux3rRndJtKFLk5r23qho6NLWFfkDSM5GwzDaVxShCJ2zSSAaFAFENQPjYGmQYxwBBXtQ?cluster=devnet) |
| The pinned issuer releases | **funds moved, escrow and vault closed** | [5GFsUkmV…XtTAw](https://explorer.solana.com/tx/5GFsUkmVkjQGc2XKuB8XsZYBQwAVMSCehzWN9D3c6Q5rKoR6zYpSz4kbTXxguQ2iRiqgBTzt6E2d7PDcpPTXtTAw?cluster=devnet) |

The spent marker `AuABkkbzw8V5SjgRGACd2CayrLWMyRFMcGagPWVjhrEJ` now exists
permanently: that binding can never be released again.

**The one that matters.** Rows four and five are a single argument. The admin key
was used, successfully, to authorize an attacker's issuer — and the payment still
could not be released, because release is also gated on the issuer the payer
pinned at deposit. A fully compromised operator cannot move a user's funds. That
is on chain, not in a threat model.
