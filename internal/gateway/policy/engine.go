package policy

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)



type PolicyEngine struct {
	mu            sync.RWMutex
	index         *PolicyIndex
	scheduleCache map[string]*proto.Schedule
}

func NewEngine(ctx context.Context, bboltStore *store.BoltStore, sigVerifier *store.RootKey, logger *zap.Logger) (*PolicyEngine, error) {

	logger.Info("policy bootstrap: loading from local store")

	cp, err := bboltStore.GetCheckpoint(ctx)
	if err != nil {
		return  nil, fmt.Errorf("load checkpoint: %w", err)
	}

	records, err := bboltStore.LoadAllRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}

	if len(records) == 0 {
		logger.Warn("no local policies; awaiting first CP sync")
		return nil, nil
	}

		// --- Verify state hash ---
	rules := make([]*proto.PolicyRule, len(records))
	for i, r := range records {
		rules[i] = r.Rule
	}
	computedHash := store.CalculateStateHash(sm.gatewayID, sm.tenantID, rules)

	if cp.StateHash != "" && cp.StateHash != computedHash {
		return nil, fmt.Errorf("%w: stored=%s computed=%s", ErrHashMismatch, cp.StateHash, computedHash)
	}

		// --- Verify signatures (if checkpoint claims CP-signed) ---
	if cp.SignedByCP {
		for _, rec := range records {
			if err := sigVerifier.VerifyPolicy(rec.Rule, rec.Signature, rec.CPKeyID); err != nil {
				return nil, fmt.Errorf("%w: policy %s: %v", ErrSigInvalid, rec.ID, err)
			}
		}
	}

	// --- Build index and swap ---
	bundle := &proto.PolicyBundle{
		Version: cp.LastBundleVersion,
		Rules:   rules,
	}
	idx, err := BuildFromBundle(bundle)
	if err != nil {
		return nil, fmt.Errorf("build index: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.index = idx

	sm.logger.InfoContext(ctx, "policy bootstrap complete",
		slog.Int("policies", len(rules)),
		slog.Int64("bundle_version", cp.LastBundleVersion),
		slog.String("state_hash", computedHash),
	)
	return nil
	
	// index, err := buildIndexFromRecords(records, sigVerifier, logger)
	// if err != nil {
	// 	return nil, fmt.Errorf("build index: %w", err)
	// }

	
	//Load from store, verify policies, and Compile
	return &PolicyEngine{
		index:         NewIndex(),
		scheduleCache: make(map[string]*proto.Schedule),
	}, nil
}

func (e *PolicyEngine) SwapIndex(newIndex *PolicyIndex) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.index = newIndex
}

func (e *PolicyEngine) CurrentVersion() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.index == nil {
		return 0
	}
	return e.index.version
}

func (e *PolicyEngine) Evaluate(ctx AuthorizationContext) Decision {
	now := time.Now()

	e.mu.RLock()
	idx := e.index
	e.mu.RUnlock()

	if idx == nil {
		return Decision{
			Effect:      EffectDeny,
			Reason:      "no_policies_loaded",
			EvaluatedAt: now,
			UserID:      ctx.Principal.UserID,
			AppID:       ctx.Resource.AppID,
			GatewayID:   ctx.GatewayID,
			TenantID:    ctx.TenantID,
		}
	}

	candidates := make([]*CompiledPolicy, 0, 32)
	if globals, ok := idx.globalPolicies[ctx.TenantID]; ok {
		candidates = append(candidates, globals...)
	}
	if tenantApps, ok := idx.byTenantApp[ctx.TenantID]; ok {
		if appPolicies, ok := tenantApps[ctx.Resource.AppID]; ok {
			candidates = append(candidates, appPolicies...)
		}
	}

	var denies, allows []*CompiledPolicy
	for _, cp := range candidates {
		if cp.Effect == EffectDeny {
			denies = append(denies, cp)
		} else {
			allows = append(allows, cp)
		}
	}

	// STEP 1: DENY policies
	for _, cp := range denies {
		if !cp.matchesSubject(ctx.Principal) || !cp.matchesResource(ctx.Resource) {
			continue
		}
		if ok, reason := cp.matchesConditions(ctx); ok {
			return Decision{
				Effect:        EffectDeny,
				PolicyID:      cp.PolicyID,
				Reason:        reason,
				EvaluatedAt:   now,
				UserID:        ctx.Principal.UserID,
				AppID:         ctx.Resource.AppID,
				GatewayID:     ctx.GatewayID,
				TenantID:      ctx.TenantID,
				PolicyVersion: cp.Version,
			}
		}
	}

	// STEP 2: ALLOW policies
	for _, cp := range allows {
		if !cp.matchesSubject(ctx.Principal) || !cp.matchesResource(ctx.Resource) {
			continue
		}
		if ok, _ := cp.matchesConditions(ctx); ok {
			return Decision{
				Effect:        EffectAllow,
				PolicyID:      cp.PolicyID,
				Reason:        "policy_matched",
				EvaluatedAt:   now,
				UserID:        ctx.Principal.UserID,
				AppID:         ctx.Resource.AppID,
				GatewayID:     ctx.GatewayID,
				TenantID:      ctx.TenantID,
				PolicyVersion: cp.Version,
			}
		}
	}

	// STEP 3: DEFAULT DENY
	return Decision{
		Effect:      EffectDeny,
		PolicyID:    "",
		Reason:      "default_deny",
		EvaluatedAt: now,
		UserID:      ctx.Principal.UserID,
		AppID:       ctx.Resource.AppID,
		GatewayID:   ctx.GatewayID,
		TenantID:    ctx.TenantID,
	}
}