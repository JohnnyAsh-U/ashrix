package store

import (
	// "crypto/ecdsa"
	"crypto/ed25519"
	"crypto/x509"
	_ "embed"
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

// VerifyBundle checks the signature over a proto.PolicyBundle.
func (v *RootKey) VerifyBundle(bundle *pb.PolicyBundle, sig []byte, keyID string) error {

	data, err := proto.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
	}
	if !ed25519.Verify(v.PublicKey, data, sig) {
		return fmt.Errorf("bundle signature invalid")
	}
	return nil
}

// VerifyPolicy checks the signature over a single proto.PolicyRule.
func (v *RootKey) VerifyPolicy(rule *pb.PolicyRule, sig []byte, keyID string) error {

	data, err := proto.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal rule: %w", err)
	}
	if !ed25519.Verify(v.PublicKey, data, sig) {
		return fmt.Errorf("policy %s signature invalid", rule.PolicyId)
	}
	return nil
}
