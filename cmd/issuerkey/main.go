// Command issuerkey binds one Ed25519 identity across the two halves of the
// system: the CAT issuer inside an RFC 8693 identity exchange, and the issuer
// the Solana escrow trusts on chain.
//
// The escrow verifies an issuer signature over a fixed-width binding, with the
// issuer required to be on an on-chain allowlist AND to be the key the payer
// pinned at deposit. The identity bridge signs CATs with an Ed25519 key seeded
// from SPT_IDP_CAT_SEED_HEX. Those are the same kind of key, so one key can be
// both — which makes the authority established by an enterprise identity
// exchange the same authority a Solana program enforces.
//
// This does NOT put a CAT on chain and does not make the escrow verify one. It
// binds the two sides at the KEY. Say it that way.
//
//	# you already have an escrow issuer key: derive the bridge config from it
//	go run ./cmd/issuerkey -from-key ~/.config/spt-txn/issuer-devnet.json
//
//	# or mint a fresh identity usable by both sides
//	go run ./cmd/issuerkey -new -out ~/.config/spt-txn/issuer-devnet.json
//
// The seed is a SECRET and is printed only with -print-seed. Do that off camera.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gagliardetto/solana-go"
)

func main() {
	var (
		fromKey   = flag.String("from-key", "", "read an existing Solana keygen json key file and report its identity")
		newKey    = flag.Bool("new", false, "generate a new key and write it in Solana keygen json format")
		out       = flag.String("out", "", "path to write with -new (refuses to overwrite)")
		printSeed = flag.Bool("print-seed", false, "print the 32-byte seed hex for SPT_IDP_CAT_SEED_HEX — a SECRET, off camera only")
	)
	flag.Parse()

	var priv ed25519.PrivateKey

	switch {
	case *fromKey != "" && *newKey:
		die("choose one of -from-key or -new, not both")

	case *fromKey != "":
		priv = readSolanaKeyFile(*fromKey)

	case *newKey:
		if *out == "" {
			die("-new requires -out")
		}
		if _, err := os.Stat(*out); err == nil {
			die(*out + " already exists — refusing to overwrite an issuer key")
		} else if !errors.Is(err, os.ErrNotExist) {
			die("stat " + *out + ": " + err.Error())
		}
		var err error
		if _, priv, err = ed25519.GenerateKey(rand.Reader); err != nil {
			die("generate: " + err.Error())
		}
		writeSolanaKeyFile(*out, priv)
		fmt.Printf("wrote %s (mode 0600)\n\n", *out)

	default:
		flag.Usage()
		os.Exit(2)
	}

	pub := priv.Public().(ed25519.PublicKey)

	fmt.Println("One identity, both sides:")
	fmt.Println()
	fmt.Printf("  public key (base58)   %s\n", solana.PublicKeyFromBytes(pub))
	fmt.Println("      -> the escrow allowlist entry, and what the payer pins at deposit")
	fmt.Printf("  public key (hex)      %s\n", hex.EncodeToString(pub))
	fmt.Println("      -> the CAT issuer key a verifier or trust-registry snapshot records")
	fmt.Println()

	if *printSeed {
		fmt.Printf("  SPT_IDP_CAT_SEED_HEX=%s\n", hex.EncodeToString(priv.Seed()))
		fmt.Println()
		fmt.Println("  ^ SECRET. Do not commit it, do not screenshot it, do not leave it in shell history.")
	} else {
		fmt.Println("  Re-run with -print-seed to emit SPT_IDP_CAT_SEED_HEX for the identity bridge.")
		fmt.Println("  It is withheld by default so it cannot appear on a screen recording by accident.")
	}
}

// readSolanaKeyFile parses the Solana keygen json array the escrow tooling uses:
// 64 ints, the Ed25519 private key as seed||pub.
func readSolanaKeyFile(path string) ed25519.PrivateKey {
	buf, err := os.ReadFile(path)
	if err != nil {
		die("read " + path + ": " + err.Error())
	}
	var ints []int
	if err := json.Unmarshal(buf, &ints); err != nil {
		die(path + " is not a Solana keygen json array: " + err.Error())
	}
	if len(ints) != ed25519.PrivateKeySize {
		die(fmt.Sprintf("%s is %d bytes, want %d", path, len(ints), ed25519.PrivateKeySize))
	}
	key := make(ed25519.PrivateKey, len(ints))
	for i, v := range ints {
		if v < 0 || v > 255 {
			die(path + " contains a non-byte value")
		}
		key[i] = byte(v)
	}
	return key
}

func writeSolanaKeyFile(path string, priv ed25519.PrivateKey) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		die("mkdir: " + err.Error())
	}
	ints := make([]int, len(priv))
	for i, b := range priv {
		ints[i] = int(b)
	}
	buf, err := json.Marshal(ints)
	if err != nil {
		die("encode: " + err.Error())
	}
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		die("write " + path + ": " + err.Error())
	}
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, "issuerkey: "+msg)
	os.Exit(1)
}
