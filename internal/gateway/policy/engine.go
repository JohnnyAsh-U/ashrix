package policy

import (
	"sync"
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)



type PolicyEngine struct {
	mu            sync.RWMutex
	index         *PolicyIndex
	scheduleCache map[string]*proto.Schedule
}

func NewEngine() *PolicyEngine {
	return &PolicyEngine{
		index:         NewIndex(),
		scheduleCache: make(map[string]*proto.Schedule),
	}
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