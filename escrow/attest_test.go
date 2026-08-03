package escrow

import (
	"crypto/ed25519"
	"encoding/binary"
	"testing"
)

// TestAttestMsgLen pins the exact length the program requires. It is an equality
// check on-chain, not a minimum, so drift here is an immediate hard denial.
func TestAttestMsgLen(t *testing.T) {
	if AttestMsgLen != 71 {
		t.Fatalf("AttestMsgLen = %d, want 71 (30 tag + 1 version + 32 binding + 8 iat)", AttestMsgLen)
	}
	a := Attestation{Binding: pk(0x5A), IAT: 1_700_000_000}
	if got := len(a.Marshal()); got != AttestMsgLen {
		t.Fatalf("marshalled length = %d, want %d", got, AttestMsgLen)
	}
}

// TestAttestLayout reproduces the Rust test fixture verify.rs::valid_msg byte for
// byte and asserts our encoder produces the identical bytes.
func TestAttestLayout(t *testing.T) {
	var want []byte
	want = append(want, []byte(DomainTagAttest)...)
	want = append(want, LayoutVersion)
	b := pk(0x5A)
	want = append(want, b[:]...)
	var iat [8]byte
	binary.LittleEndian.PutUint64(iat[:], uint64(int64(1_700_000_000)))
	want = append(want, iat[:]...)

	got := Attestation{Binding: b, IAT: 1_700_000_000}.Marshal()
	if string(got) != string(want) {
		t.Fatalf("attestation layout mismatch\n got: %x\nwant: %x", got, want)
	}
}

func TestAttestRoundTrip(t *testing.T) {
	in := Attestation{Binding: pk(0x9E), IAT: -1} // negative iat must survive as i64
	out, err := ParseAttestation(in.Marshal())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out != in {
		t.Fatalf("round trip lost data: got %+v, want %+v", out, in)
	}
}

// TestAttestParseRejects mirrors verify.rs::parse_rejects_bad_inputs for the
// message-level checks: wrong length, wrong tag, wrong version.
func TestAttestParseRejects(t *testing.T) {
	good := Attestation{Binding: pk(0x5A), IAT: 1_700_000_000}.Marshal()

	cases := map[string][]byte{
		"empty":     {},
		"truncated": good[:len(good)-1],
		"too long":  append(append([]byte{}, good...), 0x00),
	}
	badTag := append([]byte{}, good...)
	badTag[0] ^= 1
	cases["bad domain tag"] = badTag

	badVer := append([]byte{}, good...)
	badVer[AttestTagLen] = 0xFF
	cases["bad version"] = badVer

	for name, msg := range cases {
		if _, err := ParseAttestation(msg); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

// TestFreshnessBounds mirrors verify.rs::freshness_bounds exactly, including the
// inclusive/exclusive edges. An off-by-one here means we build releases the
// validator rejects (or, worse, believe a stale one is fine).
func TestFreshnessBounds(t *testing.T) {
	const iat int64 = 1000
	cases := []struct {
		now int64
		ok  bool
	}{
		{iat, true},
		{iat + MaxTokenAgeSecs, true},
		{iat + MaxTokenAgeSecs + 1, false},
		{iat - MaxClockSkewSecs, true},
		{iat - MaxClockSkewSecs - 1, false},
	}
	for _, c := range cases {
		err := CheckFreshness(iat, c.now)
		if (err == nil) != c.ok {
			t.Errorf("CheckFreshness(%d, %d) = %v, want ok=%v", iat, c.now, err, c.ok)
		}
	}
}

func TestFreshnessConstants(t *testing.T) {
	if MaxTokenAgeSecs != 120 || MaxClockSkewSecs != 30 || MaxEscrowSecs != 900 {
		t.Fatalf("freshness constants drifted from constants.rs: age=%d skew=%d escrow=%d",
			MaxTokenAgeSecs, MaxClockSkewSecs, MaxEscrowSecs)
	}
}

// TestSignVerify confirms the signature the runtime will check is over exactly
// the bytes we parse. Sign and Marshal must never derive the message
// independently.
func TestSignVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	att := Attestation{Binding: pk(0x77), IAT: 1_700_000_042}

	msg, sig, err := att.Sign(priv)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, msg, sig) {
		t.Fatal("signature does not verify over the marshalled attestation")
	}
	parsed, err := ParseAttestation(msg)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != att {
		t.Fatal("signed bytes do not parse back to the attestation that was signed")
	}

	// A one-bit change anywhere must break verification — this is what makes
	// the precompile a meaningful check rather than a formality.
	tampered := append([]byte{}, msg...)
	tampered[attestOffBinding] ^= 1
	if ed25519.Verify(pub, tampered, sig) {
		t.Fatal("signature verified over a tampered binding")
	}
}

func TestSignRejectsBadKey(t *testing.T) {
	if _, _, err := (Attestation{}).Sign(ed25519.PrivateKey{1, 2, 3}); err == nil {
		t.Fatal("expected an error for an undersized private key")
	}
}
