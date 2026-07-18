# Build Plan — SPT-Txn × x402 on Solana

Target: **Colosseum fall hackathon (Sep 28 – Nov 2, 2026)**; also submission-ready
for **Eternal** if it reopens. Judges weigh a legible GitHub repo and the ability
to *prioritize, iterate, and ship* in a 4-week sprint. Plan accordingly:
under-promise on-chain scope, over-deliver a clean working demo.

Best-fit track: **Best Trustless Agent** (identity, delegation, validation) — with
**Best x402 Dev Tool** as the fallback framing.

---

## What already exists (reused, not rebuilt)

From the open SPT-Txn reference implementation (`github.com/rudizee007/spt-txn-poc`):

- **Offline 8-step verifier** + attenuating `CAT → CT → SPT-Txn` delegation.
- **`internal/ledger/solana`** — Solana context-hash binding (tested).
- **`cmd/x402gate`** — the x402 payer-gate: ALLOW/DENY before signing (engine is
  chain-agnostic; currently demoed on XRPL).
- **`clients/sol-pay`** — real Solana submitter: SOL transfer + human anchor
  on-chain via SPL Memo, **devnet-proven**, mainnet gated, key from env only.

That means Option 1 is roughly **80% done**. The hackathon delta is genuine x402
protocol wiring, USDC/SPL support, and clean packaging.

---

## Milestones

### M0 — Repo & proof-of-life (this sprint, week 1)
- [x] Separate public repo scaffolded (`spt-txn-x402-solana`).
- [x] Architecture diagram, README, monetization one-pager.
- [ ] Import the SPT-Txn engine as a Go module dependency (no code copy).
- [ ] `gate/` targets Solana: port `x402gate` from the XRPL demo to the Solana
      `ledger` adapter; ALLOW/DENY over a SOL payment on devnet end-to-end.
- [ ] One command that reproduces the devnet proof (`make demo-devnet`).

### M1 — Real x402 protocol wiring (weeks 1–2)  ← the core deliverable
- [ ] Stand up (or point at) an x402 resource server that returns a real `402`
      with payment requirements.
- [ ] Client flow: receive `402` → gate decides → on ALLOW, `settle/` pays and
      retries with proof → resource released. On DENY, no payment, evidence
      recorded.
- [ ] Bind the SPT-Txn intent digest to the x402 payment requirement fields
      (amount, pay-to, resource) — differential-tested against the verifier.
- [ ] Facilitator round-trip on devnet.

### M2 — USDC / SPL payments (week 2)
- [ ] Extend `settle/` from SOL to **USDC (SPL)** — the currency agents actually
      transact in. Amounts in base units; ceiling/price semantics unchanged.
- [ ] (Stretch) Token-2022 mint support.

### M3 — Evidence & transparency (week 3)
- [ ] Emit the signed receipt per decision; write receipts to an append-only log.
- [ ] Anchor the log's Merkle root on Solana (root only — not verification).
      Gives judges an on-chain artifact and the "tamper-evident evidence" story.

### M4 — Demo & submission (week 4)
- [ ] 3-minute demo video (see `demo/`): show a DENY (over-scope / injected
      payment refused) and an ALLOW (in-scope pays, anchor visible on explorer).
- [ ] README quickstart reproducible by a judge in < 5 minutes on devnet.
- [ ] (Stretch) one **mainnet** transaction, gated and clearly labeled.

### Option 2 — on-chain enforcement (near path, after M1–M2 land)
- [ ] Replace the `spt_anchor/` counter scaffold with an Anchor program that:
      holds payment in escrow; verifies the SPT-Txn signature via the **Ed25519
      precompile**; reconstructs and checks the intent digest on-chain; releases
      only on a valid, in-scope proof; fails closed.
- [ ] Differential-test the on-chain canonicalization against the off-chain
      verifier — **this is the highest-risk step** (canonicalization mismatch =
      bypass). Do not ship without it.

---

## Devnet → mainnet readiness checklist

- [ ] Devnet demo reproducible from a clean clone with documented env vars.
- [ ] All key material env-only; nothing sensitive in the repo (`.gitignore`
      verified; `gitleaks` clean).
- [ ] Amounts handled in integer base units (lamports / USDC micro-units) — no
      float rounding on the money path.
- [ ] Mainnet path requires explicit confirmation; devnet is the default.
- [ ] Fail-closed paths tested: over-scope, expired, revoked, widened delegation,
      malformed token, facilitator unreachable → all DENY with correct evidence
      class.
- [ ] `govulncheck` clean; dependencies pinned; commits signed.
- [ ] Mainnet only after a fresh adversarial "find the bypass" review of the x402
      binding and (if built) the on-chain verifier.

---

## Disclosure guardrail (§0)

Everything in this repo is public the moment it is pushed. It must be composed
**only** from the already-published SPT-Txn layer + x402 + standard Solana
primitives. No unpublished IP, no reconstruction of private-repo material, no
commit message or comment that references it. If a task appears to need private
material, stop and confirm which repo we're in.
