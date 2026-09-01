package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/rudizee007/spt-txn-pep/gate"
	"github.com/rudizee007/spt-txn-pep/mcpgate"
	"github.com/rudizee007/spt-txn-pep/translog"
)

// ── microUSDC ─────────────────────────────────────────────────────────────

func TestMicroUSDC_Accepts(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"1", 1_000_000},
		{"0.5", 500_000},
		{"0.000001", 1},
		{"1.5", 1_500_000},
		{"1.000001", 1_000_001},
		{"1000000", 1_000_000_000_000},
		{"0.100000", 100_000},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := microUSDC(c.in)
			if err != nil {
				t.Fatalf("microUSDC(%q) = error %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("microUSDC(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// Every refusal here is a value that would otherwise have been silently
// reinterpreted: rounded, sign-flipped, or converted through a float64 whose
// out-of-range behaviour is implementation-defined.
func TestMicroUSDC_Refuses(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"negative", "-1", "not a plain decimal"},
		{"explicit plus", "+1", "not a plain decimal"},
		{"exponent", "1e6", "not a plain decimal"},
		{"seven decimal places", "0.0000001", "more than 6 decimal places"},
		{"leading zero", "01", "leading zero"},
		{"no integer part", ".5", "no integer part"},
		{"trailing dot", "1.", "malformed fractional part"},
		{"zero", "0", "zero"},
		{"zero with decimals", "0.000000", "zero"},
		{"empty", "", "empty"},
		{"not a number", "abc", "not a plain decimal"},
		{"whitespace", " 1", "not a plain decimal"},
		{"integer part out of range", "99999999999999999999999999", "out of range"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := microUSDC(c.in)
			if err == nil {
				t.Fatalf("microUSDC(%q) was accepted as %d", c.in, got)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("microUSDC(%q) error %q does not diagnose %q", c.in, err, c.want)
			}
		})
	}
}

// The float64 path this replaced turned a large amount into the MAXIMUM uint64
// on amd64 — a spend ceiling rounding in the attacker's favour. Pin the
// direction: an amount that cannot be represented is refused, never saturated.
func TestMicroUSDC_OverflowIsRefusedNotSaturated(t *testing.T) {
	// One micro-USDC above the largest representable whole amount.
	tooBig := strconv.FormatUint(math.MaxUint64/1_000_000+1, 10)
	got, err := microUSDC(tooBig)
	if err == nil {
		t.Fatalf("an unrepresentable amount was accepted as %d (max uint64 is %d)", got, uint64(math.MaxUint64))
	}
	if !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("error %q does not diagnose the overflow", err)
	}
	// And the largest amount that IS representable still works.
	ok := strconv.FormatUint(math.MaxUint64/1_000_000-1, 10)
	if _, err := microUSDC(ok); err != nil {
		t.Fatalf("a representable amount was refused: %v", err)
	}
}

// ── the tool call ─────────────────────────────────────────────────────────

func newTestServer(t *testing.T) *server {
	t.Helper()
	_, rk, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	merchant := addr(0x11)
	asset := addr(0x22)
	return &server{
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
		},
	}
}

func callArgs(t *testing.T, args string) string {
	t.Helper()
	s := newTestServer(t)
	res := s.toolsCall(json.RawMessage(fmt.Sprintf(
		`{"name":"authorize_payment","arguments":%s}`, args)))
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The defect this pins: `required` in the input schema is advisory. An omitted
// amount_usdc used to decode to 0.0, and a zero amount is inside every ceiling,
// so the enforcement point authorized it.
func TestToolsCall_OmittedAmountIsRefusedNotTreatedAsZero(t *testing.T) {
	got := callArgs(t, `{"to":"merchant","resource":"invoice:42"}`)
	if strings.Contains(got, "AUTHORIZED") {
		t.Fatalf("an omitted amount was authorized: %s", got)
	}
	if !strings.Contains(got, "amount_usdc is required") {
		t.Fatalf("the refusal does not name the missing field: %s", got)
	}
}

func TestToolsCall_ExplicitZeroIsRefused(t *testing.T) {
	got := callArgs(t, `{"to":"merchant","amount_usdc":0,"resource":"invoice:42"}`)
	if strings.Contains(got, "AUTHORIZED") {
		t.Fatalf("a zero-amount payment was authorized: %s", got)
	}
}

func TestToolsCall_NegativeAmountIsRefused(t *testing.T) {
	got := callArgs(t, `{"to":"merchant","amount_usdc":-1,"resource":"invoice:42"}`)
	if strings.Contains(got, "AUTHORIZED") {
		t.Fatalf("a negative amount was authorized: %s", got)
	}
}

func TestToolsCall_HugeAmountIsRefusedNotSaturated(t *testing.T) {
	got := callArgs(t, `{"to":"merchant","amount_usdc":1e300,"resource":"invoice:42"}`)
	if strings.Contains(got, "AUTHORIZED") {
		t.Fatalf("an unrepresentable amount was authorized: %s", got)
	}
}

func TestToolsCall_InScopePaymentIsAuthorized(t *testing.T) {
	got := callArgs(t, `{"to":"merchant","amount_usdc":0.5,"resource":"invoice:42"}`)
	if !strings.Contains(got, "AUTHORIZED") {
		t.Fatalf("an in-scope payment was refused: %s", got)
	}
}

// The prompt-injection case the demo exists to show.
func TestToolsCall_HijackedRecipientIsRefused(t *testing.T) {
	got := callArgs(t, `{"to":"attacker","amount_usdc":0.5,"resource":"invoice:42"}`)
	if strings.Contains(got, "AUTHORIZED") {
		t.Fatalf("a hijacked recipient was authorized: %s", got)
	}
	if !strings.Contains(got, "REFUSED") {
		t.Fatalf("no refusal on the wire: %s", got)
	}
}

func TestToolsCall_OverLimitAmountIsRefused(t *testing.T) {
	got := callArgs(t, `{"to":"merchant","amount_usdc":2,"resource":"invoice:42"}`)
	if strings.Contains(got, "AUTHORIZED") {
		t.Fatalf("an over-limit payment was authorized: %s", got)
	}
}

func TestToolsCall_WrongResourceIsRefused(t *testing.T) {
	got := callArgs(t, `{"to":"merchant","amount_usdc":0.5,"resource":"invoice:99"}`)
	if strings.Contains(got, "AUTHORIZED") {
		t.Fatalf("a payment for the wrong resource was authorized: %s", got)
	}
}

func TestToolsCall_UnknownToolIsRefused(t *testing.T) {
	s := newTestServer(t)
	res := s.toolsCall(json.RawMessage(`{"name":"drain_wallet","arguments":{}}`))
	b, _ := json.Marshal(res)
	if !strings.Contains(string(b), "unknown tool") {
		t.Fatalf("an unknown tool was not refused: %s", b)
	}
}

func TestToolsCall_MalformedParamsAreRefused(t *testing.T) {
	s := newTestServer(t)
	res := s.toolsCall(json.RawMessage(`{"name":`))
	b, _ := json.Marshal(res)
	if !strings.Contains(string(b), "invalid tool arguments") {
		t.Fatalf("malformed params were not refused: %s", b)
	}
}

func TestResolveTo_LabelsMapToDistinctAddresses(t *testing.T) {
	s := newTestServer(t)
	if s.resolveTo("merchant") != s.merchant {
		t.Fatal("the merchant label does not resolve to the merchant")
	}
	if s.resolveTo("attacker") == s.merchant {
		t.Fatal("the attacker label resolves to the merchant — the injection demo proves nothing")
	}
	if got := s.resolveTo("SomeOtherBase58Addr"); got != "SomeOtherBase58Addr" {
		t.Fatalf("a literal address was rewritten to %q", got)
	}
}
