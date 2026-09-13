package policy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyEngine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	// 1. Create a temp directory for the bbolt database
	tempDir := t.TempDir()
	boltStore, err := store.OpenBoltStore(tempDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		boltStore.Close()
	})

	// 2. Initialize the engine (should start empty but successful)
	engine, err := NewEngine(ctx, boltStore, nil, "GATE-1", logger)
	require.NoError(t, err)
	require.NotNil(t, engine)

	// Check that we have a default deny when empty
	authCtx := AuthorizationContext{
		Principal: Principal{
			UserID: "user-123",
			Groups: []string{"engineering"},
		},
		Resource: Resource{
			AppID:  "app-1",
			Path:   "/api/v1/resource",
			Method: "GET",
		},
		TenantID:  "tenant-abc",
		GatewayID: "gw-1",
	}
	decision := engine.Evaluate(authCtx)
	assert.Equal(t, EffectDeny, decision.Effect)
	assert.Equal(t, "default_deny", decision.Reason)

	// 3. Define some mock policy rules
	rule1 := &proto.PolicyRule{
		PolicyId: "policy-allow-eng",
		TenantId: "tenant-abc",
		Effect:   proto.EffectEnum_EFFECT_ENUM_ALLOW,
		Priority: 10,
		Version:  1,
		Sequence: 1,
		Subject: &proto.SubjectSelector{
			Groups: []string{"engineering"},
		},
		Resource: &proto.ResourceSelector{
			AppIds:  []string{"app-1"},
			Methods: []string{"GET"},
		},
	}

	rule2 := &proto.PolicyRule{
		PolicyId: "policy-deny-blacklisted",
		TenantId: "tenant-abc",
		Effect:   proto.EffectEnum_EFFECT_ENUM_DENY,
		Priority: 20,
		Version:  1,
		Sequence: 2,
		Subject: &proto.SubjectSelector{
			Users: []string{"blacklisted-user"},
		},
		Resource: &proto.ResourceSelector{
			AppIds: []string{"*"},
		},
	}

	// 4. Update the engine with rule1 (ALLOW for engineering)
	rec1 := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule1,
	}

	checkpoint := store.SyncCheckpoint{
		LastSequence: 1,
		LastSyncAt:   time.Now(),
	}

	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1}, 1)
	require.NoError(t, err)

	// Evaluate again - user-123 is in engineering group, should match rule1 (ALLOW)
	decision = engine.Evaluate(authCtx)
	assert.Equal(t, EffectAllow, decision.Effect)
	assert.Equal(t, "policy-allow-eng", decision.PolicyID)

	// Evaluate for another user not in engineering - should get default deny
	authCtxNotEng := authCtx
	authCtxNotEng.Principal.Groups = []string{"sales"}
	decision = engine.Evaluate(authCtxNotEng)
	assert.Equal(t, EffectDeny, decision.Effect)
	assert.Equal(t, "default_deny", decision.Reason)

	// 5. Update the engine with rule2 (DENY for blacklisted-user) and delete nothing
	rec2 := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule2,
	}

	checkpoint.LastSequence = 2
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec2}, 2)
	require.NoError(t, err)

	// Evaluate for a blacklisted user who is also in engineering - DENY should take precedence
	authCtxBlacklisted := authCtx
	authCtxBlacklisted.Principal.UserID = "blacklisted-user"
	decision = engine.Evaluate(authCtxBlacklisted)
	assert.Equal(t, EffectDeny, decision.Effect)
	assert.Equal(t, "policy-deny-blacklisted", decision.PolicyID)

	// 6. Delete rule1 (ALLOW for engineering)
	recDelete1 := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_DELETE,
		Timestamp: time.Now().UnixMilli(),
		Rule: &proto.PolicyRule{
			PolicyId: "policy-allow-eng",
			TenantId: "tenant-abc",
			Version: 2,
			Sequence: 3,
		},
	}

	checkpoint.LastSequence = 3
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{recDelete1}, 3)
	require.NoError(t, err)

	// Now evaluating the engineering user should fallback to default deny
	decision = engine.Evaluate(authCtx)
	assert.Equal(t, EffectDeny, decision.Effect)
	assert.Equal(t, "default_deny", decision.Reason)
}

func TestPolicyEngineConditions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()
	tempDir := t.TempDir()
	boltStore, err := store.OpenBoltStore(tempDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		boltStore.Close()
	})

	engine, err := NewEngine(ctx, boltStore, nil, "GATE-1", logger)
	require.NoError(t, err)

	// Rule that requires MFA and restricts IP network
	rule := &proto.PolicyRule{
		PolicyId: "policy-mfa-network",
		TenantId: "tenant-abc",
		Effect:   proto.EffectEnum_EFFECT_ENUM_ALLOW,
		Priority: 10,
		Version:  1,
		Sequence: 1,
		Subject: &proto.SubjectSelector{
			Users: []string{"user-1"},
		},
		Resource: &proto.ResourceSelector{
			AppIds: []string{"app-1"},
		},
		Conditions: &proto.PolicyConditions{
			Mfa: &proto.MFACondition{
				Required: true,
				MinLevel: "totp",
			},
			Network: &proto.NetworkCondition{
				AllowedCidrs: []string{"192.168.1.0/24"},
			},
		},
	}

	rec := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule,
	}

	// checkpoint := store.SyncCheckpoint{
	// 	LastSequence: 1,
	// 	LastSyncAt:        time.Now(),
	// }

	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec}, 1)
	require.NoError(t, err)

	// Auth context missing MFA and wrong IP
	authCtx := AuthorizationContext{
		Principal: Principal{
			UserID:   "user-1",
			MFALevel: "",
		},
		Resource: Resource{
			AppID: "app-1",
		},
		Network: NetworkContext{
			SourceIP: net.ParseIP("10.0.0.1"),
		},
		TenantID:  "tenant-abc",
		GatewayID: "gw-1",
	}

	decision := engine.Evaluate(authCtx)
	assert.Equal(t, EffectDeny, decision.Effect) // doesn't match conditions -> default deny

	// Auth context with MFA but wrong IP
	authCtx.Principal.MFALevel = "totp"
	decision = engine.Evaluate(authCtx)
	assert.Equal(t, EffectDeny, decision.Effect)

	// Auth context with MFA and correct IP
	authCtx.Network.SourceIP = net.ParseIP("192.168.1.50")
	decision = engine.Evaluate(authCtx)
	assert.Equal(t, EffectAllow, decision.Effect)
	assert.Equal(t, "policy-mfa-network", decision.PolicyID)
}

func TestPolicyRollbackTombstone(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()
	tempDir := t.TempDir()
	boltStore, err := store.OpenBoltStore(tempDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		boltStore.Close()
	})

	engine, err := NewEngine(ctx, boltStore, nil, "GATE-1", logger)
	require.NoError(t, err)

	rule := &proto.PolicyRule{
		PolicyId: "policy-rollback",
		TenantId: "tenant-abc",
		Effect:   proto.EffectEnum_EFFECT_ENUM_ALLOW,
		Sequence: 10,
		Version: 1,
	}

	// 1. Upsert at sequence 10
	rec1 := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule,
	}
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1}, 10)
	require.NoError(t, err)

	// 2. Delete at sequence 11
	rule.Sequence = 11
	rule.Version = 2
	recDelete := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_DELETE,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule,
	}
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{recDelete}, 11)
	require.NoError(t, err)

	// 3. Try to replay sequence 10 (should fail due to rollback protection against tombstone)
	rule.Sequence = 10
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1}, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stale policy sequence")
}

func TestPolicyEngineAuthenticity(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	// Generate key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	verifier := &store.RootKey{PublicKey: pubKey}

	// 1. Create a temp directory for the bbolt database
	tempDir := t.TempDir()
	boltStore, err := store.OpenBoltStore(tempDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		boltStore.Close()
	})

	// 2. Initialize the engine with our verifier
	engine, err := NewEngine(ctx, boltStore, verifier, "GATE-1", logger)
	require.NoError(t, err)
	require.NotNil(t, engine)

	rule1 := &proto.PolicyRule{
		PolicyId: "policy-auth-1",
		TenantId: "tenant-abc",
		Effect:   proto.EffectEnum_EFFECT_ENUM_ALLOW,
		Priority: 10,
		Version:  1,
		Sequence: 1,
		Subject: &proto.SubjectSelector{
			Users: []string{"user-1"},
		},
		Resource: &proto.ResourceSelector{
			AppIds: []string{"app-1"},
		},
	}

	rec1 := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule1,
	}

	// Case A: Update policies with an UNSIGNED record should fail
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1}, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature invalid")

	// Case B: Update policies with a CORRECTLY signed record should succeed
	payload1 := store.RecordSigningPayload(rec1)
	sig1 := ed25519.Sign(privKey, payload1)
	rec1.Signature = sig1

	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1}, 1)
	require.NoError(t, err)

	// Case C: ApplyVerifiedDelta with an UNSIGNED bundle should fail
	rec2 := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule: &proto.PolicyRule{
			PolicyId: "policy-auth-2",
			TenantId: "tenant-abc",
			Effect:   proto.EffectEnum_EFFECT_ENUM_ALLOW,
			Priority: 10,
			Version:  1,
			Sequence: 2,
			Subject: &proto.SubjectSelector{
				Users: []string{"user-2"},
			},
			Resource: &proto.ResourceSelector{
				AppIds: []string{"app-1"},
			},
		},
	}
	payload2 := store.RecordSigningPayload(rec2)
	rec2.Signature = ed25519.Sign(privKey, payload2)

	bundle := &proto.PolicyBundle{
		LastSequence: 2,
		IssuedAt:     time.Now().UnixMilli(),
		Records:      []*proto.PolicyRecord{rec2},
	}

	err = engine.ApplyVerifiedDelta(ctx, bundle)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bundle signature verification failed")

	// Case D: ApplyVerifiedDelta with a CORRECTLY signed bundle should succeed
	bundlePayload := store.BundleSigningPayload(bundle)
	bundle.Signature = ed25519.Sign(privKey, bundlePayload)

	err = engine.ApplyVerifiedDelta(ctx, bundle)
	require.NoError(t, err)

	// Case E: Initialize a new engine from a store containing a tampered/invalid record signature
	recTampered := &proto.PolicyRecord{
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule: &proto.PolicyRule{
			PolicyId: "policy-tampered",
			TenantId: "tenant-abc",
			Effect:   proto.EffectEnum_EFFECT_ENUM_ALLOW,
			Sequence: 3,
		},
		Signature: []byte("bad-signature-value-here"),
	}

	// ApplyDelta writes directly to database without verification
	err = boltStore.ApplyDelta(ctx, []*proto.PolicyRecord{recTampered}, 3)
	require.NoError(t, err)

	// Attempting to create a new engine should now fail during bootstrap verification of the local store
	_, err = NewEngine(ctx, boltStore, verifier, "GATE-1", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrap verify policy 3")
}
