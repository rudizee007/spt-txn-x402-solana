//go:build devnet

// Command paydevnet performs a REAL USDC TransferChecked on Solana devnet, gated
// by the SPT-Txn settle guard (SPEC-X402 §6.4). It builds the transfer, decodes
// the *actual* instruction, asserts it pays exactly the bound recipient / asset /
// amount under the payer's authority, and ONLY THEN signs and sends. If the guard
// is unhappy, it refuses to sign — no funds move.
//
// This is the one path that touches keys and the network, so it is behind the
// `devnet` build tag and excluded from the default `go test ./...`. The signing
// key stays in your Solana keypair file; it is never read into an env var, never
// printed, and never committed.
//
// Setup:
//
//	go get github.com/gagliardetto/solana-go
//	# fund your devnet wallet with devnet USDC: https://faucet.circle.com
//	go run -tags devnet ./cmd/paydevnet -amount 100000            # pay 0.10 USDC to yourself
//	go run -tags devnet ./cmd/paydevnet -to <merchant> -amount 100000
//	go run -tags devnet ./cmd/paydevnet -amount 100000 -tamper    # adversarial: guard refuses to sign
//
// Note: this file could not be compiled in the authoring environment (no Go /
// no SDK there). Expect to `go get` the SDK first; if any solana-go API name has
// drifted in your version, it is a small fix.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"

	"github.com/rudizee007/spt-txn-x402-solana/settle"
)

func defaultKeypair() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "solana", "id.json")
}

// bytesRepeat returns a 32-byte slice filled with b — a stand-in "attacker"
// recipient for the -tamper demo (its on-chain existence is irrelevant; the guard
// refuses before anything is sent).
func bytesRepeat(b byte) []byte {
	s := make([]byte, 32)
	for i := range s {
		s[i] = b
	}
	return s
}

func main() {
	keypairPath := flag.String("keypair", defaultKeypair(), "path to a Solana CLI keypair json (devnet)")
	toStr := flag.String("to", "", "merchant wallet pubkey (base58); default: pay yourself")
	amount := flag.Uint64("amount", 100_000, "amount in micro-USDC (100000 = 0.10 USDC)")
	tamper := flag.Bool("tamper", false, "adversarial demo: build the transfer to a DIFFERENT recipient than the bound payTo; the guard must refuse to sign")
	flag.Parse()

	ctx := context.Background()

	// 1. Load the payer key from the keypair file (stays in the file).
	payer, err := solana.PrivateKeyFromSolanaKeygenFile(*keypairPath)
	if err != nil {
		log.Fatalf("load keypair %q: %v", *keypairPath, err)
	}
	payerPub := payer.PublicKey()

	// 2. USDC mint (bytes verified in settle) and the recipient.
	usdcMint := solana.PublicKeyFromBytes(settle.USDCDevnetMint[:])
	merchant := payerPub
	if *toStr != "" {
		merchant, err = solana.PublicKeyFromBase58(*toStr)
		if err != nil {
			log.Fatalf("bad -to pubkey: %v", err)
		}
	}

	// 3. Derive both USDC token accounts (ATAs). The x402 requirement's payTo is
	//    the recipient's token account, so the bound payTo == destATA.
	sourceATA, _, err := solana.FindAssociatedTokenAddress(payerPub, usdcMint)
	if err != nil {
		log.Fatalf("derive payer ATA: %v", err)
	}
	destATA, _, err := solana.FindAssociatedTokenAddress(merchant, usdcMint)
	if err != nil {
		log.Fatalf("derive merchant ATA: %v", err)
	}

	// Pre-flight: print the exact payer wallet and confirm the source USDC token
	// account exists and is funded — so a missing/underfunded/ wrong-wallet case
	// is a clear message, not a raw on-chain simulation dump.
	rpcClient := rpc.New(rpc.DevNet_RPC)
	fmt.Printf("payer wallet: %s\n", payerPub)
	fmt.Printf("USDC mint:    %s\n", usdcMint)
	fmt.Printf("source ATA:   %s\n", sourceATA)
	fmt.Printf("dest ATA:     %s\n\n", destATA)

	bal, err := rpcClient.GetTokenAccountBalance(ctx, sourceATA, rpc.CommitmentConfirmed)
	if err != nil {
		log.Fatalf("no devnet USDC token account for wallet %s\n"+
			"  -> fund THIS exact address at https://faucet.circle.com (select Solana devnet)\n"+
			"  -> it must match `solana address`, not a browser wallet like Phantom\n"+
			"  (rpc: %v)", payerPub, err)
	}
	if bal == nil || bal.Value == nil {
		log.Fatalf("could not read USDC balance for %s (fund at https://faucet.circle.com)", sourceATA)
	}
	have, _ := strconv.ParseUint(bal.Value.Amount, 10, 64)
	if have < *amount {
		log.Fatalf("insufficient devnet USDC in %s: have %d, need %d micro-USDC\n"+
			"  -> top up at https://faucet.circle.com (Solana devnet)", sourceATA, have, *amount)
	}
	fmt.Printf("balance:      %d micro-USDC (ok)\n", have)

	// 4. Build the REAL TransferChecked. In -tamper mode we deliberately build the
	//    transfer to a DIFFERENT recipient than the bound payTo, to prove the guard
	//    refuses to sign before anything touches the chain.
	buildDest := destATA
	if *tamper {
		buildDest = solana.PublicKeyFromBytes(bytesRepeat(0xAA))
		fmt.Printf("\n[tamper] building transfer to %s\n", buildDest)
		fmt.Printf("[tamper] but the bound recipient is        %s\n", destATA)
	}
	ix := token.NewTransferCheckedInstruction(
		*amount, settle.USDCDecimals,
		sourceATA, usdcMint, buildDest, payerPub,
		nil, // no multisig signers
	).Build()

	// 5. §6.4 pre-send gate: decode the ACTUAL instruction and assert it pays the
	//    bound recipient / asset / amount under the payer's authority. Refuse to
	//    sign on any mismatch.
	if err := assertBound(ix, destATA, usdcMint, payerPub, *amount); err != nil {
		log.Fatalf("REFUSING TO SIGN — settle guard rejected the transfer: %v", err)
	}

	// 6. Assemble, sign, send, confirm.
	recent, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		log.Fatalf("get blockhash: %v", err)
	}
	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		recent.Value.Blockhash,
		solana.TransactionPayer(payerPub),
	)
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

	fmt.Printf("settled %d micro-USDC (%.6f USDC)\n", *amount, float64(*amount)/1e6)
	fmt.Printf("  payer:    %s\n", payerPub)
	fmt.Printf("  merchant: %s\n", merchant)
	fmt.Printf("  tx:       https://explorer.solana.com/tx/%s?cluster=devnet\n", sig)
}

// assertBound decodes the real instruction into the settle representation and
// runs the §6.4 assertion against it — the guard inspects the exact bytes that
// would be signed, not a stand-in.
func assertBound(ix solana.Instruction, destATA, mint, payer solana.PublicKey, amount uint64) error {
	data, err := ix.Data()
	if err != nil {
		return err
	}
	accounts := ix.Accounts()
	accts := make([][32]byte, len(accounts))
	for i, m := range accounts {
		accts[i] = [32]byte(m.PublicKey)
	}
	si := settle.Instruction{
		ProgramID: [32]byte(ix.ProgramID()),
		Data:      data,
		Accounts:  accts,
	}
	dec := settle.Decoder{TokenPrograms: [][32]byte{settle.SPLTokenProgramID}}
	tr, err := dec.FindTransfer([]settle.Instruction{si})
	if err != nil {
		return err
	}
	return settle.AssertMatches(tr, settle.BoundPayment{
		PayTo:  [32]byte(destATA),
		Asset:  [32]byte(mint),
		Payer:  [32]byte(payer),
		Amount: new(big.Int).SetUint64(amount),
	})
}
