# Demo video — storyboard & script (HARD CAP 3:00)

> **Colosseum Eternal requirement, verbatim:** *"Please submit a demo video of
> your product. YouTube, Loom, or Vimeo. **Up to 3 minutes. Should show the live
> product, not a slide deck, not a code walkthrough.**"*
>
> Two consequences:
>
> 1. **3:00 is a cap, not a target.** With beat 1b this script runs 3:09 and 7b
>    takes it to 3:24 — both over. Cut beat 3 (`go test ./...`) and drop the
>    optional custody line in beat 5 to land near 2:50.
> 2. **"Not a code walkthrough" rules out beat 3.** A passing test suite is the
>    closest thing here to reading code on camera, and it is the least persuasive
>    30 seconds in the script. The tests are in the repo; the judges asked to see
>    the product run. Cut it and spend nothing.
>
> Everything else — the HTTP flow, the devnet settlement, the refusal, the
> escrow, the gateway — is the live product doing its job, which is exactly what
> they asked for. For a developer-infrastructure product the terminal *is* the
> product surface; that is not a code walkthrough.

The hook is one real setup showing **two outcomes**: an authorized USDC payment
settles on devnet, and a tampered one is **refused before signing**. Everything on
screen is live and reproducible from the README quickstart — no slideware.

Tone: calm, technical, confident. Let the terminal do the talking. Total spoken
words ≈ 455 (≈150 wpm).

**Beat 1b is an edit-suite insert, not a shot.** It splices existing footage
(`SPT-TXN-MCP-live-demo.mp4`) and adds nothing to the recording session. It earns
its twelve seconds because the track is *Best Trustless Agent* and every other
beat demonstrates the agent problem with a CLI standing in for an agent — 1b is
the only place a real one appears.

---

## Pre-flight checklist (before you hit record)

- Terminal font large (≈18–20pt), dark theme, wide enough that no line wraps.
- `cd spt-txn-x402-solana`; clear scrollback (`clear`).
- Devnet wallet funded: a little SOL (fees, plus ~0.002 for the merchant ATA rent)
  + USDC (faucet.circle.com — rate-limited, check this first, it is the one thing
  that can block the whole shoot).
- **Generate two throwaway merchants**, one to warm up with and one to record
  against:

  ```sh
  solana-keygen new --no-bip39-passphrase --silent -o /tmp/merchA.json
  solana-keygen new --no-bip39-passphrase --silent -o /tmp/merchB.json
  A=$(solana-keygen pubkey /tmp/merchA.json); B=$(solana-keygen pubkey /tmp/merchB.json)
  go run -tags devnet ./cmd/paydevnet -to $A -amount 100000   # off-camera: proves the path
  export MERCHANT=$B                                          # on-camera: fresh, ATA created live
  ```

  Warming up against A proves the merchant-pay path end to end; recording against
  a still-fresh B means the ATA creation happens on camera, which is the point.
- Browser open to a blank Solana Explorer tab (devnet cluster preselected).
- Pre-build so there's no compile lag on camera: `go build ./... >/dev/null`.
- Close notifications / hide anything private (the payer address is fine to show).

---

## Beat sheet

| # | Time | On screen | Say (verbatim) |
|---|------|-----------|----------------|
| 1 | 0:00–0:22 | Title card: **"SPT-Txn × x402 — authorization for agent payments"**, then cut to terminal | "x402 lets an AI agent pay for things over HTTP. It answers *did the money move* — but nothing checks whether the agent was *allowed* to move it. When an agent is prompt-injected or hijacked, x402 will faithfully pay the attacker. That gap is what we close." |
| **1b** | 0:22–0:34 | **No recording needed** — splice ~12s from the existing `SPT-TXN-MCP-live-demo.mp4`: Claude Desktop calls the payment tool, the approved payment settles, the hijacked recipient is refused | "And that's not hypothetical. Here's a real agent — Claude Desktop — calling a payment tool over MCP. The one payment a human approved settles on devnet. The hijacked recipient is refused by the enforcement point." |
| 2 | 0:34–0:52 | Terminal, `README.md` open or the one-line pitch on screen | "SPT-Txn is a per-transaction authorization layer. Authority exists only inside a short-lived token bound to one exact payment — one asset, one amount, one recipient — verified offline, with no call home. A hijacked agent holds a token that's cryptographically useless for any other payment." |
| 3 | 0:52–1:17 | Run `go test ./...` — all packages `ok` | "It's all here and tested. The intent binding is differential-tested against an independent Python implementation, the Merkle log is RFC 6962, and every fail-closed path has a negative test. No custom cryptography." |
| 4 | 1:17–1:47 | Run `go run ./cmd/x402demo` — the four lines print, then the evidence root | "Here's the real HTTP flow. A 402 payment request comes back; the gate decides. In-scope: allowed, resource released. A replayed token: refused. An over-budget payment: denied before signing. And a *tampered* payment — the gate allowed the legitimate request, but the pre-sign guard sees the money is being redirected and refuses to sign. Every decision drops a signed receipt; here's the Merkle root over all of them." |
| 5 | 1:47–2:27 | Run `go run -tags devnet ./cmd/paydevnet -to $MERCHANT -amount 100000`; when it prints the tx link, **cut to browser**, paste link, show the confirmed USDC transfer | "Now on Solana devnet, with real USDC. The guard checks the actual transaction pays exactly the bound recipient and amount, under my authority — then signs and settles. There it is on-chain: a real, authorization-gated USDC transfer." *(optional, ~7s, drop if the take runs long:)* "And note what 'my authority' means — the token binds the **transfer authority**, whichever key is permitted to move the funds. Self-custody or a custodian, it's the same check. We never hold keys." |
| 6 | 2:27–2:50 | Back to terminal, run `go run -tags devnet ./cmd/paydevnet -to $MERCHANT -amount 100000 -tamper` → **`REFUSING TO SIGN`**, exit 1 | "Same wallet, same USDC — but now the built transfer points at a different recipient. The bound payTo hasn't changed, so the guard refuses to sign. Nothing touches the chain. One setup: the authorized payment settled; the redirected one never left the machine." |
| 7 | 2:50–3:02 | Run `go run -tags devnet ./cmd/anchordevnet`; show root + memo tx link (optional quick browser cut to the memo) | "And the evidence is a byproduct: the receipt Merkle root is anchored on-chain via SPL Memo. Any single decision can be proven to belong to that batch — tamper-evident, no PII on the ledger." |
| 7b *(optional)* | 3:02–3:17 | Run `go run ./cmd/gateway` — the enforcement lines, then `/transparency/root` and `"verified":true` | "And it drops in: any x402 server wraps one middleware to get this on every request — served, replay refused, over-budget denied — plus a live transparency log. Fetch the anchored root, prove any single decision belongs to it, without seeing the others." |
| 8 | 3:17–3:24 *(3:02–3:09 without 7b)* | Title/end card: repo + links (IETF draft, Zenodo DOI), plus on-screen text: **Research project · not externally audited · Solana devnet** | "x402 moves the money. SPT-Txn proves the agent was allowed to. Repo and spec in the description." |

---

## Command order

### Off camera — run these, THEN start recording

```sh
export MERCHANT=<merchB pubkey>
go build ./... >/dev/null
clear
```

`export` would put a 44-character base58 string on screen for no reason, and
`go build` exists solely so nothing compiles mid-take. Neither belongs in the
video. Setting `MERCHANT` must happen in the *recording* shell — it does not
survive opening a new terminal.

### On camera — six commands, one per beat, narrated

```
go test ./...
go run ./cmd/x402demo
go run -tags devnet ./cmd/paydevnet -to $MERCHANT -amount 100000
go run -tags devnet ./cmd/paydevnet -to $MERCHANT -amount 100000 -tamper
go run -tags devnet ./cmd/anchordevnet
go run ./cmd/gateway
```

Beat 3, 4, 5, 6, 7, 7b respectively. Beat 1b is a post-production splice, not a
shot; beats 1, 2 and 8 have no command.

**Type these without trailing `# comments`.** Interactive zsh does not treat `#`
as a comment, so a pasted annotation becomes an argument and the command errors
on camera. This has already happened twice in setup.

**`-to` is not optional on camera.** Without it `paydevnet` pays *yourself* —
source ATA and dest ATA identical, payer and merchant the same address. It
settles, but a judge who opens the Explorer link sees one address on both sides,
which quietly destroys the beat. With `-to` naming a merchant who has no token
account yet, the run prepends `CreateAssociatedTokenAccountIdempotent` and the
merchant's ATA is created *in the same transaction* — the Week 1 capability, and
a visibly better Explorer page.

Core cut runs ~3:09 (beats 1–8, including the 1b insert); adding the optional
gateway beat 7b takes it to ~3:24. Drop the optional custody line in beat 5 to
claw back seven seconds if you need to land under 3:00.

If a devnet call is slow to confirm on camera, cut the dead air in edit; do not
re-run mid-shot.

## Lower-thirds (optional captions)

- Beat 1b: `a real agent · Claude Desktop over MCP · hijack refused`
- Beat 3: `differential-tested · RFC 6962 · fail-closed`
- Beat 4: `402 → gate → guard → settle, all enforced`
- Beat 5: `real USDC · Solana devnet`
- Beat 6: `refused before signing — funds never move`
- Beat 7: `evidence = byproduct of enforcement`
- Beat 7b: `drop-in middleware · live transparency log`

## 60-second cut (if a short version is needed)

Beats 1 (trimmed to the gap), **1b**, 6 (the tamper refusal), 8. Prefer 1b over
beat 4 here: at sixty seconds a real agent being refused lands harder than a
four-line CLI trace, and it is the only cut that shows the threat and the defence
in the same shot. Problem, real agent, on-chain refusal, ask.

## Accuracy guardrails (do not overclaim)

- Say **devnet**, never mainnet. Mainnet is intentionally not wired.
- Don't call the receipt anchor a "human anchor" — it's a Merkle root of decision
  receipts. Only hashes go on-chain.
- Everything shown is Option 1 (off-chain gate + settlement guard + receipts). Do
  not mention or hint at any unpublished on-chain-enforcement work.
- The numbers on screen are the truth; let them stand without embellishment.
- **Nothing is externally audited and nothing is in production.** Don't claim or
  imply otherwise. Putting it on the end card is better than being asked.
- Say "reproducible **from the README**" — never a bare "reproducible", which a
  security audience hears as reproducible *builds*, a claim not yet earned.
- On custody: SPT-Txn binds the transfer authority and never holds keys. Do NOT
  say a custodian integration exists — none has been built or tested. The design
  accommodates it; that is the whole claim.
