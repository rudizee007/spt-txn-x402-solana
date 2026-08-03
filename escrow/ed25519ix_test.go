package escrow

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

// TestEd25519InstructionHeader pins the encoded header against the Rust test
// fixture verify.rs::good_ix, which builds the offsets as
// [48, 0xFFFF, 16, 0xFFFF, 112, msg_len, 0xFFFF].
//
// For a 71-byte attestation that is, byte for byte:
//
//	01 00        num_signatures = 1, padding
//	30 00 ff ff  sig_off = 48,  sig_ix  = u16::MAX
//	10 00 ff ff  pk_off  = 16,  pk_ix   = u16::MAX
//	70 00        msg_off = 112
//	47 00        msg_size = 71
//	ff ff        msg_ix  = u16::MAX
func TestEd25519InstructionHeader(t *testing.T) {
	msg := Attestation{Binding: pk(0x5A), IAT: 1_700_000_000}.Marshal()
	data, err := BuildEd25519Instruction(bytes.Repeat([]byte{0xAB}, 32), bytes.Repeat([]byte{0xCD}, 64), msg)
	if err != nil {
		t.Fatal(err)
	}

	const want = "01003000ffff1000ffff70004700ffff"
	if got := hex.EncodeToString(data[:16]); got != want {
		t.Fatalf("header mismatch\n got: %s\nwant: %s", got, want)
	}

	if len(data) != ed25519MsgOff+len(msg) {
		t.Fatalf("length = %d, want %d", len(data), ed25519MsgOff+len(msg))
	}
	if !bytes.Equal(data[16:48], bytes.Repeat([]byte{0xAB}, 32)) {
		t.Error("pubkey is not at offset 16")
	}
	if !bytes.Equal(data[48:112], bytes.Repeat([]byte{0xCD}, 64)) {
		t.Error("signature is not at offset 48")
	}
	if !bytes.Equal(data[112:], msg) {
		t.Error("message is not at offset 112")
	}
}

// TestEd25519RoundTrip runs the whole off-chain half end to end: sign an
// attestation, encode the precompile instruction, then read it back with the
// same checks the program applies, and confirm the signature verifies over
// exactly the message the program would extract.
func TestEd25519RoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	att := Attestation{Binding: pk(0x33), IAT: 1_700_000_000}

	msg, sig, err := att.Sign(priv)
	if err != nil {
		t.Fatal(err)
	}
	data, err := BuildEd25519Instruction(pub, sig, msg)
	if err != nil {
		t.Fatal(err)
	}

	gotPub, gotMsg, err := ParseEd25519Instruction(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !bytes.Equal(gotPub[:], pub) {
		t.Error("extracted pubkey differs from the signing key")
	}
	if !bytes.Equal(gotMsg, msg) {
		t.Error("extracted message differs from the signed message")
	}
	if !ed25519.Verify(ed25519.PublicKey(gotPub[:]), gotMsg, sig) {
		t.Fatal("signature does not verify over the extracted (pubkey, message)")
	}
	parsed, err := ParseAttestation(gotMsg)
	if err != nil || parsed != att {
		t.Fatalf("extracted message did not parse back to the attestation: %v %+v", err, parsed)
	}
}

// TestEd25519ParseRejects mirrors verify.rs::parse_rejects_bad_inputs, using the
// same byte offsets the Rust test pokes.
func TestEd25519ParseRejects(t *testing.T) {
	msg := Attestation{Binding: pk(0x5A), IAT: 1_700_000_000}.Marshal()
	good := func() []byte {
		d, err := BuildEd25519Instruction(bytes.Repeat([]byte{0xAB}, 32), bytes.Repeat([]byte{0xCD}, 64), msg)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}

	cases := map[string]func() []byte{
		"empty":             func() []byte { return nil },
		"truncated offsets": func() []byte { return []byte{1, 0, 0, 0} },
		"num_sigs = 2":      func() []byte { d := good(); d[0] = 2; return d },
		"num_sigs = 0":      func() []byte { d := good(); d[0] = 0; return d },
		"pk_ix not self":    func() []byte { d := good(); d[8], d[9] = 0, 0; return d },
		"msg_ix not self":   func() []byte { d := good(); d[14], d[15] = 0, 0; return d },
		"msg_off OOB":       func() []byte { d := good(); d[10], d[11] = 0xFF, 0xFF; return d },
		"msg_size not 71":   func() []byte { d := good(); d[12] = byte(AttestMsgLen - 1); return d },
	}
	for name, build := range cases {
		if _, _, err := ParseEd25519Instruction(build()); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

// TestEd25519BuildRejects covers the encoder's own input validation.
func TestEd25519BuildRejects(t *testing.T) {
	pubOK := bytes.Repeat([]byte{1}, 32)
	sigOK := bytes.Repeat([]byte{2}, 64)
	msg := Attestation{}.Marshal()

	if _, err := BuildEd25519Instruction(pubOK[:31], sigOK, msg); err != ErrPubkeyLength {
		t.Errorf("short pubkey: got %v", err)
	}
	if _, err := BuildEd25519Instruction(pubOK, sigOK[:63], msg); err != ErrSignatureLength {
		t.Errorf("short signature: got %v", err)
	}
	if _, err := BuildEd25519Instruction(pubOK, sigOK, make([]byte, 0x10000)); err != ErrMessageTooLong {
		t.Errorf("oversized message: got %v", err)
	}
}

// TestSelfSentinelIsEnforced states the T2 property directly: the program will
// only accept an attestation whose pubkey and message live in the precompile
// instruction itself. Offsets pointing at another instruction must be rejected,
// even when everything else is well formed.
func TestSelfSentinelIsEnforced(t *testing.T) {
	msg := Attestation{Binding: pk(1), IAT: 0}.Marshal()
	data, err := BuildEd25519Instruction(bytes.Repeat([]byte{0xAB}, 32), bytes.Repeat([]byte{0xCD}, 64), msg)
	if err != nil {
		t.Fatal(err)
	}
	// Point the message at instruction index 0 instead of "self".
	data[14], data[15] = 0x00, 0x00
	if _, _, err := ParseEd25519Instruction(data); err != ErrIxNotSelf {
		t.Fatalf("expected ErrIxNotSelf, got %v", err)
	}
}
