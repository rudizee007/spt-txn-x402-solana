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

Use the printed `aud` verbatim in the next step. If it is a list, use the entry
the bridge should accept.

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
