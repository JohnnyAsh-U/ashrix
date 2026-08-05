package policy

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPolicyEngine(t *testing.T) {
	logger := zap.NewNop()
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
		Subject: &proto.SubjectSelector{
			Users: []string{"blacklisted-user"},
		},
		Resource: &proto.ResourceSelector{
			AppIds: []string{"*"},
		},
	}

	// 4. Update the engine with rule1 (ALLOW for engineering)
	rec1 := &proto.PolicyRecord{
		Sequence:  1,
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule1,
	}

	checkpoint := store.SyncCheckpoint{
		LastBundleVersion: 1,
		LastSyncAt:        time.Now(),
	}

	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1}, checkpoint)
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
		Sequence:  1,
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule2,
	}

	checkpoint.LastBundleVersion = 2
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec2}, checkpoint)
	require.NoError(t, err)

	// Evaluate for a blacklisted user who is also in engineering - DENY should take precedence
	authCtxBlacklisted := authCtx
	authCtxBlacklisted.Principal.UserID = "blacklisted-user"
	decision = engine.Evaluate(authCtxBlacklisted)
	assert.Equal(t, EffectDeny, decision.Effect)
	assert.Equal(t, "policy-deny-blacklisted", decision.PolicyID)

	// 6. Delete rule1 (ALLOW for engineering)
	recDelete1 := &proto.PolicyRecord{
		Sequence:  2,
		Operation: proto.OperationEnum_OPERATION_ENUM_DELETE,
		Timestamp: time.Now().UnixMilli(),
		Rule: &proto.PolicyRule{
			PolicyId: "policy-allow-eng",
			TenantId: "tenant-abc",
		},
	}

	checkpoint.LastBundleVersion = 3
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{recDelete1}, checkpoint)
	require.NoError(t, err)

	// Now evaluating the engineering user should fallback to default deny
	decision = engine.Evaluate(authCtx)
	assert.Equal(t, EffectDeny, decision.Effect)
	assert.Equal(t, "default_deny", decision.Reason)
}

func TestPolicyEngineConditions(t *testing.T) {
	logger := zap.NewNop()
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
		Sequence:  1,
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule,
	}

	checkpoint := store.SyncCheckpoint{
		LastBundleVersion: 1,
		LastSyncAt:        time.Now(),
	}

	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec}, checkpoint)
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
	logger := zap.NewNop()
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
	}

	// 1. Upsert at sequence 10
	rec1 := &proto.PolicyRecord{
		Sequence:  10,
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule,
	}
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1}, store.SyncCheckpoint{LastBundleVersion: 1})
	require.NoError(t, err)

	// 2. Delete at sequence 11
	recDelete := &proto.PolicyRecord{
		Sequence:  11,
		Operation: proto.OperationEnum_OPERATION_ENUM_DELETE,
		Timestamp: time.Now().UnixMilli(),
		Rule:      rule,
	}
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{recDelete}, store.SyncCheckpoint{LastBundleVersion: 2})
	require.NoError(t, err)

	// 3. Try to replay sequence 10 (should fail due to rollback protection against tombstone)
	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1}, store.SyncCheckpoint{LastBundleVersion: 3})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stale policy sequence")
}
