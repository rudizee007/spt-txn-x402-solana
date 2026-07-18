package demo

// This file wraps the in-process demo (Accounts / Scope / ResourceServer /
// gate + settle) as a real x402 exchange over HTTP: a resource server that
// answers an unpaid GET with 402 + PaymentRequirements, and a client that pays
// via the X-PAYMENT header and retries. The enforcement is unchanged — the same
// gate.Evaluate and settle.AssertTransactionPays — only the transport is new.

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
	"github.com/rudizee007/spt-txn-x402-solana/settle"
)

// ── wire types ──────────────────────────────────────────────────────────────

// prJSON is the x402 PaymentRequirements as sent on the wire (asset/payTo are
// base58, as x402 specifies).
type prJSON struct {
	Scheme            string `json:"scheme"`
	Network           string `json:"network"`
	Asset             string `json:"asset"`
	PayTo             string `json:"payTo"`
	MaxAmountRequired string `json:"maxAmountRequired"`
	Resource          string `json:"resource"`
}

// req402 is the JSON body of an HTTP 402 response.
type req402 struct {
	X402Version int      `json:"x402Version"`
	Accepts     []prJSON `json:"accepts"`
	Error       string   `json:"error,omitempty"`
}

// ixJSON is one instruction inside the X-PAYMENT payload. Pubkeys are hex here
// (this is our own payload format, not the 402 requirement), so no base58 is
// needed on the server decode path.
type ixJSON struct {
	ProgramID string   `json:"programId"` // 32-byte hex
	Data      string   `json:"data"`      // base64
	Accounts  []string `json:"accounts"`  // 32-byte hex each
}

// paymentJSON is the X-PAYMENT payload (base64 of this JSON goes in the header).
type paymentJSON struct {
	X402Version  int      `json:"x402Version"`
	Scheme       string   `json:"scheme"`
	Network      string   `json:"network"`
	Instructions []ixJSON `json:"instructions"`
	Signed       bool     `json:"signed"`
}

func prToJSON(pr gate.PaymentRequirements) prJSON {
	return prJSON{pr.Scheme, pr.Network, pr.Asset, pr.PayTo, pr.MaxAmountRequired, pr.Resource}
}

func prFromJSON(j prJSON) gate.PaymentRequirements {
	return gate.PaymentRequirements{
		Scheme:            j.Scheme,
		Network:           j.Network,
		Asset:             j.Asset,
		PayTo:             j.PayTo,
		MaxAmountRequired: j.MaxAmountRequired,
		Resource:          j.Resource,
	}
}

func pkHex(p [32]byte) string { return hex.EncodeToString(p[:]) }

func hexPk(s string) ([32]byte, error) {
	var p [32]byte
	b, err := hex.DecodeString(s)
	if err != nil {
		return p, err
	}
	if len(b) != 32 {
		return p, fmt.Errorf("pubkey must be 32 bytes, got %d", len(b))
	}
	copy(p[:], b)
	return p, nil
}

func ixsToJSON(ixs []settle.Instruction) []ixJSON {
	out := make([]ixJSON, len(ixs))
	for i, ix := range ixs {
		accts := make([]string, len(ix.Accounts))
		for j, a := range ix.Accounts {
			accts[j] = pkHex(a)
		}
		out[i] = ixJSON{
			ProgramID: pkHex(ix.ProgramID),
			Data:      base64.StdEncoding.EncodeToString(ix.Data),
			Accounts:  accts,
		}
	}
	return out
}

func ixsFromJSON(js []ixJSON) ([]settle.Instruction, error) {
	out := make([]settle.Instruction, len(js))
	for i, j := range js {
		pid, err := hexPk(j.ProgramID)
		if err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(j.Data)
		if err != nil {
			return nil, err
		}
		accts := make([][32]byte, len(j.Accounts))
		for k, s := range j.Accounts {
			pk, err := hexPk(s)
			if err != nil {
				return nil, err
			}
			accts[k] = pk
		}
		out[i] = settle.Instruction{ProgramID: pid, Data: data, Accounts: accts}
	}
	return out, nil
}

// ── server ──────────────────────────────────────────────────────────────────

// NewHTTPHandler exposes a ResourceServer as a real x402 endpoint at /resource:
// GET without X-PAYMENT → 402 + PaymentRequirements; GET with a valid X-PAYMENT
// → 200 + released content. Facilitator verification is the same
// ResourceServer.Settle used by the in-process demo, so the server independently
// rejects a payment that does not match its own demand.
func NewHTTPHandler(server *ResourceServer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/resource", func(w http.ResponseWriter, r *http.Request) {
		xp := r.Header.Get("X-PAYMENT")
		if xp == "" {
			write402(w, server, "payment required")
			return
		}
		raw, err := base64.StdEncoding.DecodeString(xp)
		if err != nil {
			http.Error(w, "bad X-PAYMENT encoding", http.StatusBadRequest)
			return
		}
		var pay paymentJSON
		if err := json.Unmarshal(raw, &pay); err != nil {
			http.Error(w, "bad X-PAYMENT json", http.StatusBadRequest)
			return
		}
		ixs, err := ixsFromJSON(pay.Instructions)
		if err != nil {
			http.Error(w, "bad instructions", http.StatusBadRequest)
			return
		}
		content, err := server.Settle(Payment{Instructions: ixs, Signed: pay.Signed})
		if err != nil {
			write402(w, server, err.Error())
			return
		}
		w.Header().Set("X-PAYMENT-RESPONSE", "settled")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "resource released: "+content)
	})
	return mux
}

func write402(w http.ResponseWriter, server *ResourceServer, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	_ = json.NewEncoder(w).Encode(req402{
		X402Version: 1,
		Accepts:     []prJSON{prToJSON(server.Require())},
		Error:       msg,
	})
}

// ── client ──────────────────────────────────────────────────────────────────

// HTTPClient runs the payer side of the x402 flow over HTTP, reusing the same
// gate + settle enforcement as the in-process demo.
type HTTPClient struct {
	acc    Accounts
	scope  Scope
	spend  gate.SpendLog
	client *http.Client
}

// NewHTTPClient builds a payer with a fresh single-use log.
func NewHTTPClient(acc Accounts, scope Scope) *HTTPClient {
	return &HTTPClient{acc: acc, scope: scope, spend: gate.NewMemSpendLog(), client: &http.Client{}}
}

// Pay performs GET → 402 → gate → settle → X-PAYMENT retry → 200 against url.
// tamper redirects the built transfer to the attacker, demonstrating the settle
// guard aborting before signing (no X-PAYMENT is ever sent in that case).
func (c *HTTPClient) Pay(url string, token gate.Token, now time.Time, tamper bool) (Outcome, error) {
	// 1. First GET — expect 402.
	resp, err := c.client.Get(url)
	if err != nil {
		return Outcome{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPaymentRequired {
		return Outcome{}, fmt.Errorf("expected 402, got %d", resp.StatusCode)
	}
	var r req402
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Outcome{}, err
	}
	if len(r.Accepts) == 0 {
		return Outcome{}, fmt.Errorf("402 response had no accepts")
	}
	req := prFromJSON(r.Accepts[0])

	// 2. Gate: ALLOW / DENY before signing.
	d := gate.Evaluate(c.acc.allowlist(), req, token, ScopePolicy{scope: c.scope}, c.spend, now)
	if d.Class != gate.Allow {
		return Outcome{Decision: d.Class, Reason: d.Reason}, nil
	}

	// 3. Build transfer + §6.4 pre-send assertion.
	price, err := strconv.ParseUint(req.MaxAmountRequired, 10, 64)
	if err != nil {
		return Outcome{Decision: gate.DenyViolation, Reason: "client: bad price"}, nil
	}
	dest := c.acc.Merchant
	if tamper {
		dest = c.acc.Attacker
	}
	ixs := buildTransfer(c.acc.Source, c.acc.Asset, dest, c.acc.Payer, price)
	bound := settle.BoundPayment{PayTo: c.acc.Merchant, Asset: c.acc.Asset, Payer: c.acc.Payer, Amount: new(big.Int).SetUint64(price)}
	dec := settle.Decoder{TokenPrograms: [][32]byte{demoTokenProgram}}
	if err := settle.AssertTransactionPays(dec, ixs, bound); err != nil {
		return Outcome{Decision: d.Class, Reason: err.Error(), AbortedBeforeSign: true}, nil
	}

	// 4. Build X-PAYMENT and resend.
	payload, err := json.Marshal(paymentJSON{
		X402Version:  1,
		Scheme:       req.Scheme,
		Network:      req.Network,
		Instructions: ixsToJSON(ixs),
		Signed:       true,
	})
	if err != nil {
		return Outcome{}, err
	}
	req2, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Outcome{}, err
	}
	req2.Header.Set("X-PAYMENT", base64.StdEncoding.EncodeToString(payload))
	resp2, err := c.client.Do(req2)
	if err != nil {
		return Outcome{}, err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		return Outcome{Decision: d.Class, Reason: "server rejected payment (status " + strconv.Itoa(resp2.StatusCode) + ")"}, nil
	}
	body, _ := io.ReadAll(resp2.Body)
	return Outcome{Decision: d.Class, Reason: d.Reason, Paid: true, Released: string(body)}, nil
}
