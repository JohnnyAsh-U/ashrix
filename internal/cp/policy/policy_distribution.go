package policy

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/crypto"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================
// Policy Distributor — compiles signed bundles and pushes to gateways
// ============================================================

type PolicyDistributor struct {
	registry    *registry.GatewayRegistry
	policyStore Repository
	signer      crypto.BundleSigning // raw key access for Ed25519 signing
	log         *slog.Logger

	// Cache of compiled bundles by org. Key: "orgID:version"
	bundleCache map[string]*pb.PolicyBundle
	mu          sync.RWMutex
}

func NewPolicyDistributor(
	reg *registry.GatewayRegistry,
	store Repository,
	signer crypto.BundleSigning,
	log *slog.Logger,
) *PolicyDistributor {
	return &PolicyDistributor{
		registry:    reg,
		policyStore: store,
		signer:      signer,
		log:         log,
		bundleCache: make(map[string]*pb.PolicyBundle),
	}
}

// Distribute is called after any policy mutation. It compiles the latest
// snapshot, signs it, caches it, and pushes to all connected gateways.
func (d *PolicyDistributor) Distribute(ctx context.Context, orgID uuid.UUID) error {
	bundle, version, err := d.compileSnapshotBundle(ctx, orgID)
	if err != nil {
		return fmt.Errorf("compile bundle for org %s: %w", orgID, err)
	}

	cacheKey := fmt.Sprintf("%s:%d", orgID, version)
	d.mu.Lock()
	d.bundleCache[cacheKey] = bundle
	d.mu.Unlock()

	gateways := d.registry.GetConnectionsForTenant(orgID.String())
	if len(gateways) == 0 {
		d.log.Info("no connected gateways for org", "org_id", orgID, "version", version)
		return nil
	}

	var wg sync.WaitGroup
	for _, gw := range gateways {
		wg.Add(1)
		go func(conn *registry.GatewayConn) {
			defer wg.Done()
			if err := d.sendBundle(ctx, conn, bundle); err != nil {
				d.log.Error("failed to push bundle", "gateway_id", conn.GatewayID, "error", err)
			}
		}(gw)
	}
	wg.Wait()

	return nil
}

// PushToGateway sends the latest full snapshot to a single gateway.
// Use this when a gateway reconnects and HelloAck.needs_policy=true.
func (d *PolicyDistributor) PushToGateway(ctx context.Context, conn *registry.GatewayConn, orgID uuid.UUID) error {
	bundle, _, err := d.compileSnapshotBundle(ctx, orgID)
	if err != nil {
		return fmt.Errorf("compile snapshot: %w", err)
	}
	return d.sendBundle(ctx, conn, bundle)
}

// PushDelta sends only mutations since sinceVersion. Falls back to snapshot
// if delta computation fails.
func (d *PolicyDistributor) PushDelta(ctx context.Context, conn *registry.GatewayConn, orgID uuid.UUID, sinceVersion int64) error {
	bundle, err := d.compileDeltaBundle(ctx, orgID, sinceVersion)
	if err != nil {
		d.log.Warn("delta compilation failed, falling back to snapshot", "error", err)
		return d.PushToGateway(ctx, conn, orgID)
	}
	return d.sendBundle(ctx, conn, bundle)
}

// ---------------------------------------------------------------------
// Bundle compilation
// ---------------------------------------------------------------------

func (d *PolicyDistributor) compileSnapshotBundle(ctx context.Context, orgID uuid.UUID) (*pb.PolicyBundle, int64, error) {
	policies, err := d.policyStore.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("list policies: %w", err)
	}

	sequence, err := d.policyStore.GetLatestPolicySequence(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("get latest version: %w", err)
	}

	now := time.Now().UTC()
	issuedAt := now.UnixMilli()
	records := make([]*pb.PolicyRecord, 0, len(policies))

	for _, p := range policies {
		record := policyToRecord(p, issuedAt)
		if err := d.signRecord(record); err != nil {
			return nil, 0, fmt.Errorf("sign record %s: %w", p.ID, err)
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return &pb.PolicyBundle{}, 0, nil
	}

	bundle := &pb.PolicyBundle{
		LastSequence: sequence,
		IssuedAt:     issuedAt,
		Records:      records,
		Signature:    nil, // signed below
	}

	if err := d.signBundle(bundle); err != nil {
		return nil, 0, fmt.Errorf("sign bundle: %w", err)
	}

	return bundle, sequence, nil
}

func (d *PolicyDistributor) compileDeltaBundle(ctx context.Context, orgID uuid.UUID, sinceVersion int64) (*pb.PolicyBundle, error) {
	mutations, err := d.policyStore.GetMutationsSince(ctx, orgID, sinceVersion)
	if err != nil {
		return nil, fmt.Errorf("get mutations: %w", err)
	}
	if len(mutations) == 0 {
		return nil, nil
	}

	now := time.Now().UTC().UnixMilli()
	records := make([]*pb.PolicyRecord, 0, len(mutations))

	for _, m := range mutations {
		record, err := mutationToRecord(m)
		if err != nil {
			return nil, fmt.Errorf("convert mutation v%d: %w", m.Version, err)
		}
		// Override timestamp to bundle issuance time for snapshot consistency
		record.Timestamp = now
		if err := d.signRecord(record); err != nil {
			return nil, fmt.Errorf("sign mutation v%d: %w", m.Version, err)
		}
		records = append(records, record)
	}

	latestSequence := mutations[len(mutations)-1].Sequence
	bundle := &pb.PolicyBundle{
		LastSequence: latestSequence,
		IssuedAt:     now,
		Records:      records,
		Signature:    nil,
	}

	if err := d.signBundle(bundle); err != nil {
		return nil, fmt.Errorf("sign delta bundle: %w", err)
	}

	return bundle, nil
}

// Get latest version of policy for a given org
func (d *PolicyDistributor) LatestSequence(orgID uuid.UUID) int64 {
	version, err := d.policyStore.GetLatestPolicySequence(context.Background(), orgID)
	if err != nil {
		d.log.Error("failed to get latest version", "org_id", orgID, "error", err)
		return 0
	}
	return version
}

// ---------------------------------------------------------------------
// Signing (must exactly match gateway verification in store/verify.go)
// ---------------------------------------------------------------------

func (d *PolicyDistributor) signRecord(record *pb.PolicyRecord) error {
	payload := recordSigningPayload(record)
	sig := d.signer.SignBundle(payload)
	record.Signature = sig
	return nil
}

func (d *PolicyDistributor) signBundle(bundle *pb.PolicyBundle) error {
	payload := bundleSigningPayload(bundle)
	sig := d.signer.SignBundle(payload)
	bundle.Signature = sig
	return nil
}

// recordSigningPayload mirrors the gateway's RecordSigningPayload exactly.
func recordSigningPayload(record *pb.PolicyRecord) []byte {
	h := sha256.New()
	binary.Write(h, binary.BigEndian, record.Rule.Sequence)
	binary.Write(h, binary.BigEndian, int32(record.Operation))
	binary.Write(h, binary.BigEndian, record.Timestamp)
	h.Write(canonicalRuleHash(record.Rule))
	return h.Sum(nil)
}

// bundleSigningPayload mirrors the gateway's BundleSigningPayload exactly.
func bundleSigningPayload(bundle *pb.PolicyBundle) []byte {
	h := sha256.New()
	binary.Write(h, binary.BigEndian, bundle.LastSequence)
	binary.Write(h, binary.BigEndian, bundle.IssuedAt)
	binary.Write(h, binary.BigEndian, int64(len(bundle.Records)))
	for _, rec := range bundle.Records {
		h.Write(rec.Signature)
	}
	return h.Sum(nil)
}

// canonicalRuleHash mirrors the gateway's canonicalRuleHash.
func canonicalRuleHash(rule *pb.PolicyRule) []byte {
	if rule == nil {
		h := sha256.Sum256([]byte{0x00})
		return h[:]
	}
	h := sha256.New()
	// Use deterministic proto marshal. This MUST match the gateway exactly.
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(rule)
	if err != nil {
		panic(fmt.Sprintf("marshal rule: %v", err))
	}
	h.Write(b)
	return h.Sum(nil)
}

// ---------------------------------------------------------------------
// Transport
// ---------------------------------------------------------------------

func (d *PolicyDistributor) sendBundle(ctx context.Context, conn *registry.GatewayConn, bundle *pb.PolicyBundle) error {
	msg := &pb.CPEnvelope{
		SentAt: timestamppb.Now(),
		Payload: &pb.CPEnvelope_PolicyBundle{
			PolicyBundle: bundle,
		},
	}
	return conn.Send(msg)
}

// ---------------------------------------------------------------------
// Domain → Proto conversion
// ---------------------------------------------------------------------

func policyToRecord(p Policy, timestamp int64) *pb.PolicyRecord {
	rule := &pb.PolicyRule{
		PolicyId:    p.ID.String(),
		TenantId:    p.OrgID.String(),
		Name:        p.Name,
		Description: p.Description,
		Priority:    p.Priority,
		Enabled:     p.Enabled,
		Version:     p.Version,
		CreatedAt:   timestamppb.New(p.CreatedAt),
	}

	switch p.Effect {
	case EffectAllow:
		rule.Effect = pb.EffectEnum_EFFECT_ENUM_ALLOW
	case EffectDeny:
		rule.Effect = pb.EffectEnum_EFFECT_ENUM_DENY
	}

	users, groups, apps := partitionSubjects(p.Subjects)
	rule.Subject = &pb.SubjectSelector{Users: users, Groups: groups, Apps: apps}

	apps, paths, methods := partitionResources(p.Resources)
	rule.Resource = &pb.ResourceSelector{AppIds: apps, Paths: paths, Methods: methods}

	rule.Conditions = conditionsToProto(p.Conditions)

	return &pb.PolicyRecord{
		Operation: pb.OperationEnum_OPERATION_ENUM_UPSERT,
		Timestamp: timestamp,
		Rule:      rule,
	}
}

func mutationToRecord(m Mutation) (*pb.PolicyRecord, error) {
	rule, err := snapshotToRule(m.Snapshot, m.Version, m.PolicyID.String())
	if err != nil {
		return nil, err
	}

	op := pb.OperationEnum_OPERATION_ENUM_UNSPECIFIED
	switch m.Op {
	case "UPSERT":
		op = pb.OperationEnum_OPERATION_ENUM_UPSERT
	case "DELETE":
		op = pb.OperationEnum_OPERATION_ENUM_DELETE
	}

	return &pb.PolicyRecord{
		Operation: op,
		Timestamp: m.RecordTimestamp,
		Rule:      rule,
	}, nil
}

func snapshotToRule(snapshot map[string]interface{}, version int64, policyMutationID string) (*pb.PolicyRule, error) {
	rule := &pb.PolicyRule{
		PolicyId:    policyMutationID,
		TenantId:    getString(snapshot, "tenant_id"),
		Name:        getString(snapshot, "name"),
		Description: getString(snapshot, "description"),
		Version:     version,
		Enabled:     getBool(snapshot, "enabled"),
	}

	if eff, ok := snapshot["effect"].(string); ok {
		switch eff {
		case "ALLOW":
			rule.Effect = pb.EffectEnum_EFFECT_ENUM_ALLOW
		case "DENY":
			rule.Effect = pb.EffectEnum_EFFECT_ENUM_DENY
		}
	}

	if p, ok := snapshot["priority"].(float64); ok {
		rule.Priority = int32(p)
	}

	if subj, ok := snapshot["subject"].(map[string]interface{}); ok {
		rule.Subject = &pb.SubjectSelector{
			Users:  getStringSlice(subj, "users"),
			Groups: getStringSlice(subj, "groups"),
			Apps:   getStringSlice(subj, "apps"),
		}
	}

	if res, ok := snapshot["resource"].(map[string]interface{}); ok {
		rule.Resource = &pb.ResourceSelector{
			AppIds:  getStringSlice(res, "app_ids"),
			Paths:   getStringSlice(res, "paths"),
			Methods: getStringSlice(res, "methods"),
		}
	}

	if conds, ok := snapshot["conditions"].(map[string]interface{}); ok {
		rule.Conditions = mapToConditions(conds)
	}

	if ca, ok := snapshot["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, ca); err == nil {
			rule.CreatedAt = timestamppb.New(t)
		}
	}

	return rule, nil
}

func conditionsToProto(c Conditions) *pb.PolicyConditions {
	if c.MFA == nil && c.Device == nil && c.Network == nil && c.Time == nil {
		return nil
	}
	pc := &pb.PolicyConditions{}
	if c.MFA != nil {
		pc.Mfa = &pb.MFACondition{Required: c.MFA.Required, MinLevel: c.MFA.MinLevel}
	}
	if c.Device != nil {
		pc.Device = &pb.DeviceCondition{Postures: c.Device.Postures}
	}
	if c.Network != nil {
		pc.Network = &pb.NetworkCondition{
			AllowedCountries: c.Network.AllowedCountries,
			BlockedCountries: c.Network.BlockedCountries,
			AllowedCidrs:     c.Network.AllowedCIDRs,
			BlockedCidrs:     c.Network.BlockedCIDRs,
			BlockTor:         c.Network.BlockTor,
		}
	}
	if c.Time != nil {
		pc.Time = &pb.TimeCondition{ScheduleName: c.Time.ScheduleName}
	}
	return pc
}

func mapToConditions(m map[string]interface{}) *pb.PolicyConditions {
	pc := &pb.PolicyConditions{}

	if mfa, ok := m["mfa"].(map[string]interface{}); ok {
		pc.Mfa = &pb.MFACondition{
			Required: getBool(mfa, "required"),
			MinLevel: getString(mfa, "min_level"),
		}
	}
	if dev, ok := m["device"].(map[string]interface{}); ok {
		pc.Device = &pb.DeviceCondition{
			Postures: getStringSlice(dev, "postures"),
		}
	}
	if net, ok := m["network"].(map[string]interface{}); ok {
		pc.Network = &pb.NetworkCondition{
			AllowedCountries: getStringSlice(net, "allowed_countries"),
			BlockedCountries: getStringSlice(net, "blocked_countries"),
			AllowedCidrs:     getStringSlice(net, "allowed_cidrs"),
			BlockedCidrs:     getStringSlice(net, "blocked_cidrs"),
			BlockTor:         getBool(net, "block_tor"),
		}
	}
	if tm, ok := m["time"].(map[string]interface{}); ok {
		pc.Time = &pb.TimeCondition{
			ScheduleName: getString(tm, "schedule_name"),
		}
	}

	if pc.Mfa == nil && pc.Device == nil && pc.Network == nil && pc.Time == nil {
		return nil
	}
	return pc
}

// ---------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getStringSlice(m map[string]interface{}, key string) []string {
	v, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(v))
	for _, item := range v {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// signer, _ := crypto.BundleSigningKeys(baseDir)
// dist := policy.NewPolicyDistributor(registry, repo, signer.(*crypto.BundleSigner), logger)

// // On policy change:
// dist.Distribute(ctx, orgID)

// // On gateway reconnect (HelloAck.needs_policy=true):
// dist.PushToGateway(ctx, conn, orgID)

// // On gateway reconnect with known version:
// dist.PushDelta(ctx, conn, orgID, conn.CurrentPolicyVersion)
