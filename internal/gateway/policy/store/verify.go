package store

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"fmt"

	// "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/proto"
)

type RootKey struct {
	PublicKey ed25519.PublicKey
}

func RootPublicKey(encoded string) (*RootKey, error) {
	der, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		der, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, fmt.Errorf("decode ASHRIX_POLICY_PUBLIC_KEY: %w", err)
	}
	return ParseRootPublicKey(der)
}

func ParseRootPublicKey(der []byte) (*RootKey, error) {
	if block, _ := pem.Decode(der); block != nil {
		if block.Type != "PUBLIC KEY" {
			return nil, fmt.Errorf("policy verification key PEM must be PUBLIC KEY")
		}
		der = block.Bytes
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse policy verification public key: %w", err)
	}
	key, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("policy verification key is not ed25519")
	}
	return &RootKey{
		PublicKey: key,
	}, nil
}

// canonicalRuleHash creates a deterministic hash of a PolicyRule.
//
// WARNING: proto.MarshalOptions{Deterministic:true} is only deterministic
// within the same version of google.golang.org/protobuf. It is NOT guaranteed
// across languages or library versions. For maximum robustness, replace this
// with explicit field-by-field serialization (e.g. write strings/ints in fixed
// order to a sha256.Writer).
func canonicalRuleHash(rule *pb.PolicyRule) []byte {
	if rule == nil {
		// Sentinel for DELETE operations where rule may be omitted
		h := sha256.Sum256([]byte{0x00})
		return h[:]
	}
	h := sha256.New()
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(rule)
	if err != nil {
		panic(fmt.Sprintf("marshal rule: %v", err))
	}
	h.Write(b)
	return h.Sum(nil)
}

// RecordSigningPayload returns the exact bytes that must be signed for a PolicyRecord.
// The record.signature field itself is EXCLUDED.
func RecordSigningPayload(record *pb.PolicyRecord) []byte {
	h := sha256.New()

	// Fixed-order envelope fields. BigEndian is explicit and stable.
	binary.Write(h, binary.BigEndian, record.Rule.Sequence)
	binary.Write(h, binary.BigEndian, int32(record.Operation))
	binary.Write(h, binary.BigEndian, record.Timestamp)

	// The actual policy payload
	h.Write(canonicalRuleHash(record.Rule))

	return h.Sum(nil)
}

// BundleSigningPayload returns the exact bytes that must be signed for a PolicyBundle.
// The bundle.signature field itself is EXCLUDED.
func BundleSigningPayload(bundle *pb.PolicyBundle) []byte {
	h := sha256.New()

	binary.Write(h, binary.BigEndian, bundle.LastSequence)
	binary.Write(h, binary.BigEndian, bundle.IssuedAt)
	binary.Write(h, binary.BigEndian, int64(len(bundle.Records)))

	for _, record := range bundle.Records {
		// Bind the bundle to the exact signed records. Using the record
		// signatures is sufficient because each signature is already a
		// cryptographic commitment to that record's content.
		h.Write(record.Signature)
	}

	return h.Sum(nil)
}

// VerifyBundle verifies the top-level bundle signature AND every record inside.
// Call this once when the gateway RECEIVES the bundle.
func (v *RootKey) VerifyBundle(bundle *pb.PolicyBundle) error {
	payload := BundleSigningPayload(bundle)
	if !ed25519.Verify(v.PublicKey, payload, bundle.Signature) {
		return fmt.Errorf("bundle signature invalid")
	}

	// Verify every record signature inside the bundle
	for _, record := range bundle.Records {
		if err := v.VerifyRecord(record); err != nil {
			return fmt.Errorf("bundle contains invalid record: %w", err)
		}
	}
	return nil
}

// VerifyRecord verifies the signature on a single PolicyRecord.
// Call this at STARTUP when loading from bbolt, and when applying a record.
func (v *RootKey) VerifyRecord(record *pb.PolicyRecord) error {
	payload := RecordSigningPayload(record)
	if !ed25519.Verify(v.PublicKey, payload, record.Signature) {
		pid := "<nil>"
		if record.Rule != nil {
			pid = record.Rule.PolicyId
		}
		return fmt.Errorf("record seq=%d policy=%s signature invalid", record.Rule.Sequence, pid)
	}
	return nil
}
