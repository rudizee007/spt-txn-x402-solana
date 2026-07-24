// Command mcp-gateway is a live MCP (Model Context Protocol) server over stdio
// that exposes a single make_payment tool gated by the SPT-Txn enforcement point
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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
	"github.com/rudizee007/spt-txn-x402-solana/mcpgate"
	"github.com/rudizee007/spt-txn-x402-solana/receipt"
)

const usdc = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

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
}

func main() {
	_, rk, _ := ed25519.GenerateKey(nil)
	merchant := addr(0x11)

	s := &server{
		merchant: merchant,
		enf: &mcpgate.Enforcer{
			Scheme:    "exact",
			Network:   "solana:devnet",
			Allowlist: gate.Allowlist{Schemes: map[string]byte{"exact": 1}, Networks: map[string]byte{"solana:devnet": 2}},
			Policy:    mcpgate.ExactPayment{Asset: usdc, PayTo: merchant, Resource: "invoice:42", MaxAmount: 1_000_000},
			Spend:     gate.NewMemSpendLog(),
			Log:       receipt.NewLog(rk.Public().(ed25519.PublicKey)),
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
				"name": "make_payment",
				"description": "Pay a recipient in USDC. Every call is authorized by an SPT-Txn " +
					"policy-enforcement point: only the exact payment the human approved is allowed; " +
					"any other recipient, amount, or resource is cryptographically refused.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"to":          map[string]interface{}{"type": "string", "description": "recipient: a base58 address, or the demo label \"merchant\" or \"attacker\""},
						"amount_usdc": map[string]interface{}{"type": "number", "description": "amount in USDC"},
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
			To         string  `json:"to"`
			AmountUSDC float64 `json:"amount_usdc"`
			Resource   string  `json:"resource"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return toolText("invalid tool arguments", true)
	}
	if p.Name != "make_payment" {
		return toolText("unknown tool: "+p.Name, true)
	}
	if p.Arguments.AmountUSDC < 0 {
		return toolText("DENY: negative amount", true)
	}

	atomic := strconv.FormatUint(uint64(math.Round(p.Arguments.AmountUSDC*1_000_000)), 10)
	var nonce [32]byte
	rand.Read(nonce[:])

	r := s.enf.Authorize(mcpgate.ToolCall{
		To:       s.resolveTo(p.Arguments.To),
		Asset:    usdc,
		Amount:   atomic,
		Resource: p.Arguments.Resource,
		Nonce:    nonce,
		Expiry:   time.Now().Add(time.Minute),
	})

	if r.Allowed() {
		return toolText(fmt.Sprintf("ALLOW — authorized by the SPT-Txn enforcement point (no funds moved in this demo). Receipt %s.", r.Receipt), false)
	}
	return toolText(fmt.Sprintf("DENY — refused by the SPT-Txn enforcement point: %s. No payment was made.", r.Reason), true)
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
