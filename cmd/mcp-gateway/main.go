// Command mcp-gateway is a live MCP (Model Context Protocol) server over stdio
// that exposes a single authorize_payment tool gated by the SPT-Txn enforcement point
// (package mcpgate). A real MCP client — Claude Desktop or any agent runtime —
// connects to it; every tool-call the agent makes is authorized against the one
// payment the human approved. A prompt-injected call (wrong recipient, inflated
// amount, different resource) is refused, with a signed receipt emitted either
// way.
//
// It reuses the SAME gate + receipt core as the HTTP x402 PEP. Protocol details
// (newline-delimited JSON-RPC 2.0; initialize / tools/list / tools/call) are
// implemented directly — no external MCP dependency.
//
// Run it directly for a smoke test, or register it as an MCP server:
//
//	{ "mcpServers": { "spt-txn": { "command": "go",
//	    "args": ["run", "./cmd/mcp-gateway"], "cwd": "/path/to/spt-txn-x402-solana" } } }
package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rudizee007/spt-txn-pep/gate"
	"github.com/rudizee007/spt-txn-pep/mcpgate"
	"github.com/rudizee007/spt-txn-pep/translog"
	"github.com/rudizee007/spt-txn-x402-solana/settle"
)

func b32(x byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = x
	}
	return a
}

func addr(x byte) string {
	a := b32(x)
	return gate.EncodeBase58(a[:])
}

// ── JSON-RPC 2.0 ──────────────────────────────────────────────────────────

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

func writeMsg(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	b = append(b, '\n')
	os.Stdout.Write(b)
}

func reply(id json.RawMessage, result interface{}) {
	writeMsg(rpcResp{JSONRPC: "2.0", ID: id, Result: result})
}

func replyErr(id json.RawMessage, code int, msg string) {
	writeMsg(rpcResp{JSONRPC: "2.0", ID: id, Error: &rpcErr{Code: code, Message: msg}})
}

// ── server ────────────────────────────────────────────────────────────────

type server struct {
	enf      *mcpgate.Enforcer
	merchant string
	asset    string
}

func main() {
	_, rk, err := ed25519.GenerateKey(nil)
	if err != nil {
		// Without a receipt key there is no evidence, and a decision with no
		// evidence is not a decision this system is willing to make.
		fmt.Fprintln(os.Stderr, "fatal: receipt key:", err)
		os.Exit(1)
	}
	merchant := demoMerchant()
	asset := gate.EncodeBase58(settle.USDCDevnetMint[:]) // devnet USDC mint

	s := &server{
		merchant: merchant,
		asset:    asset,
		enf: &mcpgate.Enforcer{
			Scheme:    "exact",
			Network:   "solana:devnet",
			Allowlist: gate.Allowlist{Schemes: map[string]byte{"exact": 1}, Networks: map[string]byte{"solana:devnet": 2}},
			Policy:    mcpgate.ExactPayment{Asset: asset, PayTo: merchant, Resource: "invoice:42", MaxAmount: 1_000_000},
			Spend:     gate.NewMemSpendLog(),
			Log:       translog.NewLog(rk.Public().(ed25519.PublicKey)),
			RKey:      rk,
			// real clock (Now nil → time.Now)
		},
	}

	// Diagnostics go to stderr — stdout is reserved for the MCP protocol.
	fmt.Fprintln(os.Stderr, "spt-txn mcp-gateway ready (stdio).")
	fmt.Fprintln(os.Stderr, "Approved capability: pay <=1 USDC to the MERCHANT for invoice:42.")
	fmt.Fprintln(os.Stderr, "  MERCHANT (approved):", merchant)
	fmt.Fprintln(os.Stderr, "  ATTACKER (for the injection test):", addr(0xEE))

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcReq
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		s.handle(req)
	}
}

func (s *server) handle(req rpcReq) {
	isRequest := len(req.ID) > 0 // requests have an id; notifications don't
	switch req.Method {
	case "initialize":
		ver := "2024-11-05"
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(req.Params, &p) == nil && p.ProtocolVersion != "" {
			ver = p.ProtocolVersion
		}
		reply(req.ID, map[string]interface{}{
			"protocolVersion": ver,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "spt-txn-mcp-gateway", "version": "0.1.0"},
		})
	case "tools/list":
		reply(req.ID, s.toolsList())
	case "tools/call":
		reply(req.ID, s.toolsCall(req.Params))
	case "resources/list":
		reply(req.ID, map[string]interface{}{"resources": []interface{}{}})
	case "prompts/list":
		reply(req.ID, map[string]interface{}{"prompts": []interface{}{}})
	case "ping":
		reply(req.ID, map[string]interface{}{})
	case "notifications/initialized", "notifications/cancelled":
		// notifications: no reply
	default:
		if isRequest {
			replyErr(req.ID, -32601, "method not found: "+req.Method)
		}
	}
}

func (s *server) toolsList() interface{} {
	return map[string]interface{}{
		"tools": []interface{}{
			map[string]interface{}{
				"name": "authorize_payment",
				"description": "Ask the SPT-Txn authorization enforcement point whether a proposed " +
					"payment is permitted by the policy a human pre-approved. Returns ALLOW or DENY. " +
					"This is a security demonstration on Solana devnet using valueless test tokens: the " +
					"agent holds no keys and moves no real funds — it only requests the authorization " +
					"decision. On ALLOW, the enforcement point (not the agent) records a devnet test " +
					"settlement and returns the transaction link.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"to":          map[string]interface{}{"type": "string", "description": "recipient: a base58 address, or the demo label \"merchant\" or \"attacker\""},
						"amount_usdc": map[string]interface{}{"type": "number", "description": "amount in USDC, at most 6 decimal places (required; an omitted amount is refused, not treated as zero)"},
						"resource":    map[string]interface{}{"type": "string", "description": "what is being paid for, e.g. invoice:42"},
					},
					"required": []interface{}{"to", "amount_usdc", "resource"},
				},
			},
		},
	}
}

func (s *server) toolsCall(params json.RawMessage) interface{} {
	var p struct {
		Name      string `json:"name"`
		Arguments struct {
			To string `json:"to"`
			// AmountUSDC is decoded as a *json.Number, not a float64, and the
			// pointer is load-bearing twice over.
			//
			// json.Number keeps the caller's digits as written. Decoding money
			// into a float64 and multiplying by 1e6 loses the low digits of a
			// large amount, and the float->uint64 conversion that followed is
			// implementation-defined when the value is out of range: on amd64
			// a huge amount became the MAXIMUM uint64, which is the wrong
			// direction for a spend ceiling to round.
			//
			// The pointer separates "absent" from "zero". Previously an
			// omitted amount_usdc decoded to 0.0 and asked the enforcement
			// point to authorize a zero-amount payment, which the policy
			// ceiling accepted — a required field that was not required.
			AmountUSDC *json.Number `json:"amount_usdc"`
			Resource   string       `json:"resource"`
		} `json:"arguments"`
	}
	dec := json.NewDecoder(bytes.NewReader(params))
	dec.UseNumber()
	if err := dec.Decode(&p); err != nil {
		return toolText("invalid tool arguments", true)
	}
	if p.Name != "authorize_payment" {
		return toolText("unknown tool: "+p.Name, true)
	}
	if p.Arguments.AmountUSDC == nil {
		return toolText("DENY: amount_usdc is required — an omitted amount is not a zero amount", true)
	}
	micro, err := microUSDC(string(*p.Arguments.AmountUSDC))
	if err != nil {
		return toolText("DENY: "+err.Error(), true)
	}
	atomic := strconv.FormatUint(micro, 10)
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		// A predictable or zero nonce is not a single-use authorization.
		return toolText("DENY: unable to generate a single-use nonce", true)
	}

	to := s.resolveTo(p.Arguments.To)
	r := s.enf.Authorize(mcpgate.ToolCall{
		To:       to,
		Asset:    s.asset,
		Amount:   atomic,
		Resource: p.Arguments.Resource,
		Nonce:    nonce,
		Expiry:   time.Now().Add(time.Minute),
	})
	if !r.Allowed() {
		return toolText(fmt.Sprintf("REFUSED by the SPT-Txn enforcement point: %s. The proposed payment is not authorized.", r.Reason), true)
	}

	// Authorized. settlePayment is a no-op in the default build; with -tags
	// devnet it performs a real USDC transfer and returns the tx signature.
	sig, err := settlePayment(to, micro)
	switch {
	case err != nil:
		return toolText(fmt.Sprintf("AUTHORIZED (log entry %s) — but the devnet test settlement failed: %v", r.LogEntry, err), true)
	case sig != "":
		return toolText(fmt.Sprintf("AUTHORIZED by the SPT-Txn enforcement point. The enforcement point settled the payment on Solana devnet (test tokens, no real value). Transparency-log entry %s.\n  tx: https://explorer.solana.com/tx/%s?cluster=devnet", r.LogEntry, sig), false)
	default:
		return toolText(fmt.Sprintf("AUTHORIZED by the SPT-Txn enforcement point (test mode; no settlement performed). Transparency-log entry %s.", r.LogEntry), false)
	}
}

// microUSDC converts a caller-supplied decimal USDC amount to integer micro-USDC
// (6 decimal places, the SPL mint's scale) without ever going through a float.
//
// The grammar is deliberately narrow — an optional integer part, an optional
// fractional part of at most six digits, no sign, no exponent, no leading
// zeros, and the result must be strictly positive:
//
//	amount = int [ "." frac ]
//	int    = "0" / ( %x31-39 *DIGIT )
//	frac   = 1*6DIGIT
//
// Everything it refuses, it refuses because accepting it would be a silent
// change of value: an exponent ("1e6") reads as one amount and means another;
// a seventh decimal place is precision the mint cannot hold, so accepting it
// would round the caller's number without saying so; a negative amount is not
// a payment; and zero authorizes nothing while still consuming an
// authorization, which is not something to grant by accident.
func microUSDC(s string) (uint64, error) {
	if s == "" {
		return 0, errors.New("amount_usdc is empty")
	}
	intPart, fracPart, hasFrac := strings.Cut(s, ".")
	if intPart == "" {
		return 0, fmt.Errorf("amount %q has no integer part — write 0.5, not .5", s)
	}
	if len(intPart) > 1 && intPart[0] == '0' {
		return 0, fmt.Errorf("amount %q has a leading zero", s)
	}
	if !allDigits(intPart) {
		return 0, fmt.Errorf("amount %q is not a plain decimal number (no sign, no exponent)", s)
	}
	if hasFrac {
		if fracPart == "" || !allDigits(fracPart) {
			return 0, fmt.Errorf("amount %q has a malformed fractional part", s)
		}
		if len(fracPart) > 6 {
			return 0, fmt.Errorf("amount %q has more than 6 decimal places; USDC cannot hold it, "+
				"and rounding a caller's amount silently is not an option", s)
		}
	}
	whole, err := strconv.ParseUint(intPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q is out of range", s)
	}
	// Pad the fraction to exactly 6 digits, then combine with checked arithmetic.
	for len(fracPart) < 6 {
		fracPart += "0"
	}
	frac, err := strconv.ParseUint(fracPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q is out of range", s)
	}
	if whole > (math.MaxUint64-frac)/1_000_000 {
		return 0, fmt.Errorf("amount %q overflows the 6-decimal atomic representation", s)
	}
	micro := whole*1_000_000 + frac
	if micro == 0 {
		return 0, errors.New("amount is zero; a zero-amount payment authorizes nothing")
	}
	return micro, nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// resolveTo maps the demo labels "merchant"/"attacker" to concrete addresses so
// an agent can be driven in natural language; any other value is treated as a
// literal base58 recipient address.
func (s *server) resolveTo(to string) string {
	switch to {
	case "merchant":
		return s.merchant
	case "attacker":
		return addr(0xEE)
	default:
		return to
	}
}

func toolText(text string, isError bool) interface{} {
	return map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": text},
		},
		"isError": isError,
	}
}
