//go:build devnet

// Command escrowdevnet drives the release-on-proof settlement path end to end on
// Solana devnet: the gate decides, funds move into a program-owned vault, and the
// ONLY way out to the recipient is an instruction the on-chain program executes
// solely against a fresh, allowlisted, correctly-bound issuer attestation.
//
// This is the difference between the two enforcement points, made concrete.
// cmd/paydevnet demonstrates the pre-send guard (SPEC-X402 §6.4): it decodes the
// real transaction and refuses to sign a transfer that does not match the bound
// payment. That is the right control for a cooperating client, but it is still
// the payer's own code refusing to sign — nothing on chain stops a payer who
// deletes the guard. Here the payer's cooperation is not part of the trust model
// at all. Once init_escrow has run, the funds are held by a PDA whose only exit
// is release_with_proof, and that instruction fails closed on every path.
//
// Modes, in the order the runsheet uses them:
//
//	setup          init_config (once per deployment) + add_issuer
//	deposit        gate ALLOW -> init_escrow, USDC moves payer -> vault
//	deny-binding   a VALIDLY SIGNED attestation over the wrong binding  -> 6105
//	deny-issuer    a VALIDLY SIGNED attestation from a rogue issuer     -> 6102
//	deny-unpinned  a COMPROMISED ADMIN allowlists a rogue issuer and it
//	               still cannot release                                 -> 6108
//	release        the real proof                                       -> funds released
//	refund         after expiry, funds return to the payer
//	all            setup (if needed), deposit, all three denies, release
//
// Why the deny demos sign for real: tampering with the *signature* makes the
// native Ed25519 precompile fail during transaction verification, so the
// transaction is dropped by the validator and never lands. A dropped transaction
// is not evidence of anything — there is no explorer link and no program log. To
// produce an on-chain DENY the signature must be genuine and the failure must
// happen INSIDE the program, which is exactly what a wrong binding (6105
// BindingMismatch) and a non-allowlisted issuer (6102 IssuerNotAuthorized)
// produce. All are sent with preflight skipped so the failure is recorded on
// chain rather than rejected by the RPC node's simulation.
//
// deny-unpinned is the strongest of the three and the reason the others are not
// enough on their own. deny-issuer proves an OUTSIDER cannot release. It says
// nothing about the admin, who by definition can turn any outsider into an
// allowlisted issuer with one transaction. So deny-unpinned hands the attacker
// the admin key and lets them do exactly that: mint a key, add_issuer it for
// real (that transaction succeeds and is on chain), then sign a fresh, correctly
// bound attestation. It fails with 6108 IssuerNotPinned, because the payer named
// their issuer at deposit and no admin instruction can reach that field. That is
// the control-of-customer-funds property stated as an on-chain fact rather than
// a paragraph of prose: a fully compromised admin can stop payments, and cannot
// redirect one.
//
// Keys. THREE distinct secrets are involved and none is ever printed, logged,
// put in an environment variable, or written into the repository:
//
//   - the payer/releaser wallet stays in the Solana CLI keypair file. On devnet
//     this is also the program's upgrade authority, because it deployed it;
//   - the ISSUER-ALLOWLIST ADMIN key stays in its own file (default
//     ~/.config/spt-txn/admin-devnet.json, created 0600 by -gen-admin);
//   - the SPT-Txn issuer's Ed25519 key stays in its own file (default
//     ~/.config/spt-txn/issuer-devnet.json, created 0600 by -gen-issuer).
//
// The admin is a separate key because the program refuses to let one key hold
// both roles: init_config rejects an admin equal to the signing upgrade
// authority (6306 AdminIsUpgradeAuthority), and accept_admin re-checks it so a
// later rotation cannot quietly re-collapse them. Separation of duties here is a
// runtime constraint, not a deployment convention — so the demo cannot run with
// one key even if that would be more convenient. The admin never pays a fee and
// never needs SOL: it signs add_issuer/remove_issuer and nothing else.
//
// The rogue key used by deny-issuer is generated in memory for one transaction
// and never touches disk.
//
//	go run -tags devnet ./cmd/escrowdevnet -gen-admin       # once: create the admin key
//	go run -tags devnet ./cmd/escrowdevnet -gen-issuer      # once: create the issuer key
//	go run -tags devnet ./cmd/escrowdevnet -mode setup      # once per deployment
//	go run -tags devnet ./cmd/escrowdevnet -mode all        # the full demo
//
// Admin ROTATION (propose_admin then accept_admin) is deliberately not a mode
// here. It needs the nominee's key to sign step two, which makes it a two-party
// flow that a single-operator devnet script can only fake; the program's own
// test suite holds both keys and asserts the real property instead. See
// tests/integration.rs::admin_rotation_is_two_step_and_transfers_the_role.
//
// Needs devnet SOL for fees/rent and devnet USDC (https://faucet.circle.com).
//
// Like cmd/paydevnet, this file could not be compiled in the authoring
// environment: solana-go is unreachable there (proxy.golang.org is refused by
// the egress allowlist), so `go build` cannot resolve the import graph. Expect
// to `go mod download` first; if a solana-go API name has drifted in your
// version, it is a small fix.
//
// What WAS verified offline, and why that is the part that matters: the escrow
// package has no external dependencies at all, so `go vet ./escrow/...` and its
// full test suite run clean without the proxy — including the known-answer test
// that pins all eight Anchor discriminators and the 112-byte init_escrow
// encoding. Every call site in this file was then checked against those
// declarations by AST. Nothing security-relevant lives in the SDK calls: every
// address, discriminator and byte layout comes from the escrow package. A
// solana-go drift costs a compile error, which is loud. A wrong byte layout
// would be silent, and that is what the offline tests cover.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/rudizee007/spt-txn-x402-solana/escrow"
	"github.com/rudizee007/spt-txn-x402-solana/gate"
	"github.com/rudizee007/spt-txn-x402-solana/settle"
)

// Anchor adds 6000 to every #[error_code] discriminant. These are the two the
// deny demos must produce; seeing any other number means the program rejected
// for a reason we did not intend to demonstrate, which is a failed demo, not a
// successful one.
const (
	errIssuerNotAuthorized = 6102
	errBindingMismatch     = 6105
	errIssuerNotPinned     = 6108
)

// Native addresses this command needs that are not part of the escrow package's
// security contract (they are deploy plumbing, not protocol).
var (
	rentSysvarID           = solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")
	bpfLoaderUpgradeableID = solana.MustPublicKeyFromBase58("BPFLoaderUpgradeab1e11111111111111111111111")
	ataProgramID           = solana.MustPublicKeyFromBase58(escrow.AssociatedTokenProgramIDBase58)
)

func defaultKeypair() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "solana", "id.json")
}

func defaultIssuerKey() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "spt-txn", "issuer-devnet.json")
}

func defaultAdminKey() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "spt-txn", "admin-devnet.json")
}

func main() {
	var (
		keypairPath  = flag.String("keypair", defaultKeypair(), "Solana CLI keypair json for the payer/releaser (devnet); on devnet this is also the program's upgrade authority")
		issuerPath   = flag.String("issuer", defaultIssuerKey(), "SPT-Txn issuer Ed25519 key file (Solana keygen json format)")
		adminPath    = flag.String("admin", defaultAdminKey(), "issuer-allowlist admin key file; MUST be a different key from -keypair (the program enforces it)")
		issuerPubB58 = flag.String("issuer-pub", "", "pin THIS issuer public key at deposit (base58). Default: the public half of -issuer. A payer only ever needs the public key; this demo happens to hold both roles")
		genIssuer    = flag.Bool("gen-issuer", false, "create the issuer key file (0600) if it does not exist, print only its public key, and exit")
		genAdmin     = flag.Bool("gen-admin", false, "create the admin key file (0600) if it does not exist, print only its public key, and exit")
		mode         = flag.String("mode", "all", "setup | deposit | deny-binding | deny-issuer | deny-unpinned | release | refund | all")
		toStr        = flag.String("to", "", "recipient wallet pubkey (base58); default: yourself")
		amount       = flag.Uint64("amount", 100_000, "amount in micro-USDC (100000 = 0.10 USDC)")
		resource     = flag.String("resource", "https://api.example.com/v1/quote", "the x402 resource being paid for")
		ticketPath   = flag.String("ticket", "escrow-ticket.json", "where deposit records this escrow's public parameters for the later release")
		programStr   = flag.String("program", escrow.EscrowProgramIDBase58, "deployed spt_x402_escrow program id")
	)
	flag.Parse()
	ctx := context.Background()

	if *genIssuer {
		generateKeyFile(*issuerPath, roleIssuer)
		return
	}
	if *genAdmin {
		generateKeyFile(*adminPath, roleAdmin)
		return
	}

	programID, err := escrow.DecodePubkey(*programStr)
	if err != nil {
		log.Fatalf("bad -program id: %v", err)
	}
	programPub := solana.PublicKeyFromBytes(programID[:])

	payer, err := solana.PrivateKeyFromSolanaKeygenFile(*keypairPath)
	if err != nil {
		log.Fatalf("load keypair %q: %v", *keypairPath, err)
	}
	payerPub := payer.PublicKey()

	cl := rpc.New(rpc.DevNet_RPC)
	env := &env{
		ctx:      ctx,
		cl:       cl,
		payer:    payer,
		payerPub: payerPub,
		program:  programPub,
		progID:   programID,
		ticket:   *ticketPath,
	}

	fmt.Printf("program:  %s\n", programPub)
	fmt.Printf("payer:    %s\n", payerPub)
	fmt.Printf("mode:     %s\n\n", *mode)

	// The admin key is loaded only for the modes that actually need an admin
	// signature. Requiring it for `-mode release` would be a lie about who can
	// release: the admin has no part in the release path at all.
	switch *mode {
	case "setup", "deny-unpinned", "all":
		env.loadAdmin(*adminPath)
	}

	switch *mode {
	case "setup":
		env.setup(loadIssuerPub(*issuerPath))
	case "deposit":
		env.deposit(*toStr, *amount, *resource, pinnedIssuer(*issuerPubB58, *issuerPath))
	case "deny-binding":
		env.release(loadIssuer(*issuerPath), denyBinding)
	case "deny-issuer":
		env.release(rogueIssuer(), denyIssuer)
	case "deny-unpinned":
		env.denyCompromisedAdmin()
	case "release":
		env.release(loadIssuer(*issuerPath), allow)
	case "refund":
		env.refund()
	case "all":
		issuer := loadIssuer(*issuerPath)
		env.setup(issuer.Public().(ed25519.PublicKey))
		env.deposit(*toStr, *amount, *resource, pinnedIssuer(*issuerPubB58, *issuerPath))
		env.release(issuer, denyBinding)
		env.release(rogueIssuer(), denyIssuer)
		env.denyCompromisedAdmin()
		env.release(issuer, allow)
	default:
		log.Fatalf("unknown -mode %q", *mode)
	}
}

// pinnedIssuer resolves the issuer public key the payer will pin into the
// escrow. -issuer-pub takes precedence, because that is the realistic shape:
// the payer knows the issuer by its PUBLIC key and has no business holding the
// private half. Falling back to the key file is a convenience of this
// single-operator demo, not a property of the design.
func pinnedIssuer(b58, issuerPath string) [32]byte {
	var out [32]byte
	if b58 != "" {
		k, err := escrow.DecodePubkey(b58)
		if err != nil {
			log.Fatalf("bad -issuer-pub: %v", err)
		}
		return k
	}
	copy(out[:], loadIssuerPub(issuerPath))
	return out
}

type env struct {
	ctx      context.Context
	cl       *rpc.Client
	payer    solana.PrivateKey
	payerPub solana.PublicKey
	program  solana.PublicKey
	progID   [32]byte
	ticket   string

	// The issuer-allowlist admin. nil unless the mode needs it — the release and
	// refund paths never touch it, and carrying it around for them would blur the
	// one property this program exists to demonstrate.
	admin    solana.PrivateKey
	adminPub solana.PublicKey
}

// loadAdmin reads the admin key and refuses the collapsed-roles configuration
// locally, before spending a devnet round trip on it. The program rejects it too
// (6306 AdminIsUpgradeAuthority) — this check exists so the operator gets a
// sentence explaining WHY rather than an Anchor error number.
func (e *env) loadAdmin(path string) {
	key := solana.PrivateKey(loadEd25519KeyFile(path, roleAdmin))
	pub := key.PublicKey()
	if pub.Equals(e.payerPub) {
		log.Fatalf("admin key %s is the SAME key as the payer/upgrade authority (%s).\n"+
			"  The program refuses this: one key holding both the upgrade authority and the\n"+
			"  issuer allowlist is exactly the concentration the separation is meant to prevent.\n"+
			"  Create a distinct admin key with:  go run -tags devnet ./cmd/escrowdevnet -gen-admin",
			path, pub)
	}
	e.admin, e.adminPub = key, pub
}

// ─────────────────────────────── setup ──────────────────────────────────────

// setup creates the Config (deny-by-default: the allowlist starts EMPTY) and
// authorizes one issuer. Both steps are conditional, so running it twice is
// harmless rather than an IssuerAlreadyPresent failure.
//
// init_config can only be run by the program's upgrade authority — the program
// account and its ProgramData are passed in and the program checks that the
// signer is the recorded upgrade authority. That is what stops anyone else from
// front-running the deployment and installing themselves as admin.
//
// It NAMES a different key as admin. The upgrade authority signs; the admin is
// passed as an account and does not sign, because the point of the instruction
// is to record a role, not to exercise it. The program refuses if the two keys
// match. What the admin can then do is bounded to exactly two instructions —
// add_issuer and remove_issuer — and neither of them can cause a release: every
// release is additionally gated on the issuer the payer pinned at deposit
// (THREAT-MODEL T9). A stolen admin key buys an attacker the ability to add
// issuers nobody has pinned, or to revoke issuers and strand escrows into the
// refund path. Its best available outcome is that payers get their money back.
func (e *env) setup(issuerPub ed25519.PublicKey) {
	var issuer [32]byte
	copy(issuer[:], issuerPub)

	cfg := e.pda("config", [][]byte{[]byte(escrow.SeedConfig)}, func() ([32]byte, uint8, error) {
		return escrow.ConfigPDA(e.progID)
	})

	fmt.Printf("config PDA:   %s\n", cfg)
	fmt.Printf("upgrade auth: %s (signs init_config; does NOT become admin)\n", e.payerPub)
	fmt.Printf("admin:        %s (add_issuer/remove_issuer only)\n", e.adminPub)
	fmt.Printf("issuer:       %s\n", solana.PublicKeyFromBytes(issuer[:]))

	onChainAdmin, pending, issuers, exists := e.readConfig(cfg)
	if !exists {
		programData, _, err := solana.FindProgramAddress([][]byte{e.program.Bytes()}, bpfLoaderUpgradeableID)
		if err != nil {
			log.Fatalf("derive ProgramData: %v", err)
		}
		fmt.Printf("program data: %s\n", programData)
		fmt.Println("\ninit_config — creating the config with an EMPTY allowlist (deny-by-default)")

		// Account order is the InitConfig struct's field order, which is the wire
		// format: config, upgrade_authority, admin, program, program_data, system.
		// `admin` is NOT a signer — it is being named, not exercised.
		ix := rawInstruction{
			prog: e.program,
			metas: []*solana.AccountMeta{
				{PublicKey: cfg, IsWritable: true},
				{PublicKey: e.payerPub, IsSigner: true, IsWritable: true},
				{PublicKey: e.adminPub},
				{PublicKey: e.program},
				{PublicKey: programData},
				{PublicKey: solana.SystemProgramID},
			},
			data: escrow.InitConfigData(),
		}
		sig := e.send("init_config", []solana.Instruction{ix}, false)
		fmt.Printf("  tx: %s\n", explorer(sig))
	} else {
		fmt.Printf("config already exists (admin %s, %d issuer(s) authorized)\n",
			onChainAdmin, len(issuers))
		if !pending.IsZero() {
			fmt.Printf("  NOTE: an admin handover to %s is pending acceptance\n", pending)
		}
		// The on-chain admin is the only key add_issuer will accept. Catching the
		// mismatch here turns a 2101 ConstraintHasOne into a sentence.
		if !onChainAdmin.Equals(e.adminPub) {
			log.Fatalf("the config's admin is %s, but -admin holds %s.\n"+
				"  add_issuer will be rejected. Point -admin at the key this deployment was\n"+
				"  configured with, or hand the role over with propose_admin/accept_admin.",
				onChainAdmin, e.adminPub)
		}
		for _, k := range issuers {
			if k == issuer {
				fmt.Println("issuer is already on the allowlist — nothing to do")
				return
			}
		}
	}

	fmt.Println("\nadd_issuer — authorizing the SPT-Txn issuer key (signed by the ADMIN, not the deployer)")
	ix := rawInstruction{
		prog: e.program,
		metas: []*solana.AccountMeta{
			{PublicKey: cfg, IsWritable: true},
			{PublicKey: e.adminPub, IsSigner: true},
		},
		data: escrow.AddIssuerData(issuer),
	}
	sig := e.send("add_issuer", []solana.Instruction{ix}, false)
	fmt.Printf("  tx: %s\n", explorer(sig))
}

// readConfig fetches and decodes the Config account. The account discriminator
// is checked before any field is read, so a same-address account of a different
// type cannot be mistaken for a config.
//
// The layout is Borsh, which carries NO field names on the wire — a decoder that
// disagrees with the program about field order or width does not error, it
// returns confident nonsense. The layout this decodes is:
//
//	[0:8]      account discriminator
//	[8:40]     admin
//	[40:72]    pending_admin        (zero when no handover is in flight)
//	[72:76]    issuer vec length    (u32, little-endian)
//	[76+32i:]  issuer i
//
// `pending_admin` was added with the two-step admin handover. A decoder still
// reading the previous layout takes the first four bytes of that pubkey as the
// issuer count — which lands on the implausible-count guard below only by luck,
// so the guard is a backstop, not the check that matters. Keep these offsets in
// step with state.rs::Config.
func (e *env) readConfig(cfg solana.PublicKey) (admin, pendingAdmin solana.PublicKey, issuers [][32]byte, exists bool) {
	res, err := e.cl.GetAccountInfo(e.ctx, cfg)
	if err != nil || res == nil || res.Value == nil {
		return admin, pendingAdmin, nil, false
	}
	data := res.Value.Data.GetBinary()
	want := escrow.AccountDiscriminator("Config")
	if len(data) < 8+32+32+4+1 || string(data[:8]) != string(want[:]) {
		log.Fatalf("account %s is not a spt_x402_escrow Config (discriminator mismatch)", cfg)
	}
	admin = solana.PublicKeyFromBytes(data[8:40])
	pendingAdmin = solana.PublicKeyFromBytes(data[40:72])
	n := int(uint32(data[72]) | uint32(data[73])<<8 | uint32(data[74])<<16 | uint32(data[75])<<24)
	if n < 0 || n > 16 || len(data) < 76+32*n {
		log.Fatalf("config account %s has an implausible issuer count (%d) — "+
			"if the program's Config layout changed, this decoder is stale", cfg, n)
	}
	for i := 0; i < n; i++ {
		var k [32]byte
		copy(k[:], data[76+32*i:])
		issuers = append(issuers, k)
	}
	return admin, pendingAdmin, issuers, true
}

// ────────────────────────────── deposit ─────────────────────────────────────

// deposit runs the whole authorization story for one payment and then locks the
// funds: the gate decides ALLOW on the x402 requirements, FromGate maps that
// decision onto escrow parameters (verifying, not trusting, that the recipient
// wallet really owns the requirement's payTo token account), and init_escrow
// moves the USDC into a vault owned by the escrow PDA.
//
// Note what init_escrow still does NOT do: it authorizes no release. Custody
// setup and release authorization remain separate (SPEC §5.1) — the escrow can
// be funded by anyone, and it is the release that has to prove itself.
//
// What it DOES do is record a choice: the payer names the one issuer whose
// attestation can ever release this escrow, and the program stores it immutably.
// At release that pin is ANDed with the allowlist — never substituted for it.
// Both must hold. The pin is what makes an escrow safe against an admin key
// stolen AFTER the deposit, because the pin predates the theft and no admin
// instruction can change it; the allowlist is what keeps revocation immediate
// for escrows already in flight. Drop either and one of those two properties
// goes with it.
//
// init_escrow also refuses to pin an issuer that is not currently allowlisted.
// That is a fail-fast convenience, not a security boundary: it saves the payer
// from funding an escrow that could only ever expire.
func (e *env) deposit(toStr string, amount uint64, resource string, pinIssuer [32]byte) {
	mint := solana.PublicKeyFromBytes(settle.USDCDevnetMint[:])

	recipient := e.payerPub
	if toStr != "" {
		var err error
		recipient, err = solana.PublicKeyFromBase58(toStr)
		if err != nil {
			log.Fatalf("bad -to pubkey: %v", err)
		}
	}

	payerATA, _, err := solana.FindAssociatedTokenAddress(e.payerPub, mint)
	if err != nil {
		log.Fatalf("derive payer ATA: %v", err)
	}
	recipientATA, _, err := solana.FindAssociatedTokenAddress(recipient, mint)
	if err != nil {
		log.Fatalf("derive recipient ATA: %v", err)
	}

	// Pre-flight: a clear message beats an on-chain simulation dump.
	bal, err := e.cl.GetTokenAccountBalance(e.ctx, payerATA, rpc.CommitmentConfirmed)
	if err != nil {
		log.Fatalf("no devnet USDC token account for %s\n"+
			"  -> fund THIS exact address at https://faucet.circle.com (Solana devnet)\n"+
			"  (rpc: %v)", e.payerPub, err)
	}
	have, _ := strconv.ParseUint(bal.Value.Amount, 10, 64)
	if have < amount {
		log.Fatalf("insufficient devnet USDC in %s: have %d, need %d micro-USDC", payerATA, have, amount)
	}

	// A fresh single-use nonce. This is the token's jti: it is what ties one gate
	// decision to one escrow release, and it is what makes the binding unique to
	// this escrow instance (THREAT-MODEL T4).
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		log.Fatalf("nonce: %v", err)
	}

	al := gate.Allowlist{
		Schemes:  map[string]byte{"exact": 1},
		Networks: map[string]byte{"solana:devnet": 1},
	}
	pr := gate.PaymentRequirements{
		Scheme:            "exact",
		Network:           "solana:devnet",
		Asset:             gate.EncodeBase58(mint.Bytes()),
		PayTo:             gate.EncodeBase58(recipientATA.Bytes()),
		MaxAmountRequired: strconv.FormatUint(amount, 10),
		Resource:          resource,
	}
	tok := gate.Token{Nonce: nonce, Expiry: time.Now().Add(2 * time.Minute)}

	dec := gate.Evaluate(al, pr, tok, ceilingPolicy{ceiling: 10_000_000, asset: pr.Asset},
		gate.NewMemSpendLog(), time.Now())
	fmt.Printf("gate:         %s (%s)\n", dec.Class, dec.Reason)
	if dec.Class != gate.Allow {
		log.Fatalf("gate denied — no escrow will be opened")
	}

	var payerKey, recipientKey [32]byte
	copy(payerKey[:], e.payerPub.Bytes())
	copy(recipientKey[:], recipient.Bytes())

	p, err := escrow.FromGate(al, pr, payerKey, recipientKey, nonce)
	if err != nil {
		log.Fatalf("FromGate: %v", err)
	}

	addrs := e.derive(p)
	fmt.Printf("gate binding:  %x\n", dec.Binding)
	fmt.Printf("escrow binding:%x\n", p.Binding)
	fmt.Printf("  (distinct domain tags — neither can stand in for the other)\n\n")
	fmt.Printf("recipient:    %s\n", recipient)
	fmt.Printf("recipient ATA:%s\n", recipientATA)
	fmt.Printf("pinned issuer:%s\n", solana.PublicKeyFromBytes(pinIssuer[:]))
	fmt.Printf("  (the ONLY key whose attestation can release this escrow — immutable from here,\n")
	fmt.Printf("   and checked IN ADDITION to the allowlist, never instead of it)\n")
	fmt.Printf("config PDA:   %s (read-only: init_escrow checks the pin is allowlisted, and asserts nothing about release)\n", addrs.config)
	fmt.Printf("escrow PDA:   %s\n", addrs.escrow)
	fmt.Printf("vault PDA:    %s\n", addrs.vault)
	fmt.Printf("spent marker: %s\n", addrs.spent)

	var ixs []solana.Instruction
	if recipient != e.payerPub {
		ixs = append(ixs, createATAIdempotent(e.payerPub, recipientATA, recipient, mint))
	}
	// Account order is the InitEscrow struct's field order: payer, config,
	// recipient, mint, escrow, vault, payer_ata, token, system, rent. `config` is
	// second and read-only; it was absent before the pin existed.
	ixs = append(ixs, rawInstruction{
		prog: e.program,
		metas: []*solana.AccountMeta{
			{PublicKey: e.payerPub, IsSigner: true, IsWritable: true},
			{PublicKey: addrs.config},
			{PublicKey: recipient},
			{PublicKey: mint},
			{PublicKey: addrs.escrow, IsWritable: true},
			{PublicKey: addrs.vault, IsWritable: true},
			{PublicKey: payerATA, IsWritable: true},
			{PublicKey: solana.TokenProgramID},
			{PublicKey: solana.SystemProgramID},
			{PublicKey: rentSysvarID},
		},
		data: escrow.InitEscrowData(p.Amount, p.ResourceID, p.Nonce, pinIssuer),
	})

	fmt.Printf("\ninit_escrow — moving %d micro-USDC into the vault\n", amount)
	sig := e.send("init_escrow", ixs, false)
	fmt.Printf("  tx: %s\n", explorer(sig))

	e.saveTicket(ticket{
		Nonce:        hex.EncodeToString(nonce[:]),
		Resource:     resource,
		Amount:       amount,
		Mint:         mint.String(),
		Payer:        e.payerPub.String(),
		Recipient:    recipient.String(),
		RecipientATA: recipientATA.String(),
		PinnedIssuer: solana.PublicKeyFromBytes(pinIssuer[:]).String(),
		Binding:      hex.EncodeToString(p.Binding[:]),
		Escrow:       addrs.escrow.String(),
		DepositSig:   sig.String(),
	})
	fmt.Printf("  wrote %s (public parameters only — no keys)\n", e.ticket)
}

// ────────────────────────────── release ─────────────────────────────────────

type intent int

const (
	allow intent = iota
	denyBinding
	denyIssuer
	denyUnpinned
)

func (i intent) String() string {
	switch i {
	case denyBinding:
		return "deny-binding"
	case denyIssuer:
		return "deny-issuer"
	case denyUnpinned:
		return "deny-unpinned (COMPROMISED ADMIN)"
	default:
		return "release"
	}
}

// release builds the two-instruction transaction the whole design rests on:
//
//	ix[0]  Ed25519SigVerify111111111111111111111111111  — the native precompile
//	ix[1]  spt_x402_escrow::release_with_proof          — no arguments at all
//
// The program verifies no signature itself. It reads the Instructions sysvar,
// finds the precompile instruction, and extracts the (pubkey, message) pair the
// runtime has ALREADY verified — refusing any instruction whose offsets point
// outside itself (THREAT-MODEL T2). release_with_proof takes no arguments
// because there is nothing for a caller to vary: every input is either on chain
// already (the escrow's stored binding, the allowlist, the validator clock) or
// comes from that precompile result.
//
// intent selects which of three attestations is signed. All three are signed for
// real by a real key, so all three transactions are accepted by the validator
// and land on chain; only the program's own checks separate them.
func (e *env) release(issuer ed25519.PrivateKey, what intent) {
	t := e.loadTicket()

	var binding [32]byte
	mustHex(binding[:], t.Binding, "ticket binding")
	escrowKey := solana.MustPublicKeyFromBase58(t.Escrow)
	recipientATA := solana.MustPublicKeyFromBase58(t.RecipientATA)
	payerRefund := solana.MustPublicKeyFromBase58(t.Payer)

	cfg := e.pda("config", [][]byte{[]byte(escrow.SeedConfig)}, func() ([32]byte, uint8, error) {
		return escrow.ConfigPDA(e.progID)
	})
	vault := e.pda("vault", [][]byte{[]byte(escrow.SeedVault), escrowKey.Bytes()}, func() ([32]byte, uint8, error) {
		return escrow.VaultPDA([32]byte(escrowKey), e.progID)
	})
	spent := e.pda("spent", [][]byte{[]byte(escrow.SeedSpent), binding[:]}, func() ([32]byte, uint8, error) {
		return escrow.SpentPDA(binding, e.progID)
	})

	// The attestation. iat is stamped now, immediately before sending, because
	// the program will only accept it for MAX_TOKEN_AGE_SECS (120s).
	signedBinding := binding
	if what == denyBinding {
		// A binding for a payment that is not this escrow. One flipped bit is
		// enough: the compare is constant-time equality over all 32 bytes.
		signedBinding[0] ^= 0x01
	}
	att := escrow.Attestation{Binding: signedBinding, IAT: time.Now().Unix()}
	msg, sig, err := att.Sign(issuer)
	if err != nil {
		log.Fatalf("sign attestation: %v", err)
	}
	issuerPub := issuer.Public().(ed25519.PublicKey)

	precompile, err := escrow.BuildEd25519Instruction(issuerPub, sig, msg)
	if err != nil {
		log.Fatalf("build ed25519 instruction: %v", err)
	}
	// Read our own encoding back with the same checks the program applies, before
	// spending a devnet round trip on it.
	if gotPub, gotMsg, err := escrow.ParseEd25519Instruction(precompile); err != nil {
		log.Fatalf("self-check: our own precompile instruction does not parse: %v", err)
	} else if !ed25519.Verify(ed25519.PublicKey(gotPub[:]), gotMsg, sig) {
		log.Fatalf("self-check: signature does not verify over the extracted message")
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 72))
	fmt.Printf("%s\n", what)
	fmt.Printf("  escrow:        %s\n", escrowKey)
	fmt.Printf("  escrow binding:%x\n", binding)
	fmt.Printf("  signed binding:%x\n", signedBinding)
	fmt.Printf("  signing issuer:%s\n", solana.PublicKeyFromBytes(issuerPub))
	if t.PinnedIssuer != "" {
		fmt.Printf("  pinned issuer: %s\n", t.PinnedIssuer)
	}
	fmt.Printf("  attestation:   %d bytes, iat=%d, valid until %s\n",
		len(msg), att.IAT, escrow.FreshnessDeadline(att.IAT).Format(time.RFC3339))

	switch what {
	case denyBinding:
		fmt.Printf("  expecting:     REVERT %d BindingMismatch — the signature is genuine,\n", errBindingMismatch)
		fmt.Printf("                 the issuer is authorized, but this attestation is not for THIS escrow\n")
	case denyIssuer:
		fmt.Printf("  expecting:     REVERT %d IssuerNotAuthorized — the signature is genuine\n", errIssuerNotAuthorized)
		fmt.Printf("                 and the binding is correct, but this key is not on the allowlist\n")
	case denyUnpinned:
		fmt.Printf("  expecting:     REVERT %d IssuerNotPinned — the signature is genuine, the binding\n", errIssuerNotPinned)
		fmt.Printf("                 is correct, AND the admin has put this key on the allowlist. It still\n")
		fmt.Printf("                 fails, because it is not the issuer the payer pinned at deposit.\n")
	default:
		fmt.Printf("  expecting:     RELEASE — funds move vault -> recipient, escrow and vault close\n")
	}

	ixs := []solana.Instruction{
		rawInstruction{prog: solana.MustPublicKeyFromBase58(escrow.Ed25519ProgramIDBase58), data: precompile},
		rawInstruction{
			prog: e.program,
			metas: []*solana.AccountMeta{
				{PublicKey: cfg},
				{PublicKey: escrowKey, IsWritable: true},
				{PublicKey: vault, IsWritable: true},
				{PublicKey: recipientATA, IsWritable: true},
				{PublicKey: payerRefund, IsWritable: true},
				{PublicKey: solana.MustPublicKeyFromBase58(escrow.InstructionsSysvarBase58)},
				{PublicKey: e.payerPub, IsSigner: true, IsWritable: true},
				{PublicKey: spent, IsWritable: true},
				{PublicKey: solana.TokenProgramID},
				{PublicKey: solana.SystemProgramID},
			},
			data: escrow.ReleaseWithProofData(),
		},
	}

	txSig := e.send(what.String(), ixs, what != allow)
	fmt.Printf("  tx: %s\n", explorer(txSig))

	code, logs := e.programError(txSig)
	for _, l := range logs {
		if strings.Contains(l, "Error Code") || strings.Contains(l, "VIOLATION") || strings.Contains(l, "UNAVAILABLE") {
			fmt.Printf("  log: %s\n", strings.TrimSpace(l))
		}
	}

	switch what {
	case denyBinding:
		requireCode(code, errBindingMismatch)
	case denyIssuer:
		requireCode(code, errIssuerNotAuthorized)
	case denyUnpinned:
		requireCode(code, errIssuerNotPinned)
	default:
		if code != 0 {
			log.Fatalf("release FAILED with program error %d — see the logs above", code)
		}
		fmt.Printf("  released: vault -> %s; escrow and vault closed, rent returned to the payer\n", recipientATA)
		fmt.Printf("  spent marker %s now exists permanently — this binding can never be released again\n", spent)
	}
}

// denyCompromisedAdmin stages THREAT-MODEL T9 on chain, with the attacker given
// everything the admin role can give them.
//
// The scenario is not "someone tries an unauthorized key". It is: the admin key
// has been stolen, and the attacker uses it exactly as the legitimate admin
// would. They mint a fresh Ed25519 key, they add it to the allowlist with a real
// add_issuer transaction that SUCCEEDS and lands on chain, and then they sign a
// correctly-bound, perfectly fresh attestation over an escrow that already
// exists. Every check the allowlist can perform now passes.
//
// The release still fails, with 6108 IssuerNotPinned, because the payer named
// their issuer at deposit and no admin instruction can reach that field. The
// pin predates the compromise. That is the whole property: a fully compromised
// admin is a denial-of-service role, and the worst outcome it can force is that
// payers get refunded at expiry.
//
// The rogue issuer is removed afterwards so the allowlist is left as it was
// found — the demo is meant to be repeatable, and MAX_ISSUERS is 16.
func (e *env) denyCompromisedAdmin() {
	rogue := rogueIssuer()
	var rogueKey [32]byte
	copy(rogueKey[:], rogue.Public().(ed25519.PublicKey))

	cfg := e.pda("config", [][]byte{[]byte(escrow.SeedConfig)}, func() ([32]byte, uint8, error) {
		return escrow.ConfigPDA(e.progID)
	})

	fmt.Printf("\n%s\n", strings.Repeat("─", 72))
	fmt.Printf("simulating a COMPROMISED ADMIN KEY (THREAT-MODEL T9)\n")
	fmt.Printf("  the attacker holds %s\n", e.adminPub)
	fmt.Printf("  rogue issuer:      %s\n", solana.PublicKeyFromBytes(rogueKey[:]))
	fmt.Printf("\nadd_issuer(rogue) — this SUCCEEDS. The admin really can do this.\n")

	sig := e.send("add_issuer(rogue)", []solana.Instruction{rawInstruction{
		prog: e.program,
		metas: []*solana.AccountMeta{
			{PublicKey: cfg, IsWritable: true},
			{PublicKey: e.adminPub, IsSigner: true},
		},
		data: escrow.AddIssuerData(rogueKey),
	}}, false)
	fmt.Printf("  tx: %s\n", explorer(sig))
	fmt.Printf("  the rogue key is now a fully authorized issuer. It changes nothing.\n")

	e.release(rogue, denyUnpinned)

	fmt.Printf("\nremove_issuer(rogue) — restoring the allowlist\n")
	sig = e.send("remove_issuer(rogue)", []solana.Instruction{rawInstruction{
		prog: e.program,
		metas: []*solana.AccountMeta{
			{PublicKey: cfg, IsWritable: true},
			{PublicKey: e.adminPub, IsSigner: true},
		},
		data: escrow.RemoveIssuerData(rogueKey),
	}}, false)
	fmt.Printf("  tx: %s\n", explorer(sig))
}

func requireCode(got, want int) {
	if got != want {
		log.Fatalf("expected program error %d, got %d — the demo did not prove what it claims", want, got)
	}
	fmt.Printf("  DENIED on chain with %d, as required. No funds moved.\n", got)
}

// ────────────────────────────── refund ──────────────────────────────────────

// refund is the liveness half of the design. If no valid attestation ever
// arrives, the funds are not stranded: after MAX_ESCROW_SECS anyone may call
// refund_expired, and the only destination it can pay is the stored payer.
func (e *env) refund() {
	t := e.loadTicket()
	escrowKey := solana.MustPublicKeyFromBase58(t.Escrow)
	payerWallet := solana.MustPublicKeyFromBase58(t.Payer)
	mint := solana.MustPublicKeyFromBase58(t.Mint)

	payerATA, _, err := solana.FindAssociatedTokenAddress(payerWallet, mint)
	if err != nil {
		log.Fatalf("derive payer ATA: %v", err)
	}
	vault := e.pda("vault", [][]byte{[]byte(escrow.SeedVault), escrowKey.Bytes()}, func() ([32]byte, uint8, error) {
		return escrow.VaultPDA([32]byte(escrowKey), e.progID)
	})

	fmt.Printf("refund_expired — returning the escrow to %s\n", payerWallet)
	ix := rawInstruction{
		prog: e.program,
		metas: []*solana.AccountMeta{
			{PublicKey: escrowKey, IsWritable: true},
			{PublicKey: vault, IsWritable: true},
			{PublicKey: payerATA, IsWritable: true},
			{PublicKey: payerWallet, IsWritable: true},
			{PublicKey: solana.TokenProgramID},
		},
		data: escrow.RefundExpiredData(),
	}
	sig := e.send("refund_expired", []solana.Instruction{ix}, false)
	fmt.Printf("  tx: %s\n", explorer(sig))
}

// ─────────────────────────── PDA cross-checking ─────────────────────────────

type addresses struct {
	config, escrow, vault, spent solana.PublicKey
}

// pda derives one address twice — once with this repository's own dependency-free
// implementation, once with the SDK's — and refuses to continue if they differ.
//
// The escrow package derives PDAs itself so the whole release path can be built
// and asserted offline, which is what makes the differential tests possible. The
// cost of that choice is a second implementation of a security-critical
// derivation, and the mitigation is this function: a divergence aborts before a
// transaction is signed, rather than paying a fee to discover it.
func (e *env) pda(label string, seeds [][]byte, mine func() ([32]byte, uint8, error)) solana.PublicKey {
	ours, _, err := mine()
	if err != nil {
		log.Fatalf("derive %s PDA (escrow pkg): %v", label, err)
	}
	theirs, _, err := solana.FindProgramAddress(seeds, e.program)
	if err != nil {
		log.Fatalf("derive %s PDA (solana-go): %v", label, err)
	}
	if [32]byte(theirs) != ours {
		log.Fatalf("PDA MISMATCH for %q — REFUSING TO SEND\n"+
			"  escrow package: %s\n"+
			"  solana-go:      %s\n"+
			"  one of the two derivations is wrong; do not send a transaction until this is resolved",
			label, solana.PublicKeyFromBytes(ours[:]), theirs)
	}
	return theirs
}

func (e *env) derive(p escrow.Params) addresses {
	a, err := p.Derive(e.progID)
	if err != nil {
		log.Fatalf("derive escrow addresses: %v", err)
	}
	escrowKey := e.pda("escrow",
		[][]byte{[]byte(escrow.SeedEscrow), p.Payer[:], p.Recipient[:], p.Binding[:]},
		func() ([32]byte, uint8, error) { return a.Escrow, a.EscrowBump, nil })
	return addresses{
		config: e.pda("config", [][]byte{[]byte(escrow.SeedConfig)},
			func() ([32]byte, uint8, error) { return a.Config, a.ConfigBump, nil }),
		escrow: escrowKey,
		vault: e.pda("vault", [][]byte{[]byte(escrow.SeedVault), escrowKey.Bytes()},
			func() ([32]byte, uint8, error) { return a.Vault, a.VaultBump, nil }),
		spent: e.pda("spent", [][]byte{[]byte(escrow.SeedSpent), p.Binding[:]},
			func() ([32]byte, uint8, error) { return a.Spent, a.SpentBump, nil }),
	}
}

// ──────────────────────────── send / confirm ────────────────────────────────

// send signs and submits. expectFailure controls preflight: a transaction we
// intend to fail must skip simulation, or the RPC node rejects it locally and it
// never lands — leaving no on-chain evidence of the DENY.
func (e *env) send(label string, ixs []solana.Instruction, expectFailure bool) solana.Signature {
	recent, err := e.cl.GetLatestBlockhash(e.ctx, rpc.CommitmentFinalized)
	if err != nil {
		log.Fatalf("%s: get blockhash: %v", label, err)
	}
	tx, err := solana.NewTransaction(ixs, recent.Value.Blockhash, solana.TransactionPayer(e.payerPub))
	if err != nil {
		log.Fatalf("%s: build tx: %v", label, err)
	}
	// Two possible signers, and they are deliberately different keys. The payer
	// pays every fee; the admin signs only add_issuer/remove_issuer and never
	// needs SOL of its own.
	if _, err := tx.Sign(func(k solana.PublicKey) *solana.PrivateKey {
		switch {
		case k.Equals(e.payerPub):
			return &e.payer
		case e.admin != nil && k.Equals(e.adminPub):
			return &e.admin
		}
		return nil
	}); err != nil {
		log.Fatalf("%s: sign: %v", label, err)
	}

	sig, err := e.cl.SendTransactionWithOpts(e.ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       expectFailure,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		log.Fatalf("%s: send: %v", label, err)
	}

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		res, err := e.cl.GetSignatureStatuses(e.ctx, true, sig)
		if err == nil && res != nil && len(res.Value) > 0 && res.Value[0] != nil {
			st := res.Value[0]
			if st.ConfirmationStatus == rpc.ConfirmationStatusConfirmed ||
				st.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
				if st.Err != nil && !expectFailure {
					log.Fatalf("%s: transaction landed but failed: %v", label, st.Err)
				}
				if st.Err == nil && expectFailure {
					log.Fatalf("%s: transaction SUCCEEDED but was supposed to be denied — "+
						"the enforcement property does not hold", label)
				}
				return sig
			}
		}
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("%s: timed out waiting for %s to confirm", label, sig)
	return sig
}

// programError extracts the Anchor error number from a landed transaction's
// logs. It is deliberately tolerant: if the RPC shape is not what we expect the
// demo still prints its explorer link, it just cannot assert the exact code.
func (e *env) programError(sig solana.Signature) (int, []string) {
	res, err := e.cl.GetTransaction(e.ctx, sig, &rpc.GetTransactionOpts{
		Encoding:   solana.EncodingBase64,
		Commitment: rpc.CommitmentConfirmed,
	})
	if err != nil || res == nil || res.Meta == nil {
		return 0, nil
	}
	logs := res.Meta.LogMessages
	for _, l := range logs {
		if i := strings.Index(l, "Error Number: "); i >= 0 {
			rest := l[i+len("Error Number: "):]
			end := strings.IndexAny(rest, ". ")
			if end < 0 {
				end = len(rest)
			}
			if n, err := strconv.Atoi(strings.TrimSpace(rest[:end])); err == nil {
				return n, logs
			}
		}
	}
	if res.Meta.Err != nil {
		// Fall back to the structured error, e.g. {"InstructionError":[1,{"Custom":6105}]}.
		if b, err := json.Marshal(res.Meta.Err); err == nil {
			if i := strings.Index(string(b), `"Custom":`); i >= 0 {
				rest := string(b)[i+len(`"Custom":`):]
				end := strings.IndexAny(rest, "}], ")
				if end > 0 {
					if n, err := strconv.Atoi(rest[:end]); err == nil {
						return n, logs
					}
				}
			}
		}
	}
	return 0, logs
}

func explorer(sig solana.Signature) string {
	return "https://explorer.solana.com/tx/" + sig.String() + "?cluster=devnet"
}

// ──────────────────────────────── keys ──────────────────────────────────────

// keyRole describes one of the two on-disk secrets this command loads, so the
// generate/load helpers can give role-accurate errors instead of always saying
// "issuer". They are the same file format (Solana keygen json) and the same
// handling rules; only the consequences of losing one differ.
type keyRole struct {
	name    string
	genFlag string
	// why losing or replacing this key matters, printed on generation.
	stakes string
}

var (
	roleIssuer = keyRole{
		name:    "issuer",
		genFlag: "-gen-issuer",
		stakes: "Replacing it silently would invalidate every escrow pinned to the old key —\n" +
			"those escrows can then only expire and refund. There is no way to get it back.",
	}
	roleAdmin = keyRole{
		name:    "admin",
		genFlag: "-gen-admin",
		stakes: "This key can add and remove issuers, and nothing else. It cannot cause a\n" +
			"release: every release is also gated on the issuer the payer pinned at deposit.\n" +
			"Losing it costs you allowlist control, not custody.",
	}
)

// generateKeyFile creates a key file if it does not already exist. It refuses to
// overwrite — a silent replacement of either key is unrecoverable in its own
// way. The private half is never printed.
func generateKeyFile(path string, role keyRole) {
	if _, err := os.Stat(path); err == nil {
		log.Fatalf("%s already exists — refusing to overwrite a %s key", path, role.name)
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("stat %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("generate: %v", err)
	}
	ints := make([]int, len(priv))
	for i, b := range priv {
		ints[i] = int(b)
	}
	buf, err := json.Marshal(ints)
	if err != nil {
		log.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
	fmt.Printf("wrote %s key to %s (mode 0600)\n", role.name, path)
	fmt.Printf("%s public key: %s\n", role.name, solana.PublicKeyFromBytes(pub))
	fmt.Printf("\n%s\n", role.stakes)
	fmt.Printf("\nrecord it on chain with:  go run -tags devnet ./cmd/escrowdevnet -mode setup\n")
	fmt.Printf("this file is a SECRET. It is not in the repository and must never be committed.\n")
}

func loadEd25519KeyFile(path string, role keyRole) ed25519.PrivateKey {
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("no %s key at %s — create one with:\n"+
				"  go run -tags devnet ./cmd/escrowdevnet %s", role.name, path, role.genFlag)
		}
		log.Fatalf("read %s key: %v", role.name, err)
	}
	var ints []int
	if err := json.Unmarshal(buf, &ints); err != nil {
		log.Fatalf("%s key %s is not a Solana keygen json array: %v", role.name, path, err)
	}
	if len(ints) != ed25519.PrivateKeySize {
		log.Fatalf("%s key %s is %d bytes, want %d", role.name, path, len(ints), ed25519.PrivateKeySize)
	}
	key := make(ed25519.PrivateKey, len(ints))
	for i, v := range ints {
		if v < 0 || v > 255 {
			log.Fatalf("%s key %s contains a non-byte value", role.name, path)
		}
		key[i] = byte(v)
	}
	return key
}

func loadIssuer(path string) ed25519.PrivateKey {
	return loadEd25519KeyFile(path, roleIssuer)
}

func loadIssuerPub(path string) ed25519.PublicKey {
	return loadIssuer(path).Public().(ed25519.PublicKey)
}

// rogueIssuer mints a throwaway key for the deny-issuer demo. It is generated in
// memory, used for exactly one transaction, and never written anywhere — it is
// standing in for an attacker who holds a perfectly valid Ed25519 key that the
// allowlist has simply never heard of.
func rogueIssuer() ed25519.PrivateKey {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("generate rogue issuer: %v", err)
	}
	return priv
}

// ─────────────────────────────── ticket ─────────────────────────────────────

// ticket carries the escrow's PUBLIC parameters from deposit to release. There
// is nothing secret in it: the nonce it stores has already been spent by the
// gate and committed to on chain inside the binding, and every other field is
// visible in the transaction itself. It exists so the runsheet is two commands
// instead of a hex-copying exercise.
type ticket struct {
	Nonce        string `json:"nonce"`
	Resource     string `json:"resource"`
	Amount       uint64 `json:"amount"`
	Mint         string `json:"mint"`
	Payer        string `json:"payer"`
	Recipient    string `json:"recipient"`
	RecipientATA string `json:"recipient_ata"`
	// The issuer the payer pinned at deposit. Public, and recorded here so the
	// release step can show what the escrow will actually accept rather than
	// assuming the signing key is the right one.
	PinnedIssuer string `json:"pinned_issuer"`
	Binding      string `json:"binding"`
	Escrow       string `json:"escrow"`
	DepositSig   string `json:"deposit_sig"`
}

func (e *env) saveTicket(t ticket) {
	buf, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		log.Fatalf("encode ticket: %v", err)
	}
	if err := os.WriteFile(e.ticket, append(buf, '\n'), 0o644); err != nil {
		log.Fatalf("write ticket: %v", err)
	}
}

func (e *env) loadTicket() ticket {
	buf, err := os.ReadFile(e.ticket)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("no escrow ticket at %s — run `-mode deposit` first", e.ticket)
		}
		log.Fatalf("read ticket: %v", err)
	}
	var t ticket
	if err := json.Unmarshal(buf, &t); err != nil {
		log.Fatalf("parse ticket %s: %v", e.ticket, err)
	}
	return t
}

func mustHex(dst []byte, s, what string) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(dst) {
		log.Fatalf("%s is not %d hex bytes", what, len(dst))
	}
	copy(dst, b)
}

// ─────────────────────────────── policy ─────────────────────────────────────

// ceilingPolicy is the pluggable decision point standing in for the published
// SPT-Txn verifier: pay up to a ceiling, in one asset. The gate itself adds no
// policy semantics — that separation is the point of the interface.
type ceilingPolicy struct {
	ceiling uint64
	asset   string
}

func (c ceilingPolicy) Verify(pr gate.PaymentRequirements, _ gate.Token) error {
	amt, err := strconv.ParseUint(pr.MaxAmountRequired, 10, 64)
	if err != nil {
		return errors.New("amount not a u64")
	}
	if amt > c.ceiling {
		return fmt.Errorf("amount %d over ceiling %d", amt, c.ceiling)
	}
	if pr.Asset != c.asset {
		return errors.New("asset not allowed")
	}
	return nil
}

// ───────────────────────────── instructions ─────────────────────────────────

// rawInstruction is a minimal solana.Instruction. Every instruction this command
// sends is built from bytes the escrow package produced and tested offline, so
// there is nothing for an instruction-builder helper to add — and one less SDK
// API whose name can drift between versions.
type rawInstruction struct {
	prog  solana.PublicKey
	metas []*solana.AccountMeta
	data  []byte
}

func (r rawInstruction) ProgramID() solana.PublicKey     { return r.prog }
func (r rawInstruction) Accounts() []*solana.AccountMeta { return r.metas }
func (r rawInstruction) Data() ([]byte, error)           { return r.data, nil }

// createATAIdempotent builds the Associated-Token-Account program's
// CreateIdempotent instruction (discriminator 0x01), so paying a brand-new
// recipient just works. A no-op if the account already exists.
func createATAIdempotent(payer, ata, wallet, mint solana.PublicKey) solana.Instruction {
	return rawInstruction{
		prog: ataProgramID,
		metas: []*solana.AccountMeta{
			{PublicKey: payer, IsSigner: true, IsWritable: true},
			{PublicKey: ata, IsWritable: true},
			{PublicKey: wallet},
			{PublicKey: mint},
			{PublicKey: solana.SystemProgramID},
			{PublicKey: solana.TokenProgramID},
		},
		data: []byte{1},
	}
}
