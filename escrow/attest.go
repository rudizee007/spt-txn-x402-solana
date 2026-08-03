package escrow

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"time"
)

const (
	// DomainTagAttest separates the on-chain attestation from the escrow
	// binding and from every other SPT-Txn construction (SPEC §6).
	DomainTagAttest = "spt-txn/x402-onchain-attest/v1"

	// AttestTagLen is the domain tag length in bytes (30).
	AttestTagLen = len(DomainTagAttest)

	// Offsets into the attestation message. The layout is fixed width so the
	// program can read it with slice arithmetic and no parser (SPEC §7.1).
	attestOffVersion = AttestTagLen
	attestOffBinding = AttestTagLen + 1
	attestOffIAT     = AttestTagLen + 1 + 32

	// AttestMsgLen is the exact length of a well-formed attestation (71). The
	// program rejects any other length outright, so this is a hard equality,
	// not a minimum.
	AttestMsgLen = AttestTagLen + 1 + 32 + 8

	// MaxTokenAgeSecs bounds how long an attestation remains releasable. This
	// is enforced on-chain against the validator clock, independently of any
	// off-chain token expiry, so a captured attestation cannot be replayed
	// indefinitely (SPEC §5.2 step 4, THREAT-MODEL T7).
	MaxTokenAgeSecs = 120

	// MaxClockSkewSecs bounds how far the issuer's clock may run ahead of the
	// validator's before a future-dated attestation is rejected.
	MaxClockSkewSecs = 30

	// MaxEscrowSecs is the escrow lifetime after which refund_expired becomes
	// permitted (SPEC §5.3). Mirrored here so the off-chain side can warn
	// before it builds a release that the program will reject.
	MaxEscrowSecs = 900
)

var (
	ErrAttestLength    = errors.New("escrow: attestation is not exactly 71 bytes")
	ErrAttestTag       = errors.New("escrow: attestation domain tag mismatch")
	ErrAttestVersion   = errors.New("escrow: unsupported attestation layout version")
	ErrAttestExpired   = errors.New("escrow: attestation older than the on-chain freshness bound")
	ErrAttestFuture    = errors.New("escrow: attestation is future-dated beyond allowed clock skew")
	ErrIssuerKeyLength = errors.New("escrow: issuer key is not an ed25519 key")
)

// Attestation is the compact, fixed-layout message an authorized issuer signs to
// release an escrow.
//
// This is deliberately NOT the SPT-Txn JWT. The JWT is a rich off-chain object
// with JSON, base64 and optional claims — three separate places two
// implementations can disagree. The on-chain path gets a parallel message with
// none of those properties. The two are linked by carrying the same Binding, and
// by the issuer signing both; the program never sees, parses, or trusts the JWT.
type Attestation struct {
	Binding [32]byte // must equal the escrow's stored binding
	IAT     int64    // issued-at, unix seconds
}

// Marshal encodes the attestation into the exact 71 bytes the program parses.
//
//	DOMAIN_TAG_ATTEST (30) || version u8 || binding[32] || iat i64 little-endian
func (a Attestation) Marshal() []byte {
	out := make([]byte, 0, AttestMsgLen)
	out = append(out, []byte(DomainTagAttest)...)
	out = append(out, LayoutVersion)
	out = append(out, a.Binding[:]...)
	var iat [8]byte
	binary.LittleEndian.PutUint64(iat[:], uint64(a.IAT))
	out = append(out, iat[:]...)
	return out
}

// ParseAttestation is the mirror of Marshal, applying the same checks the
// program applies in the same order: length, domain tag, version, then fields.
// It exists so the issuer can verify its own output round-trips, and so the
// devnet command can assert what it is about to send.
func ParseAttestation(msg []byte) (Attestation, error) {
	var a Attestation
	if len(msg) != AttestMsgLen {
		return a, ErrAttestLength
	}
	if string(msg[:AttestTagLen]) != DomainTagAttest {
		return a, ErrAttestTag
	}
	if msg[attestOffVersion] != LayoutVersion {
		return a, ErrAttestVersion
	}
	copy(a.Binding[:], msg[attestOffBinding:attestOffBinding+32])
	a.IAT = int64(binary.LittleEndian.Uint64(msg[attestOffIAT : attestOffIAT+8]))
	return a, nil
}

// Sign produces the issuer signature over the marshalled attestation. The
// returned message is what must be handed to BuildEd25519Instruction — signing
// and encoding must never re-derive the bytes independently, or they can drift.
func (a Attestation) Sign(issuer ed25519.PrivateKey) (msg, sig []byte, err error) {
	if len(issuer) != ed25519.PrivateKeySize {
		return nil, nil, ErrIssuerKeyLength
	}
	msg = a.Marshal()
	return msg, ed25519.Sign(issuer, msg), nil
}

// CheckFreshness applies the program's freshness rule (verify.rs::check_freshness)
// off-chain, so a release that the validator would reject is caught before it
// costs a transaction.
//
// The bound is two-sided on purpose. Rejecting old attestations limits the
// replay window; rejecting implausibly future-dated ones stops an issuer with a
// wrong or manipulated clock from minting attestations that stay valid for
// hours.
func CheckFreshness(iat, now int64) error {
	age := now - iat
	if age > MaxTokenAgeSecs {
		return ErrAttestExpired
	}
	if age < -MaxClockSkewSecs {
		return ErrAttestFuture
	}
	return nil
}

// FreshnessDeadline reports the last wall-clock instant at which an attestation
// issued at iat is still releasable. Useful for logging and for the runsheet:
// the release transaction must land, not merely be sent, before this time.
func FreshnessDeadline(iat int64) time.Time {
	return time.Unix(iat+MaxTokenAgeSecs, 0)
}
