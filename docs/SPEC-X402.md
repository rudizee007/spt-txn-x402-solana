# SPEC — x402 Payment-Gate Wiring (M1)

Status: **draft, spec-first.** The intent-digest ↔ x402 binding (§4) is
trust-boundary: a canonicalization mismatch here is a full authorization bypass
(threat model risk #1). No gate/settle code merges until §4 is agreed and the
differential test in §7 is written.

Scope: **Option 1, off-chain enforcement.** The gate decides ALLOW/DENY *before*
any payment is signed. There is **no on-chain program** in M1 — on-chain escrow
enforcement is a separate near-path track (Option 2). M1 is the honest,
shippable hackathon core: real x402 protocol round-trip, SPT-Txn gating the payer.

Composed **only** from published primitives: the x402 protocol, the open SPT-Txn
reference engine, and standard Solana. No private-repo material (§8).

---

## 1. What x402 gives us, and the gap

x402 is an HTTP 402 flow: a resource server answers an unpaid request with `402`
plus one or more **PaymentRequirements**; the client picks one, builds a
**PaymentPayload**, resends with an `X-PAYMENT` header; a **facilitator** verifies
and settles on-chain; the server releases the resource.

x402 answers *"did the money move?"* It says nothing about *"was this agent
**allowed** to move it — on whose behalf, within what limit, under what policy?"*
When the agent is compromised or prompt-injected, x402 will faithfully pay an
attacker. **SPT-Txn is the missing ALLOW/DENY, evaluated before signing.**

## 2. Where the gate sits (data flow)

```
 Agent/client                      SPT-Txn gate (this repo)         x402 resource server
 ─────────────                     ────────────────────────         ────────────────────
 1. GET /resource ───────────────────────────────────────────────▶ 402 + PaymentRequirements[]
 2. select one requirement ◀──────────────────────────────────────┘
 3. present (requirement, SPT-Txn token) ─▶ 4. offline verify + binding check
                                              ├─ DENY  → stop, emit evidence (no payment)
                                              └─ ALLOW → 5. settle: build+sign Solana payment
 6. resend GET /resource + X-PAYMENT ─────────────────────────────▶ facilitator verifies+settles
 7. 200 + resource ◀──────────────────────────────────────────────┘ + signed receipt emitted
```

The gate is a **payer-side policy enforcement point**. It never holds the
resource server's authority and never forwards upstream credentials (confused-
deputy defense, §6). Its only outputs are a decision and a signed evidence record.

## 3. The SPT-Txn token vs the x402 payment

The SPT-Txn token authorizes an **action class**: "pay up to N of asset A to
payee P for resource R, on behalf of human H, until T." The x402
PaymentRequirements is a **concrete demand**: "pay exactly M of asset A' to P'
for R'." The gate's job is to prove the concrete demand falls *inside* the
authorized class, and that the token was minted *for this exact payment* — the
binding (§4) — not merely for some payment.

Two distinct checks, do not conflate:

- **Binding (§4, cryptographic):** the token's intent digest equals a digest
  recomputed from *these* payment fields. Fails closed on any mismatch. This is
  the trust-boundary surface.
- **Policy (the SPT-Txn verifier):** amount ≤ ceiling, asset ∈ allowed, payee ∈
  allowed, not expired, not revoked, delegation not widened. Reuses the published
  8-step verifier verbatim; M1 adds no policy logic.

## 4. The intent-digest ↔ x402 binding (trust boundary)

**Rule zero — never hash the raw JSON.** JSON has many byte-representations of the
same value (key order, whitespace, unicode escaping, number formatting). Hashing
the serialized PaymentRequirements is the canonicalization-bypass trap. Instead we
extract named fields, normalize each to a fixed-width byte form, and hash them in
a fixed order with domain separation — the same discipline proven in the escrow
binding.

Fields bound (everything that defines *which payment this is*):

| Field | Source (x402) | Normalized form |
|---|---|---|
| scheme | `scheme` | `u8` enum tag (`exact` = 1); unknown scheme → DENY |
| network | `network` (CAIP-2) | `u8` enum tag from a fixed allowlist; unknown network → DENY |
| asset | `asset` (SPL mint, base58) | decoded `Pubkey` → 32B |
| pay-to | `payTo` (base58) | decoded `Pubkey` → 32B |
| amount | `maxAmountRequired` (atomic units, string) | parsed `u128` → 16B little-endian |
| resource | `resource` (URL string) | `SHA-256(utf8_bytes_as_received)` → 32B |
| nonce | SPT-Txn token `jti`/nonce | 32B, single-use per payment |

```
DOMAIN_TAG_X402 = "spt-txn/x402-payment/v1"
binding = SHA-256(
    DOMAIN_TAG_X402 ‖ 0x00 ‖ version:u8 ‖
    scheme_tag:u8 ‖           // enum: exact = 1; unknown → DENY
    network_tag:u8 ‖          // enum from allowlist: e.g. solana-mainnet=1, solana-devnet=2; unknown → DENY
    asset:Pubkey(32) ‖
    pay_to:Pubkey(32) ‖
    amount_atomic:u128_le ‖   // 16 bytes; chain-portable (EVM 18-decimal safe)
    resource_hash:32 ‖        // SHA-256 of the resource URL, byte-exact
    nonce:32
)
```

**Network and scheme are allowlisted enums, not hashed strings.** The set of
supported (scheme, network) pairs is small and known, so each maps to a `u8` tag
via a fixed allowlist. This removes an entire confusable-string class (case,
CAIP-2 format variants) and forces both fields onto known-good values — an
unlisted scheme or network is a hard DENY, never a silent hash mismatch.

**The anti-canonicalization invariant.** `resource` is the one variable-length
field still hashed. The gate and the issuer (and, in Option 2, any on-chain
verifier) MUST compute `resource_hash` over the *identical received bytes* — the
exact UTF-8 URL as it appeared in the PaymentRequirements — with **no**
re-serialization, case-folding, URL normalization, or trailing-slash fixing. One
canonicalizer, one code path, shared by issuer and gate, fuzzed against itself
(§7). If the strings differ by one byte, the binding differs and the payment is
denied. That is the intended behavior.

**Amount semantics.** `amount_atomic` is a `u128` (16B LE) — Solana SPL/USDC fits
comfortably, and the width is chain-portable so the binding format survives an
EVM 18-decimal token without a format break. It is bound to the *exact* value the
gate authorizes for this payment; the policy check separately asserts
`amount_atomic ≤ token.ceiling`. Exact binding + policy ceiling together mean an
attacker can neither inflate the amount (binding breaks) nor exceed the authorized
ceiling (policy denies).

**Deliberately NOT bound: `extra.feePayer`, `maxTimeoutSeconds`, `description`,
`mimeType`.** On the Solana `exact` scheme, `extra.feePayer` is the *facilitator's*
gas-sponsorship account — it co-signs and submits the transaction but is not the
recipient and does not change the payer→payTo transfer of `amount` of `asset`. It
is also filled in *late by the facilitator*, so binding it at issuance would break
the flow and make tokens brittle to facilitator rotation. It is therefore unbound
**by design**, and the compensating control is the hard pre-send assertion (§6,
threat 4): before signing, the gate verifies the transaction's `TransferChecked`
instruction has authority = the authorized payer (so the source token account is
necessarily payer-controlled), destination = the bound `payTo`, mint = the bound
`asset`, and amount = the bound `amount_atomic`. That pins the
actual value-moving instruction regardless of who sponsors gas. **Any future
`extra` field that can redirect value or change payment semantics must be
re-evaluated for binding** — `feePayer` is safe to omit; a hypothetical
`extra.recipient` would not be.

**Single use.** `nonce` is the token's `jti`. The gate records spent nonces in an
append-only spend-log; a replayed token with a seen nonce → `DENY_VIOLATION`. This
is an **off-chain, single-instance** control for M1 (see §5 for its exact limits
and required handling); Option 2 enforces single-use *structurally* on-chain.

## 5. Fail-closed and evidence classes

Every path that is not a clean ALLOW resolves to DENY **with a signed evidence
record**, and the record distinguishes:

- **`DENY_VIOLATION`** — the request was well-formed but not permitted: binding
  mismatch, amount over ceiling, disallowed asset/payee, expired, revoked, widened
  delegation, replayed nonce, unknown scheme.
- **`DENY_UNAVAILABLE`** — enforcement could not complete: Trust Registry snapshot
  unreadable, facilitator unreachable at verify time, malformed 402. Operators must
  be able to tell an attack (`VIOLATION`) from an outage (`UNAVAILABLE`).

No ALLOW is ever emitted on a timeout, parse error, or unreachable dependency.
A DENY never triggers a payment.

**Nonce spend-log — scope and required handling (M1).** Replay defense is the one
inherently *stateful* part of the gate, so its limits are stated, not hidden:

- **Persist-before-ALLOW.** The nonce MUST be durably written (atomic append /
  fsync) *before* the ALLOW is emitted. Emitting ALLOW first opens a replay window
  across a crash or restart.
- **Fail-closed.** If the spend-log is unreadable or unwritable at decision time →
  `DENY_UNAVAILABLE`, never ALLOW.
- **Single-instance only.** The log is gate-local. Running N gate replicas with
  independent logs permits one replay *per replica*. M1 ships single-instance and
  labels it as such; horizontal scale would require a shared store, reintroducing
  the stateful coordination the offline model avoids — which is precisely why
  Option 2 moves single-use on-chain and structural. Naming this boundary is a
  due-diligence asset, not an admission.

## 6. Threats specific to M1

1. **Canonicalization mismatch (§4)** — the whole reason this is spec-first. One
   canonicalizer, shared, differential-tested and fuzzed.
2. **Replay** — a captured token resubmitted for a second payment. Nonce spend-log;
   Option 2 for structural on-chain single-use.
3. **Confused deputy at the gate** — the gate must act with the *token's* authority
   only. It never forwards the caller's upstream credentials to the resource server,
   and never pays from its own standing authority. (This is exactly the MCP token-
   passthrough gap; do not reproduce it.)
4. **Amount/asset substitution & fee-payer abuse (hard gate step).** A facilitator
   or MITM swaps `payTo`/`asset`/`amount`, or supplies a hostile `extra.feePayer`,
   after the decision. Mitigation is a **blocking pre-send assertion**, not a note:
   immediately before signing, the gate decodes the constructed transaction and
   asserts its `TransferChecked` instruction has authority = authorized payer,
   destination = bound `payTo`, mint = bound `asset`, and amount = bound
   `amount_atomic`, and that there is exactly one such transfer in the
   transaction. Any mismatch → abort, no signature, `DENY_VIOLATION`. This pins
   the value-moving instruction independent of the (unbound) fee payer, and is what
   makes omitting `feePayer` from the binding safe.
5. **Downgrade** — server offers a weak/foreign `network` or `scheme`. The scheme
   tag and network hash are inside the binding; an unlisted scheme/network → DENY.

## 7. Test plan (blocking)

- **Differential canonicalizer test:** compute `binding` in the Go gate and in the
  Go issuer over the same PaymentRequirements fixtures; assert byte-identical. Add
  a Python re-implementation as an independent third computation (KAT), matching the
  escrow pattern.
- **Known-answer vectors:** freeze ≥3 `(requirement, nonce) → binding` hex vectors.
- **Field-flip suite:** flipping any bound field (scheme, network, asset, payTo,
  amount, resource, nonce) changes the binding; assert `≠` for each.
- **Canonicalization fuzz (resource):** random `resource` URLs with confusable
  variants (trailing slash, case, percent-encoding, unicode NFC/NFD) MUST produce
  distinct bindings — proving we are *not* silently normalizing.
- **Allowlist enum tests:** unknown or foreign `scheme`/`network` → DENY (never a
  tag collision, never a silent pass). Assert the `TransferChecked` pre-send check
  (§6.4) aborts on any payTo/asset/amount/source mismatch.
- **Fail-closed matrix:** over-ceiling, expired, revoked, widened delegation,
  malformed 402, facilitator down, replayed nonce → each asserts the correct
  DENY class and that no payment was sent.
- `govulncheck` clean; deps pinned.

## 8. Non-goals (M1) & disclosure guardrail

- No on-chain program (Option 2).
- No new policy semantics — reuse the published verifier.
- Everything here derives from x402 + the published SPT-Txn layer + standard Solana.
  No unpublished IP, no reconstruction of private-repo material, no commit message
  or comment referencing it. If a task appears to need private material, stop and
  confirm the repo (§0).

## 9. Build order (code phase, after this spec is accepted)

1. `gate/` — parse a 402, extract PaymentRequirements, compute `binding`, run the
   verifier, emit a decision + signed evidence. Pure function, no network in the
   decision path.
2. `settle/` — on ALLOW, build a Solana payment paying the *bound* payTo/asset/
   amount, sign (key from env only), attach as `X-PAYMENT`, resend.
3. `mock/` — a minimal x402 resource server + facilitator for local end-to-end and
   for the demo, so a judge reproduces it in one command with no external accounts.
4. Wire the differential + fuzz tests (§7) first-class into CI.
