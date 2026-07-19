// Package demo wires gate + settle into a runnable, in-process x402 loop:
//
//	resource server → 402 PaymentRequirements → gate decides → settle asserts
//	→ pay → resource released
//
// It uses no network — an in-process ResourceServer stands in for the HTTP
// server plus facilitator — so the full enforcement story (including a DENY and
// a tamper attempt) runs deterministically in one command. The real net/http
// server is M4 demo polish; the security logic exercised here is exactly
// gate.Evaluate + settle.AssertTransactionPays, unchanged.
package demo

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
	"github.com/rudizee007/spt-txn-x402-solana/receipt"
	"github.com/rudizee007/spt-txn-x402-solana/settle"
)

const (
	demoScheme   = "exact"
	demoNetwork  = "solana:devnet"
	demoDecimals = 6
)

// demoTokenProgram stands in for the SPL token program id in the mock.
var demoTokenProgram = fill(0x99)

func fill(b byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = b
	}
	return a
}

// Accounts are the actors in the scenario. Real deployments use real pubkeys;
// here they are fixed byte patterns so the demo is deterministic.
type Accounts struct {
	Asset    [32]byte // an SPL mint (USDC-like)
	Merchant [32]byte // the resource server's payTo
	Attacker [32]byte // a hostile payTo used to demonstrate tamper detection
	Payer    [32]byte // the authorized transfer authority
	Source   [32]byte // the payer's token account
}

// NewAccounts returns a fixed set of demo accounts.
func NewAccounts() Accounts {
	return Accounts{
		Asset:    fill(0xA1),
		Merchant: fill(0x4E),
		Attacker: fill(0xEE),
		Payer:    fill(0x0F),
		Source:   fill(0x50),
	}
}

func (a Accounts) allowlist() gate.Allowlist {
	return gate.Allowlist{
		Schemes:  map[string]byte{demoScheme: 1},
		Networks: map[string]byte{demoNetwork: 2},
	}
}

// Scope is the payer's authorization: pay up to Ceiling of Asset to any payee in
// Payees. (In production this lives inside the SPT-Txn token and is checked by
// the published verifier; here ScopePolicy stands in as the pluggable engine.)
type Scope struct {
	Ceiling uint64
	Asset   [32]byte
	Payees  map[[32]byte]bool
}

// ScopePolicy adapts a Scope to gate.PolicyVerifier.
type ScopePolicy struct{ scope Scope }

// Verify implements gate.PolicyVerifier: amount within ceiling, asset allowed,
// payee allowed. Returns a plain error (→ DENY_VIOLATION) on any breach.
func (p ScopePolicy) Verify(pr gate.PaymentRequirements, _ gate.Token) error {
	amt, ok := new(big.Int).SetString(pr.MaxAmountRequired, 10)
	if !ok {
		return errors.New("amount not an integer")
	}
	if amt.BitLen() > 64 || amt.Uint64() > p.scope.Ceiling {
		return errors.New("amount over ceiling")
	}
	if pr.Asset != gate.EncodeBase58(p.scope.Asset[:]) {
		return errors.New("asset not allowed")
	}
	for payee := range p.scope.Payees {
		if pr.PayTo == gate.EncodeBase58(payee[:]) {
			return nil
		}
	}
	return errors.New("payee not allowed")
}

// Payment is what the client submits back to the server (its X-PAYMENT).
type Payment struct {
	Instructions []settle.Instruction
	Signed       bool
}

// ResourceServer stands in for the x402 resource server + facilitator.
type ResourceServer struct {
	acc      Accounts
	price    uint64
	resource string
}

// NewResourceServer returns a server demanding `price` of the asset to `merchant`.
func NewResourceServer(acc Accounts, price uint64, resource string) *ResourceServer {
	return &ResourceServer{acc: acc, price: price, resource: resource}
}

// Require returns the 402 PaymentRequirements the server demands.
func (s *ResourceServer) Require() gate.PaymentRequirements {
	return gate.PaymentRequirements{
		Scheme:            demoScheme,
		Network:           demoNetwork,
		Asset:             gate.EncodeBase58(s.acc.Asset[:]),
		PayTo:             gate.EncodeBase58(s.acc.Merchant[:]),
		MaxAmountRequired: strconv.FormatUint(s.price, 10),
		Resource:          s.resource,
	}
}

// Settle is the mock facilitator: it independently verifies the transfer pays
// the server's own demand (right recipient, asset, amount) and is signed, then
// releases the resource. It does NOT trust the client's gate — this mirrors a
// real facilitator verifying before settling.
func (s *ResourceServer) Settle(p Payment) (string, error) {
	dec := settle.Decoder{TokenPrograms: [][32]byte{demoTokenProgram}}
	tr, err := dec.FindTransfer(p.Instructions)
	if err != nil {
		return "", err
	}
	if tr.Destination != s.acc.Merchant {
		return "", errors.New("server: wrong recipient")
	}
	if tr.Mint != s.acc.Asset {
		return "", errors.New("server: wrong asset")
	}
	if tr.Amount != s.price {
		return "", errors.New("server: wrong amount")
	}
	if !p.Signed {
		return "", errors.New("server: payment not signed")
	}
	return s.resource, nil
}

// buildTransfer is the mock tx builder: one SPL TransferChecked paying dest.
func buildTransfer(source, mint, dest, authority [32]byte, amount uint64) []settle.Instruction {
	data := make([]byte, 10)
	data[0] = settle.TransferCheckedDiscriminator
	binary.LittleEndian.PutUint64(data[1:9], amount)
	data[9] = demoDecimals
	return []settle.Instruction{{
		ProgramID: demoTokenProgram,
		Data:      data,
		Accounts:  [][32]byte{source, mint, dest, authority},
	}}
}

// Outcome is the result of one payment attempt.
type Outcome struct {
	Decision          gate.DecisionClass
	Reason            string
	Paid              bool
	Released          string
	AbortedBeforeSign bool
}

// Client runs the payer side of the loop.
type Client struct {
	acc      Accounts
	scope    Scope
	spend    gate.SpendLog
	receipts *receipt.Log
	rkey     ed25519.PrivateKey
}

// NewClient builds a payer with the given scope, a fresh single-use log, and a
// fresh receipt-signing key (distinct from any payment key).
func NewClient(acc Accounts, scope Scope) *Client {
	_, rkey, _ := ed25519.GenerateKey(nil)
	rpub := rkey.Public().(ed25519.PublicKey)
	return &Client{
		acc:      acc,
		scope:    scope,
		spend:    gate.NewMemSpendLog(),
		receipts: receipt.NewLog(rpub),
		rkey:     rkey,
	}
}

// ReceiptRoot is the RFC 6962 Merkle root over every decision this client has
// made — the value you would anchor on-chain (see cmd/anchordevnet).
func (c *Client) ReceiptRoot() [32]byte { return c.receipts.Root() }

// ReceiptCount is the number of decisions recorded.
func (c *Client) ReceiptCount() int { return c.receipts.Len() }

// Pay runs one full loop against server with the given token. If tamper is true
// the built transfer is redirected to the attacker, to demonstrate the §6.4
// settle-guard catching it before any signature.
func (c *Client) Pay(server *ResourceServer, token gate.Token, now time.Time, tamper bool) Outcome {
	req := server.Require()

	// Gate: ALLOW/DENY before signing.
	d := gate.Evaluate(c.acc.allowlist(), req, token, ScopePolicy{scope: c.scope}, c.spend, now)
	// Evidence as a byproduct: every decision emits a signed, chained receipt.
	_, _ = c.receipts.Append(c.rkey, receipt.Decision(d.Class), d.Binding, now.Unix())
	if d.Class != gate.Allow {
		return Outcome{Decision: d.Class, Reason: d.Reason}
	}

	// Build the transfer the client intends to sign.
	price, err := strconv.ParseUint(req.MaxAmountRequired, 10, 64)
	if err != nil {
		return Outcome{Decision: gate.DenyViolation, Reason: "client: bad price"}
	}
	dest := c.acc.Merchant
	if tamper {
		dest = c.acc.Attacker
	}
	ixs := buildTransfer(c.acc.Source, c.acc.Asset, dest, c.acc.Payer, price)

	// §6.4 pre-send assertion: refuse to sign unless the transfer pays the bound
	// recipient/asset/amount under the authorized payer.
	bound := settle.BoundPayment{
		PayTo:  c.acc.Merchant,
		Asset:  c.acc.Asset,
		Payer:  c.acc.Payer,
		Amount: new(big.Int).SetUint64(price),
	}
	dec := settle.Decoder{TokenPrograms: [][32]byte{demoTokenProgram}}
	if err := settle.AssertTransactionPays(dec, ixs, bound); err != nil {
		return Outcome{Decision: d.Class, Reason: err.Error(), AbortedBeforeSign: true}
	}

	// Sign (mock) and submit to the server/facilitator.
	released, err := server.Settle(Payment{Instructions: ixs, Signed: true})
	if err != nil {
		return Outcome{Decision: d.Class, Reason: "server rejected: " + err.Error()}
	}
	return Outcome{Decision: d.Class, Reason: d.Reason, Paid: true, Released: released}
}
