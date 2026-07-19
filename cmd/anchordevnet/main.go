//go:build devnet

// Command anchordevnet builds a small SPT-Txn receipt log, takes its RFC 6962
// Merkle root, and anchors that root on Solana devnet via the SPL Memo program —
// a periodic write, never in the decision hot path. It then shows that a specific
// decision can be proven to belong to the anchored batch via an inclusion proof.
//
// Same key/network discipline as paydevnet: the signing key stays in your keypair
// file, and this path is behind the `devnet` build tag (excluded from the default
// `go test ./...`).
//
//	go run -tags devnet ./cmd/anchordevnet
//
// Requires a little devnet SOL for the fee (no USDC needed).
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"

	"github.com/rudizee007/spt-txn-x402-solana/receipt"
)

// SPL Memo program (mainnet == devnet).
var memoProgramID = solana.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")

// rawInstruction is a minimal solana.Instruction — enough to emit a memo without
// pulling in a memo-program helper.
type rawInstruction struct {
	prog  solana.PublicKey
	metas []*solana.AccountMeta
	data  []byte
}

func (r rawInstruction) ProgramID() solana.PublicKey     { return r.prog }
func (r rawInstruction) Accounts() []*solana.AccountMeta { return r.metas }
func (r rawInstruction) Data() ([]byte, error)           { return r.data, nil }

func defaultKeypair() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "solana", "id.json")
}

func fill(b byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = b
	}
	return a
}

func main() {
	keypairPath := flag.String("keypair", defaultKeypair(), "path to a Solana CLI keypair json (devnet)")
	flag.Parse()
	ctx := context.Background()

	payer, err := solana.PrivateKeyFromSolanaKeygenFile(*keypairPath)
	if err != nil {
		log.Fatalf("load keypair: %v", err)
	}
	payerPub := payer.PublicKey()

	// Build a small receipt log. The receipt-signing key is generated here and is
	// distinct from the payer/issuer key (separate blast radius). The Merkle root
	// depends only on the receipt contents, so it is reproducible: with these
	// fixed decisions it equals the unit-test KAT root.
	rpub, rpriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Fatalf("receipt keygen: %v", err)
	}
	rlog := receipt.NewLog(rpub)
	appendOrDie := func(_ receipt.Entry, e error) {
		if e != nil {
			log.Fatalf("append receipt: %v", e)
		}
	}
	appendOrDie(rlog.Append(rpriv, receipt.Allow, fill(0x11), 1_700_000_000))
	appendOrDie(rlog.Append(rpriv, receipt.DenyViolation, fill(0x22), 1_700_000_060))
	appendOrDie(rlog.Append(rpriv, receipt.DenyUnavailable, fill(0x33), 1_700_000_120))
	if err := rlog.Verify(); err != nil {
		log.Fatalf("receipt log failed self-verify: %v", err)
	}
	root := rlog.Root()
	rootHex := hex.EncodeToString(root[:])

	memoIx := rawInstruction{
		prog:  memoProgramID,
		metas: []*solana.AccountMeta{{PublicKey: payerPub, IsSigner: true, IsWritable: false}},
		data:  []byte("spt-txn/receipt-root/v1:" + rootHex),
	}

	rpcClient := rpc.New(rpc.DevNet_RPC)
	recent, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		log.Fatalf("get blockhash: %v", err)
	}
	tx, err := solana.NewTransaction([]solana.Instruction{memoIx}, recent.Value.Blockhash, solana.TransactionPayer(payerPub))
	if err != nil {
		log.Fatalf("build tx: %v", err)
	}
	if _, err := tx.Sign(func(k solana.PublicKey) *solana.PrivateKey {
		if k.Equals(payerPub) {
			return &payer
		}
		return nil
	}); err != nil {
		log.Fatalf("sign: %v", err)
	}

	wsClient, err := ws.Connect(ctx, rpc.DevNet_WS)
	if err != nil {
		log.Fatalf("ws connect: %v", err)
	}
	defer wsClient.Close()

	sig, err := confirm.SendAndConfirmTransaction(ctx, rpcClient, wsClient, tx)
	if err != nil {
		log.Fatalf("send: %v", err)
	}

	// Demonstrate that a specific decision is provably in the anchored batch.
	proof, err := rlog.Proof(1)
	if err != nil {
		log.Fatalf("proof: %v", err)
	}
	ok := receipt.VerifyInclusion(root, mustCanonical(rlog, 1), 1, rlog.Len(), proof)

	fmt.Printf("anchored %d receipts on devnet\n", rlog.Len())
	fmt.Printf("  merkle root: %s\n", rootHex)
	fmt.Printf("  memo tx:     https://explorer.solana.com/tx/%s?cluster=devnet\n", sig)
	fmt.Printf("  receipt #1 inclusion proof: %d hashes, verifies=%v\n", len(proof), ok)
}

// mustCanonical returns the canonical bytes of receipt seq for the inclusion
// check (kept tiny to avoid exporting log internals).
func mustCanonical(l *receipt.Log, seq int) []byte {
	r, ok := l.At(seq)
	if !ok {
		log.Fatalf("receipt %d not found", seq)
	}
	return r.CanonicalBytes()
}
