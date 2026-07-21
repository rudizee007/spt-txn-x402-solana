# Colosseum Eternal — submission packet

Copy-paste reference for the Eternal portal. Two things are yours to supply
(marked **TODO**): the demo-video URL, and confirming the repos are public.

## One-liner

> x402 moves the money. SPT-Txn proves the agent was allowed to — per transaction, with a  tamper-evident receipt, verifiable offline at the edge or online for instant revocation.


## Short description

SPT-Txn is the per-transaction authorization layer x402 is missing. x402 moves the
money; it never checks whether the agent was *allowed* to. SPT-Txn puts authority
inside a short-lived token bound to one exact payment — one asset, one amount, one
recipient — verified offline with no call home, so a hijacked or prompt-injected
agent holds a token that's useless for any other payment. A pre-sign guard refuses
any transaction that doesn't match; a real USDC transfer settles on Solana devnet;
and every decision emits a signed receipt whose log's Merkle root is anchored
on-chain. Drop-in middleware adds it to any x402 server in one line.

## Links

- **Main repo:** https://github.com/rudizee007/spt-txn-x402-solana  *(TODO: confirm public)*
- **On-chain escrow:** https://github.com/rudizee007/spt-txn-x402-escrow  *(TODO: publish + confirm public)*
- **Reference engine:** https://github.com/rudizee007/spt-txn-poc
- **Demo video:** *(TODO: upload + paste URL)*
- **Devnet settlement tx (authorization-gated USDC, shown in the demo):** https://explorer.solana.com/tx/376oVo5dNc8tVgJiXB6eVpckNhTNchxbrgs19ShZmcmNx1ZxkN6v8Hvw6TjFVRxo2Xzs1w1RDPFT6BdxbsPDU1u2?cluster=devnet
- **Devnet settlement tx (payer → merchant, earlier run):** https://explorer.solana.com/tx/3H4MfiYrsZ66pK23VkCFeKPpN18u2YiJQvWDnqTBNp4Hy541kMKtDWuVV9xnBN9Kp9R8WBiRN6m4uaBrCm76rNkX?cluster=devnet
- **Devnet evidence anchor tx (receipt root via Memo):** https://explorer.solana.com/tx/iFwwZkv134qXcMX3D17JtbtiVQHJcA3RYfVBaM7QgWchGUjqVtaPukLuhQepXJ2ou6QuL3AN8n7EzyjjJAwKuCi?cluster=devnet
- **Escrow program (deployed, devnet):** https://explorer.solana.com/address/C9kTmtYm5V8cFfNvgzJAcVfM2zYN1Pqv245Xe27h4NwZ?cluster=devnet
- **IETF Internet-Draft:** https://datatracker.ietf.org/doc/draft-coetzee-oauth-spt-txn-tokens/
- **Zenodo DOI:** `10.5281/zenodo.19299787`
- **ORCID:** `0009-0009-6557-8843`

## Reproduce in ~5 minutes

```sh
git clone <main repo> && cd spt-txn-x402-solana
go test ./...          # gate + settle + receipt + gateway, incl. differential KATs
go run ./cmd/x402demo  # 402 → gate → guard → settle, over real HTTP, + evidence root
go run ./cmd/gateway   # drop-in PEP middleware + transparency-log endpoints
```

Devnet (real USDC, your keypair): fund at faucet.solana.com + faucet.circle.com,
then `go run -tags devnet ./cmd/paydevnet -amount 100000` (settles), `… -tamper`
(refuses before signing), `go run -tags devnet ./cmd/anchordevnet` (anchors root).

## The ask

Eternal award + accelerator entry, converting to a pre-seed round to fund the
hosted transparency log, the first two jurisdiction policy packs (Travel Rule +
MiCA), and the x402 gateway — the shortest paths to recurring revenue.

## Pre-submission checklist

- [ ] `go test ./...` green; `govulncheck ./...` clean
- [ ] Main repo public + §0-clean (final grep: no hook / private / disclosure refs)
- [ ] Escrow repo: `anchor build` + `cargo test` green; novelty re-scan + line-by-line review done; published public
- [ ] Demo video recorded, uploaded, and linked here
- [ ] Product page filled (from `ETERNAL-PRODUCT.md`); all links above resolve
- [ ] Weekly update videos 1–4 posted
- [ ] Devnet tx links open on the explorer
