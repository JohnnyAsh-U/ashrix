package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyRecord(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	verifier := &RootKey{PublicKey: pub}

	rule := &pb.PolicyRule{
		PolicyId: "policy-1",
		TenantId: "tenant-1",
		Effect:   pb.EffectEnum_EFFECT_ENUM_ALLOW,
		Sequence:  10,
	}

	record := &pb.PolicyRecord{
		Operation: pb.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule,
	}

	// 1. Unsigned record should fail
	err = verifier.VerifyRecord(record)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature invalid")

	// 2. Correctly signed record should pass
	payload := RecordSigningPayload(record)
	sig := ed25519.Sign(priv, payload)
	record.Signature = sig

	err = verifier.VerifyRecord(record)
	assert.NoError(t, err)

	// 3. Modified sequence should fail
	record.Rule.Sequence = 11
	err = verifier.VerifyRecord(record)
	assert.Error(t, err)
	record.Rule.Sequence = 10 // restore

	// 4. Modified rule content should fail
	record.Rule.Effect = pb.EffectEnum_EFFECT_ENUM_DENY
	err = verifier.VerifyRecord(record)
	assert.Error(t, err)
	record.Rule.Effect = pb.EffectEnum_EFFECT_ENUM_ALLOW // restore

	// 5. Wrong public key / verifier should fail
	pub2, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	verifier2 := &RootKey{PublicKey: pub2}
	err = verifier2.VerifyRecord(record)
	assert.Error(t, err)
}

func TestVerifyBundle(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	verifier := &RootKey{PublicKey: pub}

	rule1 := &pb.PolicyRule{
		PolicyId: "policy-1",
		TenantId: "tenant-1",
		Effect:   pb.EffectEnum_EFFECT_ENUM_ALLOW,
		Sequence:  1,
	}
	record1 := &pb.PolicyRecord{
		Operation: pb.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule1,
	}
	// Sign record1
	payload1 := RecordSigningPayload(record1)
	record1.Signature = ed25519.Sign(priv, payload1)

	rule2 := &pb.PolicyRule{
		PolicyId: "policy-2",
		TenantId: "tenant-1",
		Effect:   pb.EffectEnum_EFFECT_ENUM_DENY,
		Sequence:  2,
	}
	record2 := &pb.PolicyRecord{
		Operation: pb.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule2,
	}
	// Sign record2
	payload2 := RecordSigningPayload(record2)
	record2.Signature = ed25519.Sign(priv, payload2)

	bundle := &pb.PolicyBundle{
		LastSequence:  2,
		IssuedAt: time.Now().UnixMilli(),
		Records:  []*pb.PolicyRecord{record1, record2},
	}

	// 1. Unsigned bundle should fail
	err = verifier.VerifyBundle(bundle)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bundle signature invalid")

	// 2. Correctly signed bundle should pass
	bundlePayload := BundleSigningPayload(bundle)
	bundle.Signature = ed25519.Sign(priv, bundlePayload)

	err = verifier.VerifyBundle(bundle)
	assert.NoError(t, err)

	// 3. Modified bundle version should fail
	bundle.LastSequence = 4
	err = verifier.VerifyBundle(bundle)
	assert.Error(t, err)
	bundle.LastSequence = 2 // restore

	// 4. Bundle with an unsigned / incorrectly signed record should fail (even if bundle signature is valid for that state)
	record3 := &pb.PolicyRecord{
		Operation: pb.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule1,
	}
	// Do not sign record3 or sign with different key
	_, priv3, _ := ed25519.GenerateKey(rand.Reader)
	record3.Signature = ed25519.Sign(priv3, RecordSigningPayload(record3))

	bundleWithBadRecord := &pb.PolicyBundle{
		LastSequence: 3,
		IssuedAt: time.Now().UnixMilli(),
		Records:  []*pb.PolicyRecord{record1, record3},
	}
	bundleWithBadRecord.Signature = ed25519.Sign(priv, BundleSigningPayload(bundleWithBadRecord))
	err = verifier.VerifyBundle(bundleWithBadRecord)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bundle contains invalid record")
}
