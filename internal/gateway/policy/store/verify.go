package store

import (
	// "crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/binary"
	"encoding/pem"
	"fmt"

	// "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/proto"
)

//go:embed root.pub
var rootPEM []byte

type RootKey struct {
	PublicKey ed25519.PublicKey
}

func RootPublicKey() (*RootKey, error) {

	block, _ := pem.Decode(rootPEM)
	if block == nil {
		return nil, fmt.Errorf("invalid embedded root key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	
	key, ok := pub.(ed25519.PublicKey)

	//Debuging
	// pubPEM := pem.EncodeToMemory(&pem.Block{
	// 	Type:  "PUBLIC KEY",
	// 	Bytes: block.Bytes,
	// })

	// fmt.Println(string(pubPEM))

	if !ok {
		return nil, fmt.Errorf("embedded key is not ed25519")
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
	binary.Write(h, binary.BigEndian, record.Sequence)
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

	binary.Write(h, binary.BigEndian, bundle.Version)
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
		return fmt.Errorf("record seq=%d policy=%s signature invalid", record.Sequence, pid)
	}
	return nil
}