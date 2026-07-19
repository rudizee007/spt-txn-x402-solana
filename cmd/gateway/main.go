// Command gateway runs the SPT-Txn PEP middleware in front of a protected
// resource and serves the receipt transparency log — all in process. It shows an
// authorized request served, a replay refused, an over-scope request denied, a
// missing-authorization rejected, and then the on-chain-anchorable Merkle root
// plus a verifiable inclusion proof for one decision.
//
//	go run ./cmd/gateway
package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
	"github.com/rudizee007/spt-txn-x402-solana/gateway"
	"github.com/rudizee007/spt-txn-x402-solana/receipt"
)

func b32(x byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = x
	}
	return a
}

type ceilingPolicy struct{ max uint64 }

func (c ceilingPolicy) Verify(pr gate.PaymentRequirements, _ gate.Token) error {
	amt, err := strconv.ParseUint(pr.MaxAmountRequired, 10, 64)
	if err != nil {
		return errors.New("bad amount")
	}
	if amt > c.max {
		return errors.New("amount over ceiling")
	}
	return nil
}

func main() {
	now := time.Unix(1_700_000_000, 0)
	_, rk, _ := ed25519.GenerateKey(nil)
	log := receipt.NewLog(rk.Public().(ed25519.PublicKey))

	usdc := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	pep := &gateway.PEP{
		Allowlist: gate.Allowlist{Schemes: map[string]byte{"exact": 1}, Networks: map[string]byte{"solana:devnet": 2}},
		Policy:    ceilingPolicy{max: 5_000_000},
		Spend:     gate.NewMemSpendLog(),
		Log:       log,
		RKey:      rk,
		Requirements: func(r *http.Request) gate.PaymentRequirements {
			amt := "1000000"
			if r.URL.Path == "/bulk" {
				amt = "10000000"
			}
			return gate.PaymentRequirements{
				Scheme: "exact", Network: "solana:devnet",
				Asset: usdc, PayTo: usdc, MaxAmountRequired: amt,
				Resource: "https://api.example.com" + r.URL.Path,
			}
		},
		Now: func() time.Time { return now },
	}

	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "premium data for "+r.URL.Path)
	})

	mux := http.NewServeMux()
	mux.Handle("/premium", pep.Wrap(protected))
	mux.Handle("/bulk", pep.Wrap(protected))
	mux.Handle("/transparency/", (&gateway.Transparency{Log: log}).Handler())

	ts := httptest.NewServer(mux)
	defer ts.Close()

	fmt.Println("SPT-Txn gateway (PEP) + transparency log —", ts.URL)
	fmt.Println("ceiling 5.000000 USDC, single-use tokens")
	fmt.Println()

	get := func(path, label, token string) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		if token != "" {
			req.Header.Set(gateway.HeaderToken, token)
		}
		resp, _ := http.DefaultClient.Do(req)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("%-28s -> %d  %s\n", label, resp.StatusCode, firstLine(string(body)))
	}

	tokA := gateway.EncodeToken(b32(0x5A), now.Add(time.Minute))
	tokB := gateway.EncodeToken(b32(0x6B), now.Add(time.Minute))
	tokC := gateway.EncodeToken(b32(0x7C), now.Add(time.Minute))

	get("/premium", "[1] authorized", tokA)
	get("/premium", "[2] replay (same token)", tokA)
	get("/bulk", "[3] over-scope (10 USDC)", tokB)
	get("/premium", "[4] no authorization", "")
	get("/premium", "[5] authorized again", tokC)

	fmt.Println()
	show := func(path string) {
		resp, _ := http.Get(ts.URL + path)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("GET %-28s %s\n", path, firstLine(string(body)))
	}
	show("/transparency/root")
	show("/transparency/receipt/0")
}

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}
