//go:build devnet

// Devnet build: on ALLOW, perform a REAL USDC TransferChecked on Solana devnet,
// gated by the settle guard (§6.4) — it refuses to sign unless the decoded
// transfer pays exactly the bound recipient/asset/amount. The signing key stays
// in your Solana keypair file (never read into an env var, never printed).
//
//	go get github.com/gagliardetto/solana-go
//	# fund your devnet wallet with devnet USDC: https://faucet.circle.com
//	export SPT_MERCHANT_ADDR=<a real devnet wallet>   # optional; default: pay yourself
//	go build -tags devnet -o bin/spt-txn-mcp ./cmd/mcp-gateway
//
// Any solana-go API name that has drifted in your SDK version is a small fix.
package main

import (
	"context"
	"fmt"
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

func keypairPath() string {
	if p := os.Getenv("SPT_KEYPAIR"); p != "" {
		return p
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "solana", "id.json")
}

// demoMerchant resolves the approved recipient wallet for the devnet build:
// SPT_MERCHANT_ADDR if set, otherwise the payer's own wallet (pay-to-self).
func demoMerchant() string {
	if a := os.Getenv("SPT_MERCHANT_ADDR"); a != "" {
		return a
	}
	payer, err := solana.PrivateKeyFromSolanaKeygenFile(keypairPath())
	if err != nil {
		panic(fmt.Sprintf("devnet mode: set SPT_MERCHANT_ADDR or provide a keypair at %s: %v", keypairPath(), err))
	}
	return payer.PublicKey().String()
}

// settlePayment performs a REAL USDC TransferChecked on devnet to toWallet's ATA,
// gated by the settle guard. Returns the confirmed transaction signature.
func settlePayment(toWallet string, micro uint64) (string, error) {
	ctx := context.Background()
	payer, err := solana.PrivateKeyFromSolanaKeygenFile(keypairPath())
	if err != nil {
		return "", fmt.Errorf("load keypair: %w", err)
	}
	payerPub := payer.PublicKey()
	usdcMint := solana.PublicKeyFromBytes(settle.USDCDevnetMint[:])

	merchant, err := solana.PublicKeyFromBase58(toWallet)
	if err != nil {
		return "", fmt.Errorf("bad recipient: %w", err)
	}
	sourceATA, _, err := solana.FindAssociatedTokenAddress(payerPub, usdcMint)
	if err != nil {
		return "", err
	}
	destATA, _, err := solana.FindAssociatedTokenAddress(merchant, usdcMint)
	if err != nil {
		return "", err
	}

	rpcClient := rpc.New(rpc.DevNet_RPC)
	bal, err := rpcClient.GetTokenAccountBalance(ctx, sourceATA, rpc.CommitmentConfirmed)
	if err != nil {
		return "", fmt.Errorf("no devnet USDC account for %s (fund at https://faucet.circle.com): %w", payerPub, err)
	}
	if bal == nil || bal.Value == nil {
		return "", fmt.Errorf("could not read USDC balance for %s", sourceATA)
	}
	have, _ := strconv.ParseUint(bal.Value.Amount, 10, 64)
	if have < micro {
		return "", fmt.Errorf("insufficient devnet USDC: have %d, need %d micro-USDC", have, micro)
	}

	transferIx := token.NewTransferCheckedInstruction(
		micro, settle.USDCDecimals,
		sourceATA, usdcMint, destATA, payerPub, nil,
	).Build()

	var ixs []solana.Instruction
	if !merchant.Equals(payerPub) {
		ixs = append(ixs, createATAIdempotent(payerPub, destATA, merchant, usdcMint))
	}
	ixs = append(ixs, transferIx)

	// §6.4 pre-send gate: decode the ACTUAL transaction and refuse to sign on
	// any mismatch with the bound payment.
	if err := assertBoundTx(ixs, destATA, usdcMint, payerPub, micro); err != nil {
		return "", fmt.Errorf("settle guard refused to sign: %w", err)
	}

	recent, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", err
	}
	tx, err := solana.NewTransaction(ixs, recent.Value.Blockhash, solana.TransactionPayer(payerPub))
	if err != nil {
		return "", err
	}
	if _, err := tx.Sign(func(k solana.PublicKey) *solana.PrivateKey {
		if k.Equals(payerPub) {
			return &payer
		}
		return nil
	}); err != nil {
		return "", err
	}
	wsClient, err := ws.Connect(ctx, rpc.DevNet_WS)
	if err != nil {
		return "", err
	}
	defer wsClient.Close()
	sig, err := confirm.SendAndConfirmTransaction(ctx, rpcClient, wsClient, tx)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

// ── settle-guard glue + ATA helper (mirrors cmd/paydevnet) ──────────────────

func assertBoundTx(ixs []solana.Instruction, destATA, mint, payer solana.PublicKey, amount uint64) error {
	sixs, err := toSettleIxs(ixs)
	if err != nil {
		return err
	}
	dec := settle.Decoder{TokenPrograms: [][32]byte{settle.SPLTokenProgramID}}
	return settle.AssertTransactionPays(dec, sixs, settle.BoundPayment{
		PayTo:  [32]byte(destATA),
		Asset:  [32]byte(mint),
		Payer:  [32]byte(payer),
		Amount: new(big.Int).SetUint64(amount),
	})
}

func toSettleIxs(ixs []solana.Instruction) ([]settle.Instruction, error) {
	out := make([]settle.Instruction, len(ixs))
	for i, ix := range ixs {
		data, err := ix.Data()
		if err != nil {
			return nil, err
		}
		accs := ix.Accounts()
		a := make([][32]byte, len(accs))
		for j, m := range accs {
			a[j] = [32]byte(m.PublicKey)
		}
		out[i] = settle.Instruction{ProgramID: [32]byte(ix.ProgramID()), Data: data, Accounts: a}
	}
	return out, nil
}

var (
	ataProgramID   = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	sysProgramID   = solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	tokenProgramID = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
)

type rawInstruction struct {
	prog  solana.PublicKey
	metas []*solana.AccountMeta
	data  []byte
}

func (r rawInstruction) ProgramID() solana.PublicKey     { return r.prog }
func (r rawInstruction) Accounts() []*solana.AccountMeta { return r.metas }
func (r rawInstruction) Data() ([]byte, error)           { return r.data, nil }

func createATAIdempotent(payer, ata, wallet, mint solana.PublicKey) solana.Instruction {
	return rawInstruction{
		prog: ataProgramID,
		metas: []*solana.AccountMeta{
			{PublicKey: payer, IsSigner: true, IsWritable: true},
			{PublicKey: ata, IsSigner: false, IsWritable: true},
			{PublicKey: wallet, IsSigner: false, IsWritable: false},
			{PublicKey: mint, IsSigner: false, IsWritable: false},
			{PublicKey: sysProgramID, IsSigner: false, IsWritable: false},
			{PublicKey: tokenProgramID, IsSigner: false, IsWritable: false},
		},
		data: []byte{1}, // CreateIdempotent
	}
}
