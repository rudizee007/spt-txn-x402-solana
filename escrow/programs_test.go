package escrow

import (
	"testing"

	"github.com/rudizee007/spt-txn-x402-solana/gate"
)

// TestWellKnownAddressesRoundTrip re-encodes every built-in address and compares
// it to the string it was decoded from, using the gate package's independent
// base58 encoder. This is what makes it safe to keep these as constants: a
// mistyped character cannot survive, because the round trip would not close.
func TestWellKnownAddressesRoundTrip(t *testing.T) {
	cases := map[string][32]byte{
		Ed25519ProgramIDBase58:         Ed25519ProgramID,
		InstructionsSysvarBase58:       InstructionsSysvarID,
		EscrowProgramIDBase58:          EscrowProgramID,
		TokenProgramIDBase58:           TokenProgramID,
		AssociatedTokenProgramIDBase58: AssociatedTokenProgramID,
		SystemProgramIDBase58:          SystemProgramID,
	}
	for s, key := range cases {
		if got := gate.EncodeBase58(key[:]); got != s {
			t.Errorf("round trip failed\n got: %s\nwant: %s", got, s)
		}
	}
}

// TestSystemProgramIsAllZero is a spot check with a value anyone can verify by
// eye, confirming the leading-'1' zero-byte handling is right.
func TestSystemProgramIsAllZero(t *testing.T) {
	var zero [32]byte
	if SystemProgramID != zero {
		t.Fatalf("system program id = %x, want all zeroes", SystemProgramID)
	}
}

func TestDecodePubkeyRejects(t *testing.T) {
	cases := map[string]string{
		"invalid character (0)": "0oooooooooooooooooooooooooooooooo",
		"invalid character (I)": "IIIIIIIIIIIIIIIIIIIIIIIIIIIIIIII",
		"too long":              "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		"too many leading ones": "111111111111111111111111111111111",
	}
	for name, s := range cases {
		if _, err := DecodePubkey(s); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

// TestDecodePubkeyMatchesGate cross-checks our decoder against the gate's for
// every built-in address, so the two packages cannot drift into disagreeing
// about what an address is.
func TestDecodePubkeyMatchesGate(t *testing.T) {
	for _, s := range []string{
		Ed25519ProgramIDBase58, InstructionsSysvarBase58, EscrowProgramIDBase58,
		TokenProgramIDBase58, AssociatedTokenProgramIDBase58, SystemProgramIDBase58,
	} {
		k, err := DecodePubkey(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if gate.EncodeBase58(k[:]) != s {
			t.Errorf("%s: decoder disagrees with gate.EncodeBase58", s)
		}
	}
}
