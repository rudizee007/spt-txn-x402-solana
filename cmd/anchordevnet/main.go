//go:build devnet

// Command anchordevnet loads the signed receipt log produced by the enforcement
// path, takes its RFC 6962 Merkle root, and anchors that root on Solana devnet
// via the SPL Memo program — a periodic write, never in the decision hot path.
// It then shows that a specific decision can be proven to belong to the anchored
// batch via an inclusion proof.
//
// The log is not synthesized here. It is read from disk, and it is verified
// before anything is signed: every receipt's signature is re-checked, the hash
// chain is re-walked, and the Merkle root is recomputed from the receipts rather
// than trusted from the file. A log that has been edited, reordered or truncated
// does not get anchored — the command exits instead.
//
//	go run ./cmd/x402demo                        # writes receipts.json
//	go run -tags devnet ./cmd/anchordevnet       # anchors that file's root
//
// The root printed by the demo and the root anchored here are the same value.
//
// Same key/network discipline as paydevnet: the signing key stays in your keypair
// file, and this path is behind the `devnet` build tag (excluded from the default
// `go test ./...`).
//
// Requires a little devnet SOL for the fee (no USDC needed).
package main

import (
	"context"
	"encoding/hex"
	"errors"
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

func main() {
	keypairPath := flag.String("keypair", defaultKeypair(), "path to a Solana CLI keypair json (devnet)")
	logPath := flag.String("receipts", "receipts.json", "signed receipt log to anchor (written by cmd/x402demo)")
	proofSeq := flag.Int("proof", 1, "receipt index to demonstrate an inclusion proof for")
	flag.Parse()
	ctx := context.Background()

	// 1. Load the evidence and verify it before anything else happens. LoadLog
	//    re-checks every signature, re-walks the hash chain, and recomputes the
	//    root from the receipts themselves. Anything short of sound fails closed.
	rlog, err := receipt.LoadLog(*logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("no receipt log at %s — run `go run ./cmd/x402demo` first, it writes one", *logPath)
		}
		log.Fatalf("load receipt log %s: %v", *logPath, err)
	}
	if rlog.Len() == 0 {
		log.Fatalf("receipt log %s is empty — nothing to anchor", *logPath)
	}
	if *proofSeq < 0 || *proofSeq >= rlog.Len() {
		log.Fatalf("-proof %d out of range: the log holds %d receipts (0..%d)", *proofSeq, rlog.Len(), rlog.Len()-1)
	}

	root := rlog.Root()
	rootHex := hex.EncodeToString(root[:])
	fmt.Printf("receipt log:  %s\n", *logPath)
	fmt.Printf("  %d receipts, verified (signatures, hash chain, merkle root)\n", rlog.Len())
	fmt.Printf("  merkle root: %s\n\n", rootHex)

	// 2. Anchor the root.
	payer, err := solana.PrivateKeyFromSolanaKeygenFile(*keypairPath)
	if err != nil {
		log.Fatalf("load keypair: %v", err)
	}
	payerPub := payer.PublicKey()

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

	// 3. Demonstrate that a specific decision is provably in the anchored batch.
	proof, err := rlog.Proof(*proofSeq)
	if err != nil {
		log.Fatalf("proof: %v", err)
	}
	leaf, ok := rlog.At(*proofSeq)
	if !ok {
		log.Fatalf("receipt %d not found", *proofSeq)
	}
	verified := receipt.VerifyInclusion(root, leaf.CanonicalBytes(), *proofSeq, rlog.Len(), proof)

	fmt.Printf("anchored %d receipts on devnet\n", rlog.Len())
	fmt.Printf("  merkle root: %s\n", rootHex)
	fmt.Printf("  memo tx:     https://explorer.solana.com/tx/%s?cluster=devnet\n", sig)
	fmt.Printf("  receipt #%d inclusion proof: %d hashes, verifies=%v\n", *proofSeq, len(proof), verified)
}
