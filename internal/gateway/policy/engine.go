package policy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// =============================================================================
// Policy Engine (Single unified component struct)
// =============================================================================

type PolicyEngine struct {
	mu            sync.RWMutex
	store         *store.BoltStore
	verifier      *store.RootKey
	logger        *slog.Logger
	scheduleCache map[string]*proto.Schedule
	gatewayID     string

	// In-memory index maps
	byTenantApp    map[string]map[string][]*CompiledPolicy
	globalPolicies map[string][]*CompiledPolicy
	version        int64
}

func NewEngine(
	ctx context.Context,
	bboltStore *store.BoltStore,
	sigVerifier *store.RootKey,
	gatewayID string,
	logger *slog.Logger,
) (*PolicyEngine, error) {
	logger.Info("policy bootstrap: loading from local store")

	engine := &PolicyEngine{
		store:          bboltStore,
		verifier:       sigVerifier,
		logger:         logger,
		gatewayID:      gatewayID,
		scheduleCache:  make(map[string]*proto.Schedule),
		byTenantApp:    make(map[string]map[string][]*CompiledPolicy),
		globalPolicies: make(map[string][]*CompiledPolicy),
	}

	cp, err := bboltStore.GetCheckpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}

	records, err := bboltStore.LoadAllRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}

	if len(records) == 0 {
		logger.Warn("no local policies; awaiting first CP sync")
		return engine, nil
	}

	// Verify signatures (if checkpoint claims CP-signed)
	if sigVerifier != nil {
		for _, rec := range records {
			if err := sigVerifier.VerifyRecord(rec); err != nil {
				return nil, fmt.Errorf("bootstrap verify policy %d: %w", rec.Rule.Sequence, err)
			}
		}
		logger.Info("all policy signatures verified", slog.Int("count", len(records)))
	}

	// Build index
	rules := make([]*proto.PolicyRule, len(records))
	for i, r := range records {
		rules[i] = r.Rule
	}

	byTenantApp, globalPolicies, err := engine.buildIndex(rules)
	if err != nil {
		return nil, fmt.Errorf("build index: %w", err)
	}

	engine.byTenantApp = byTenantApp
	engine.globalPolicies = globalPolicies
	engine.version = cp.LastSequence

	logger.Info("policy bootstrap complete",
		slog.Int("policies", len(rules)),
		slog.Int64("bundle_version", cp.LastSequence),
	)

	return engine, nil
}

func (e *PolicyEngine) Compile(rule *proto.PolicyRule) (*CompiledPolicy, error) {
	var eff Effect
	if rule.Effect == proto.EffectEnum_EFFECT_ENUM_DENY {
		eff = EffectDeny
	} else {
		eff = EffectAllow
	}

	cp := &CompiledPolicy{
		Proto:    rule,
		Effect:   eff,
		PolicyID: rule.PolicyId,
		Priority: int(rule.Priority),
		Version:  rule.Version,
	}

	if rule.Subject != nil {
		cp.Groups = makeSet(rule.Subject.Groups)
		cp.Users = makeSet(rule.Subject.Users)
		cp.SourceAppIDs = makeSet(rule.Subject.Apps)
	} else {
		cp.Groups = make(map[string]struct{})
		cp.Users = make(map[string]struct{})
		cp.SourceAppIDs = make(map[string]struct{})
	}
	if rule.Resource != nil {
		cp.AppIDs = makeSet(rule.Resource.AppIds)
		cp.Methods = makeSet(rule.Resource.Methods)
		for _, p := range rule.Resource.Paths {
			cp.PathMatchers = append(cp.PathMatchers, compilePathMatcher(p))
		}
	} else {
		cp.AppIDs = make(map[string]struct{})
		cp.Methods = make(map[string]struct{})
	}
	if len(cp.Methods) == 0 {
		cp.Methods["*"] = struct{}{}
	}

	if rule.Conditions != nil {
		if rule.Conditions.Mfa != nil {
			cp.RequireMFA = rule.Conditions.Mfa.Required
			cp.MinMFALevel = rule.Conditions.Mfa.MinLevel
		}
		if rule.Conditions.Device != nil {
			cp.RequirePosture = rule.Conditions.Device.Postures
		}
		if rule.Conditions.Network != nil {
			nc := rule.Conditions.Network
			cp.BlockTor = nc.BlockTor
			cp.Countries = makeSet(nc.AllowedCountries)
			cp.BlockCountries = makeSet(nc.BlockedCountries)
			for _, cidr := range nc.AllowedCidrs {
				_, ipnet, err := net.ParseCIDR(cidr)
				if err != nil {
					return nil, fmt.Errorf("invalid allowed_cidr %q in policy %s: %w", cidr, cp.PolicyID, err)
				}
				cp.AllowedCIDRs = append(cp.AllowedCIDRs, ipnet)
			}
			for _, cidr := range nc.BlockedCidrs {
				_, ipnet, err := net.ParseCIDR(cidr)
				if err != nil {
					return nil, fmt.Errorf("invalid blocked_cidr %q in policy %s: %w", cidr, cp.PolicyID, err)
				}
				cp.BlockedCIDRs = append(cp.BlockedCIDRs, ipnet)
			}
		}
		if rule.Conditions.Time != nil {
			cp.HasTimeCond = true
			cp.ScheduleName = rule.Conditions.Time.ScheduleName
		}
	}

	return cp, nil
}

func (e *PolicyEngine) buildIndex(rules []*proto.PolicyRule) (map[string]map[string][]*CompiledPolicy, map[string][]*CompiledPolicy, error) {
	byTenantApp := make(map[string]map[string][]*CompiledPolicy)
	globalPolicies := make(map[string][]*CompiledPolicy)

	for _, rule := range rules {
		cp, err := e.Compile(rule)
		if err != nil {
			e.logger.Warn("skipping uncompileable policy",
				slog.String("policy_id", rule.PolicyId),
				slog.Any("error", err))
			continue
		}

		tenantID := cp.Proto.TenantId
		isGlobal := len(cp.AppIDs) == 0 || hasWildcard(cp.AppIDs)

		if isGlobal {
			globalPolicies[tenantID] = append(globalPolicies[tenantID], cp)
			continue
		}

		if byTenantApp[tenantID] == nil {
			byTenantApp[tenantID] = make(map[string][]*CompiledPolicy)
		}
		for appID := range cp.AppIDs {
			byTenantApp[tenantID][appID] = append(byTenantApp[tenantID][appID], cp)
		}
	}

	return byTenantApp, globalPolicies, nil
}

func hasWildcard(m map[string]struct{}) bool {
	_, ok := m["*"]
	return ok
}

func makeSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, item := range items {
		m[item] = struct{}{}
	}
	return m
}

func (e *PolicyEngine) VerifyPolicy(rule *proto.PolicyRecord, sig []byte) error {
	if e.verifier == nil {
		return errors.New("signature verifier not initialized")
	}
	return e.verifier.VerifyRecord(rule)
}

func (e *PolicyEngine) VerifyBundle(bundle *proto.PolicyBundle) error {
	if e.verifier == nil {
		return errors.New("signature verifier not initialized")
	}
	return e.verifier.VerifyBundle(bundle)
}

// ApplyVerifiedDelta verifies the bundle signature first, then applies the delta.
// Use this for all CP-initiated updates so the entire payload is authenticated.
func (e *PolicyEngine) ApplyVerifiedDelta(
	ctx context.Context,
	policyBundle *proto.PolicyBundle,
) error {
	if e.verifier != nil {
		if err := e.verifier.VerifyBundle(policyBundle); err != nil {
			return fmt.Errorf("bundle signature verification failed: %w", err)
		}
		e.logger.Info("bundle signature verified",
			slog.Int64("version", policyBundle.LastSequence))
	}
	return e.UpdatePolicies(ctx, policyBundle.Records, policyBundle.LastSequence)
}

func (e *PolicyEngine) UpdatePolicies(
	ctx context.Context,
	policies []*proto.PolicyRecord,
	lastSeq int64,
) error {
	// 1. Verify signatures of each policy if signed
	if e.verifier != nil {
		for _, pr := range policies {
			if err := e.verifier.VerifyRecord(pr); err != nil {
				return fmt.Errorf("verify policy %s: %w", pr.Rule.PolicyId, err)
			}
		}
	}

	// 2. Validate compilation of rules before persisting
	for _, pr := range policies {
		if _, err := e.Compile(pr.Rule); err != nil {
			return fmt.Errorf("compile check for policy %s failed: %w", pr.Rule.PolicyId, err)
		}
	}

	// 3. Write to store
	if err := e.store.ApplyDelta(ctx, policies, lastSeq); err != nil {
		return fmt.Errorf("apply delta: %w", err)
	}

	// 4. Reload all live records to rebuild index and swap
	records, err := e.store.LoadAllRecords(ctx)
	if err != nil {
		return fmt.Errorf("load records post-update: %w", err)
	}

	rules := make([]*proto.PolicyRule, len(records))
	for i, r := range records {
		rules[i] = r.Rule
	}

	byTenantApp, globalPolicies, err := e.buildIndex(rules)
	if err != nil {
		return fmt.Errorf("build index post-update: %w", err)
	}

	e.mu.Lock()
	e.byTenantApp = byTenantApp
	e.globalPolicies = globalPolicies
	e.version = lastSeq
	e.mu.Unlock()

	e.logger.Info("policy update applied and engine reloaded",
		slog.Int("policies", len(rules)),
		slog.Int64("bundle_version", lastSeq),
	)
	return nil
}

func (e *PolicyEngine) CurrentVersion() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.version
}

func (e *PolicyEngine) Evaluate(ctx AuthorizationContext) Decision {
	now := time.Now()

	e.mu.RLock()
	byTenantApp := e.byTenantApp
	globalPolicies := e.globalPolicies
	schedules := e.scheduleCache
	e.mu.RUnlock()

	if byTenantApp == nil || globalPolicies == nil {
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
	if globals, ok := globalPolicies[ctx.TenantID]; ok {
		candidates = append(candidates, globals...)
	}
	if tenantApps, ok := byTenantApp[ctx.TenantID]; ok {
		if appPolicies, ok := tenantApps[ctx.Resource.AppID]; ok {
			candidates = append(candidates, appPolicies...)
		}
	}

	// Sort by priority descending so higher-priority rules match first.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})

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
		if ok, reason := cp.matchesConditions(ctx, schedules); ok {
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
		if ok, _ := cp.matchesConditions(ctx, schedules); ok {
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
