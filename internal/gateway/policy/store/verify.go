package store

import (
	"crypto/ed25519"
	"fmt"
	"os"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"

	"google.golang.org/protobuf/proto"
)

// KeyRing holds CP public keys. Supports rotation via KeyID.
type KeyRing struct {
	keys map[string]ed25519.PublicKey
}

func NewKeyRing(keys map[string]ed25519.PublicKey) *KeyRing {
	return &KeyRing{keys: keys}
}

// SignatureVerifier checks Ed25519 signatures from the CP.
type SignatureVerifier struct {
	ring *KeyRing
}

func NewSignatureVerifier() *SignatureVerifier {
	cpPubKey := ed25519.PublicKey(os.Getenv("ASHRIX_CP_PUBKEY")) // hex-decode in production

	ring := NewKeyRing(map[string]ed25519.PublicKey{
		"cp-v1": cpPubKey,
	})

	return &SignatureVerifier{ring: ring}
}

func (v *SignatureVerifier) getKey(keyID string) (ed25519.PublicKey, error) {
	k, ok := v.ring.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("unknown CP key %q", keyID)
	}
	return k, nil
}

// VerifyBundle checks the signature over a proto.PolicyBundle.
func (v *SignatureVerifier) VerifyBundle(bundle *pb.PolicyBundle, sig []byte, keyID string) error {
	pub, err := v.getKey(keyID)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
	}
	if !ed25519.Verify(pub, data, sig) {
		return fmt.Errorf("bundle signature invalid")
	}
	return nil
}

// VerifyPolicy checks the signature over a single proto.PolicyRule.
func (v *SignatureVerifier) VerifyPolicy(rule *pb.PolicyRule, sig []byte, keyID string) error {
	pub, err := v.getKey(keyID)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal rule: %w", err)
	}
	if !ed25519.Verify(pub, data, sig) {
		return fmt.Errorf("policy %s signature invalid", rule.PolicyId)
	}
	return nil
}
