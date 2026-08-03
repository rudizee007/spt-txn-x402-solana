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
// Five modes, in the order the runsheet uses them:
//
//	setup         init_config (once per deployment) + add_issuer
//	deposit       gate ALLOW -> init_escrow, USDC moves payer -> vault
//	deny-binding  a VALIDLY SIGNED attestation over the wrong binding  -> 6105
//	deny-issuer   a VALIDLY SIGNED attestation from a rogue issuer     -> 6102
//	release       the real proof                                       -> funds released
//	refund        after expiry, funds return to the payer
//	all           setup (if needed), deposit, both denies, release
//
// Why the deny demos sign for real: tampering with the *signature* makes the
// native Ed25519 precompile fail during transaction verification, so the
// transaction is dropped by the validator and never lands. A dropped transaction
// is not evidence of anything — there is no explorer link and no program log. To
// produce an on-chain DENY the signature must be genuine and the failure must
// happen INSIDE the program, which is exactly what a wrong binding (6105
// BindingMismatch) and a non-allowlisted issuer (6102 IssuerNotAuthorized)
// produce. Both are sent with preflight skipped so the failure is recorded on
// chain rather than rejected by the RPC node's simulation.
//
// Keys. Two distinct secrets are involved and neither is ever printed, logged,
// put in an environment variable, or written into the repository:
//
//   - the payer/releaser wallet stays in the Solana CLI keypair file;
//   - the SPT-Txn issuer's Ed25519 key stays in its own file (default
//     ~/.config/spt-txn/issuer-devnet.json, created 0600 by -gen-issuer).
//
// The rogue key used by deny-issuer is generated in memory for one transaction
// and never touches disk.
//
//	go run -tags devnet ./cmd/escrowdevnet -gen-issuer      # once: create the issuer key
//	go run -tags devnet ./cmd/escrowdevnet -mode setup      # once per deployment
//	go run -tags devnet ./cmd/escrowdevnet -mode all        # the full demo
//
// Needs devnet SOL for fees/rent and devnet USDC (https://faucet.circle.com).
//
// Like cmd/paydevnet, this file could not be compiled in the authoring
// environment (no Go module proxy reachable there). Expect to `go mod download`
// first; if a solana-go API name has drifted in your version, it is a small fix.
// Nothing security-relevant lives in the SDK calls — every address, discriminator
// and byte layout comes from the escrow package, which is fully tested offline
// and cross-checked against the SDK before anything is sent.
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

func main() {
	var (
		keypairPath = flag.String("keypair", defaultKeypair(), "Solana CLI keypair json for the payer/releaser (devnet)")
		issuerPath  = flag.String("issuer", defaultIssuerKey(), "SPT-Txn issuer Ed25519 key file (Solana keygen json format)")
		genIssuer   = flag.Bool("gen-issuer", false, "create the issuer key file (0600) if it does not exist, print only its public key, and exit")
		mode        = flag.String("mode", "all", "setup | deposit | deny-binding | deny-issuer | release | refund | all")
		toStr       = flag.String("to", "", "recipient wallet pubkey (base58); default: yourself")
		amount      = flag.Uint64("amount", 100_000, "amount in micro-USDC (100000 = 0.10 USDC)")
		resource    = flag.String("resource", "https://api.example.com/v1/quote", "the x402 resource being paid for")
		ticketPath  = flag.String("ticket", "escrow-ticket.json", "where deposit records this escrow's public parameters for the later release")
		programStr  = flag.String("program", escrow.EscrowProgramIDBase58, "deployed spt_x402_escrow program id")
	)
	flag.Parse()
	ctx := context.Background()

	if *genIssuer {
		generateIssuer(*issuerPath)
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

	switch *mode {
	case "setup":
		env.setup(loadIssuerPub(*issuerPath))
	case "deposit":
		env.deposit(*toStr, *amount, *resource)
	case "deny-binding":
		env.release(loadIssuer(*issuerPath), denyBinding)
	case "deny-issuer":
		env.release(rogueIssuer(), denyIssuer)
	case "release":
		env.release(loadIssuer(*issuerPath), allow)
	case "refund":
		env.refund()
	case "all":
		issuer := loadIssuer(*issuerPath)
		env.setup(issuer.Public().(ed25519.PublicKey))
		env.deposit(*toStr, *amount, *resource)
		env.release(issuer, denyBinding)
		env.release(rogueIssuer(), denyIssuer)
		env.release(issuer, allow)
	default:
		log.Fatalf("unknown -mode %q", *mode)
	}
}

type env struct {
	ctx      context.Context
	cl       *rpc.Client
	payer    solana.PrivateKey
	payerPub solana.PublicKey
	program  solana.PublicKey
	progID   [32]byte
	ticket   string
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
func (e *env) setup(issuerPub ed25519.PublicKey) {
	var issuer [32]byte
	copy(issuer[:], issuerPub)

	cfg := e.pda("config", [][]byte{[]byte(escrow.SeedConfig)}, func() ([32]byte, uint8, error) {
		return escrow.ConfigPDA(e.progID)
	})

	fmt.Printf("config PDA:   %s\n", cfg)
	fmt.Printf("issuer:       %s\n", solana.PublicKeyFromBytes(issuer[:]))

	admins, issuers, exists := e.readConfig(cfg)
	if !exists {
		programData, _, err := solana.FindProgramAddress([][]byte{e.program.Bytes()}, bpfLoaderUpgradeableID)
		if err != nil {
			log.Fatalf("derive ProgramData: %v", err)
		}
		fmt.Printf("program data: %s\n", programData)
		fmt.Println("\ninit_config — creating the config with an EMPTY allowlist (deny-by-default)")

		ix := rawInstruction{
			prog: e.program,
			metas: []*solana.AccountMeta{
				{PublicKey: cfg, IsWritable: true},
				{PublicKey: e.payerPub, IsSigner: true, IsWritable: true},
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
			admins, len(issuers))
		for _, k := range issuers {
			if k == issuer {
				fmt.Println("issuer is already on the allowlist — nothing to do")
				return
			}
		}
	}

	fmt.Println("\nadd_issuer — authorizing the SPT-Txn issuer key")
	ix := rawInstruction{
		prog: e.program,
		metas: []*solana.AccountMeta{
			{PublicKey: cfg, IsWritable: true},
			{PublicKey: e.payerPub, IsSigner: true},
		},
		data: escrow.AddIssuerData(issuer),
	}
	sig := e.send("add_issuer", []solana.Instruction{ix}, false)
	fmt.Printf("  tx: %s\n", explorer(sig))
}

// readConfig fetches and decodes the Config account. The account discriminator
// is checked before any field is read, so a same-address account of a different
// type cannot be mistaken for a config.
func (e *env) readConfig(cfg solana.PublicKey) (admin solana.PublicKey, issuers [][32]byte, exists bool) {
	res, err := e.cl.GetAccountInfo(e.ctx, cfg)
	if err != nil || res == nil || res.Value == nil {
		return admin, nil, false
	}
	data := res.Value.Data.GetBinary()
	want := escrow.AccountDiscriminator("Config")
	if len(data) < 8+32+4+1 || string(data[:8]) != string(want[:]) {
		log.Fatalf("account %s is not a spt_x402_escrow Config (discriminator mismatch)", cfg)
	}
	admin = solana.PublicKeyFromBytes(data[8:40])
	n := int(uint32(data[40]) | uint32(data[41])<<8 | uint32(data[42])<<16 | uint32(data[43])<<24)
	if n < 0 || n > 16 || len(data) < 44+32*n {
		log.Fatalf("config account %s has an implausible issuer count (%d)", cfg, n)
	}
	for i := 0; i < n; i++ {
		var k [32]byte
		copy(k[:], data[44+32*i:])
		issuers = append(issuers, k)
	}
	return admin, issuers, true
}

// ────────────────────────────── deposit ─────────────────────────────────────

// deposit runs the whole authorization story for one payment and then locks the
// funds: the gate decides ALLOW on the x402 requirements, FromGate maps that
// decision onto escrow parameters (verifying, not trusting, that the recipient
// wallet really owns the requirement's payTo token account), and init_escrow
// moves the USDC into a vault owned by the escrow PDA.
//
// Note what init_escrow does NOT do: it asserts no authorization. Custody setup
// and authorization are deliberately separate (SPEC §5.1) — the escrow can be
// funded by anyone, and it is the release that has to prove itself.
func (e *env) deposit(toStr string, amount uint64, resource string) {
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
	fmt.Printf("config PDA:   %s (not touched by init_escrow — custody setup asserts no authorization)\n", addrs.config)
	fmt.Printf("escrow PDA:   %s\n", addrs.escrow)
	fmt.Printf("vault PDA:    %s\n", addrs.vault)
	fmt.Printf("spent marker: %s\n", addrs.spent)

	var ixs []solana.Instruction
	if recipient != e.payerPub {
		ixs = append(ixs, createATAIdempotent(e.payerPub, recipientATA, recipient, mint))
	}
	ixs = append(ixs, rawInstruction{
		prog: e.program,
		metas: []*solana.AccountMeta{
			{PublicKey: e.payerPub, IsSigner: true, IsWritable: true},
			{PublicKey: recipient},
			{PublicKey: mint},
			{PublicKey: addrs.escrow, IsWritable: true},
			{PublicKey: addrs.vault, IsWritable: true},
			{PublicKey: payerATA, IsWritable: true},
			{PublicKey: solana.TokenProgramID},
			{PublicKey: solana.SystemProgramID},
			{PublicKey: rentSysvarID},
		},
		data: escrow.InitEscrowData(p.Amount, p.ResourceID, p.Nonce),
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
)

func (i intent) String() string {
	switch i {
	case denyBinding:
		return "deny-binding"
	case denyIssuer:
		return "deny-issuer"
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
	fmt.Printf("  issuer:        %s\n", solana.PublicKeyFromBytes(issuerPub))
	fmt.Printf("  attestation:   %d bytes, iat=%d, valid until %s\n",
		len(msg), att.IAT, escrow.FreshnessDeadline(att.IAT).Format(time.RFC3339))

	switch what {
	case denyBinding:
		fmt.Printf("  expecting:     REVERT %d BindingMismatch — the signature is genuine,\n", errBindingMismatch)
		fmt.Printf("                 the issuer is authorized, but this attestation is not for THIS escrow\n")
	case denyIssuer:
		fmt.Printf("  expecting:     REVERT %d IssuerNotAuthorized — the signature is genuine\n", errIssuerNotAuthorized)
		fmt.Printf("                 and the binding is correct, but this key is not on the allowlist\n")
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
	default:
		if code != 0 {
			log.Fatalf("release FAILED with program error %d — see the logs above", code)
		}
		fmt.Printf("  released: vault -> %s; escrow and vault closed, rent returned to the payer\n", recipientATA)
		fmt.Printf("  spent marker %s now exists permanently — this binding can never be released again\n", spent)
	}
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
	if _, err := tx.Sign(func(k solana.PublicKey) *solana.PrivateKey {
		if k.Equals(e.payerPub) {
			return &e.payer
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

// generateIssuer creates the issuer key file if it does not already exist. It
// refuses to overwrite: silently replacing an issuer key would invalidate every
// escrow the old key was authorized for, and there is no way to get it back.
// The private half is never printed.
func generateIssuer(path string) {
	if _, err := os.Stat(path); err == nil {
		log.Fatalf("%s already exists — refusing to overwrite an issuer key", path)
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
	fmt.Printf("wrote issuer key to %s (mode 0600)\n", path)
	fmt.Printf("issuer public key: %s\n", solana.PublicKeyFromBytes(pub))
	fmt.Printf("\nauthorize it with:  go run -tags devnet ./cmd/escrowdevnet -mode setup\n")
	fmt.Printf("this file is a SECRET. It is not in the repository and must never be committed.\n")
}

func loadIssuer(path string) ed25519.PrivateKey {
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("no issuer key at %s — create one with:\n"+
				"  go run -tags devnet ./cmd/escrowdevnet -gen-issuer", path)
		}
		log.Fatalf("read issuer key: %v", err)
	}
	var ints []int
	if err := json.Unmarshal(buf, &ints); err != nil {
		log.Fatalf("issuer key %s is not a Solana keygen json array: %v", path, err)
	}
	if len(ints) != ed25519.PrivateKeySize {
		log.Fatalf("issuer key %s is %d bytes, want %d", path, len(ints), ed25519.PrivateKeySize)
	}
	key := make(ed25519.PrivateKey, len(ints))
	for i, v := range ints {
		if v < 0 || v > 255 {
			log.Fatalf("issuer key %s contains a non-byte value", path)
		}
		key[i] = byte(v)
	}
	return key
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
