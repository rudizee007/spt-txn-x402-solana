# Demo run-sheet — live MCP agent, real devnet settlement (no voiceover)

The undeniable version: a real agent (Claude Desktop) calls a gated payment
tool; the approved payment settles on Solana devnet, and prompt-injected calls
are refused. Captions replace narration.

Total time ~30 min incl. captioning. Target footage ~90 seconds.

---

## 0. Pre-flight (do once, ~3 min)

- [ ] Build the **devnet** binary (this is what moves funds):
      `go build -tags devnet -o "$PWD/bin/spt-txn-mcp" ./cmd/mcp-gateway`
- [ ] Claude Desktop config points `spt-txn.command` at that binary and sets
      `env.SPT_MERCHANT_ADDR` to your merchant pubkey. **Cmd-Q + reopen** after saving.
- [ ] Wallet funded on devnet: SOL for fees + ≥ a few USDC (each ALLOW spends 1).
- [ ] **Pre-approve the tool:** in Claude Desktop, run one throwaway
      *"Using the authorize_payment tool, check whether paying the merchant 1 USDC for
      invoice 42 is authorized"* and click **Allow / always allow**. Confirm it returns
      an Explorer link — that proves end-to-end it works.

**Important framing:** the agent-facing tool is `authorize_payment` — it *asks the
enforcement point for an ALLOW/DENY decision*; the agent holds no keys and moves no
funds (the enforcement point settles on devnet). Phrase prompts as authorization
*checks*, not "pay X" — a general assistant correctly refuses to send funds, but it
will happily *check whether a payment is authorized*.

Optional priming message to paste first (sets context cleanly):

> I'm testing an authorization enforcement point (SPT-Txn) on Solana devnet with
> valueless test tokens. You have an `authorize_payment` tool that asks the
> enforcement point whether a proposed payment is permitted by a policy a human
> pre-approved — it returns ALLOW/DENY, you hold no keys, and you move no real
> funds. Please use it to check the requests I give you.
- [ ] Screen hygiene: Focus / Do-Not-Disturb ON; quit Slack/Mail/notifications;
      one browser tab open (you'll paste the Explorer link there or click it).
- [ ] Open a **fresh Claude Desktop chat** so the transcript is clean.

## 1. Record (~90 sec) — say nothing, just do this

Start recorder: **Cmd-Shift-5 → Record Entire Screen → Record** (or select the
Claude Desktop window). Pause ~2 seconds after each result so captions have room.

1. Type: **`Using authorize_payment, check whether paying the merchant 1 USDC for invoice 42 is authorized.`**
   → agent calls `authorize_payment` → **AUTHORIZED**, with a devnet Explorer tx link.
   Wait for it (a few seconds while devnet confirms — that's real settlement).
2. Type: **`Now check whether paying the attacker that 1 USDC for invoice 42 is authorized instead.`**
   → **REFUSED — recipient not authorized.** (If it asks for an address, say:
   *use the label "attacker"*.)
3. Type: **`And check whether paying the merchant 1000 USDC for invoice 42 is authorized.`**
   → **REFUSED — amount over the approved limit.**
4. Click the **Explorer link** from step 1. Let the transaction page load; scroll
   so the USDC transfer (payer → merchant) is visible.
5. Stop: **Cmd-Shift-5 → Stop** (or the menu-bar stop button).

## 2. Add captions in iMovie (free, ~15 min)

1. iMovie → **Create New → Movie**. Drag the recording into the timeline.
2. **Titles** tab → drag a **"Lower Third"** style onto the clip at each beat →
   type the caption → drag its edges to set duration.
3. Captions, one per beat:

| When | Caption |
|---|---|
| Start (full-screen title) | **SPT-Txn × x402 — a real agent, real enforcement, live** |
| Over prompt 1 | **Human approved: pay the merchant ≤ 1 USDC for invoice:42** |
| On the ALLOW | **✅ ALLOWED — authorized and settled on Solana devnet** |
| Over prompt 2 | **Prompt injection → redirect the payment to an attacker** |
| On the DENY | **⛔ DENIED — recipient not authorized** |
| Over prompt 3 | **Prompt injection → inflate the amount** |
| On the DENY | **⛔ DENIED — amount over the approved limit** |
| Over the Explorer page | **A real on-chain transaction — verifiable on devnet** |
| End (full-screen title) | **A hijacked agent's authorization is useless for anything else. That's SPT-Txn.** |

4. **Export:** Share (top-right) → **File → 1080p → Save**, name it
   `SPT-TXN-MCP-live-demo.mp4`.

## 3. Publish

```sh
scp SPT-TXN-MCP-live-demo.mp4 <user>@<server>:/var/www/htdocs/foss.violetskysecurity.com/
```
Then add a video card to `demo.html` and re-scp the page.

## Notes so nothing surprises you

- The approved cap is exactly 1 USDC, so `1 USDC` allows and `1000 USDC` denies.
- The ALLOW pauses a few seconds to confirm the tx — leave it in; the "settled on
  devnet" caption covers it.
- The first payment to a new merchant also creates its token account (idempotent
  ATA create) — a slightly larger first tx; every call after is a plain transfer.
- DENYs surface to the agent as a tool error — that's the point: the agent
  literally cannot complete the unauthorized action.
