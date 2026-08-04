package policy

import (
	// "context"
	// "encoding/binary"
	// "encoding/json"
	// "fmt"
	"log/slog"
	"sync"
	// "time"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	// "github.com/google/uuid"
	// "google.golang.org/grpc/encoding/proto"
)

// ============================================================
// When a policy changes, the distributor compiles the new bundle
// and pushes it to all relevant gateways.
// ============================================================

type PolicyDistributor struct {
	registry    *registry.GatewayRegistry
	policyStore Repository
	signer      pki.CASigner
	log *slog.Logger

	// Cache of compiled bundles by tenant
	// Key: "tenant_id:version"
	bundleCache map[string]*pb.SignedPayload
	mu          sync.RWMutex
}

func NewPolicyDistributor(
	reg *registry.GatewayRegistry, 
	store Repository, 
	signer pki.CASigner,
	log *slog.Logger,
	) *PolicyDistributor {
	return &PolicyDistributor{
		registry:    reg,
		policyStore: store,
		signer:      signer,
		log: log,
		bundleCache: make(map[string]*pb.SignedPayload),
	}
}


// // LatestVersion returns the latest known policy version for a tenant.
// func (d *PolicyDistributor) LatestVersion(tenantID string) uint64 {
// 	// Query from database or in-memory tracker
// 	// Implementation depends on your version tracking
// 	return 0 // placeholder
// }
// // Distribute is called after a policy is created/updated/deleted.
// // It compiles the new bundle and sends it to all gateways for the tenant.
// func (d *PolicyDistributor) Distribute(ctx context.Context, tenantID uuid.UUID) error {
// 	// 1. Compile the latest policy bundle for this tenant
// 	bundle, version, err := d.compileBundle(ctx, tenantID)
// 	if err != nil {
// 		return fmt.Errorf("compile bundle for tenant %s: %w", tenantID, err)
// 	}

// 	// 2. Sign the bundle
// 	signed, err := d.signBundle(bundle, version)
// 	if err != nil {
// 		return fmt.Errorf("sign bundle: %w", err)
// 	}

// 	// 3. Cache the signed bundle
// 	cacheKey := fmt.Sprintf("%s:%d", tenantID, version)
// 	d.mu.Lock()
// 	d.bundleCache[cacheKey] = signed
// 	d.mu.Unlock()

// 	// 4. Find all connected gateways for this tenant
// 	gateways := d.registry.GetConnectionsForTenant(tenantID.String())
// 	if len(gateways) == 0 {
// 		d.log.Info("no connected gateways for tenant %s, bundle %d queued for later delivery", tenantID, version)
// 		// The bundle will be delivered when gateways reconnect
// 		return nil
// 	}

// 	// 5. Send to each gateway (concurrently)
// 	var wg sync.WaitGroup
// 	for _, gw := range gateways {
// 		wg.Add(1)
// 		go func(conn *registry.GatewayConn) {
// 			defer wg.Done()
// 			if err := d.pushToGateway(ctx, conn, tenantID, signed); err != nil {
// 				d.log.Info("failed to push bundle to gateway %s: %v", conn.GatewayID, err)
// 			}
// 		}(gw)
// 	}
// 	wg.Wait()

// 	return nil
// }

// // PushToGateway sends the latest policy bundle to a specific gateway.
// // Used when a gateway reconnects and is behind on policy.
// func (d *PolicyDistributor) PushToGateway(conn *registry.GatewayConn, tenantID uuid.UUID) error {
// 	ctx := context.Background()

// 	// Compile latest bundle
// 	bundle, version, err := d.compileBundle(ctx, tenantID)
// 	if err != nil {
// 		return err
// 	}

// 	signed, err := d.signBundle(bundle, version)
// 	if err != nil {
// 		return err
// 	}

// 	return d.pushToGateway(ctx, conn, tenantID, signed)
// }

// func (d *PolicyDistributor) pushToGateway(ctx context.Context, conn *registry.GatewayConn, tenantID uuid.UUID, signed *pb.SignedPayload) error {
// 	// Determine if we should send a delta or full snapshot
// 	currentVersion := conn.CurrentPolicyVersion
// 	targetVersion := signed.Version

// 	if currentVersion > 0 && currentVersion < targetVersion {
// 		// Try delta first
// 		delta, err := d.computeDelta(ctx, tenantID, currentVersion, targetVersion)
// 		if err == nil && delta != nil {
// 			msg := &pb.CPEnvelope{
// 				Payload: &pb.CPEnvelope_BundleUpdate{
// 					BundleUpdate: delta,
// 				},
// 			}
// 			return conn.Send(msg)
// 		}
// 		// Fall through to snapshot if delta computation fails
// 	}

// 	// Send full snapshot
// 	msg := &pb.CPEnvelope{
// 		Payload: &pb.CPEnvelope_BundleUpdate{
// 			BundleUpdate: signed,
// 		},
// 	}
// 	return conn.Send(msg)
// }

// func (d *PolicyDistributor) compileBundle(ctx context.Context, tenantID uuid.UUID) (*proto.PolicyBundle, uint64, error) {
// 	// Read all active policies for the tenant
// 	policies, err := d.policyStore.ListByOrg(ctx, tenantID)
// 	if err != nil {
// 		return nil, 0, err
// 	}

// 	// Get the latest mutation version for this tenant
// 	var version uint64
// 	// ... query from policy_versions table or max(policies.version)

// 	bundle := &proto.PolicyBundle{
// 		Version:  version,
// 		TenantId: tenantID,
// 		IssuedAt: uint64(time.Now().Unix()),
// 	}

// 	for _, p := range policies {
// 		rule := &pb.PolicyRule{
// 			PolicyId:    p.PolicyID,
// 			TenantId:    p.TenantID,
// 			Name:        p.Name,
// 			Effect:      pb.Effect(pb.Effect_value[string(p.Effect)]),
// 			Priority:    int32(p.Priority),
// 			Subject:     &pb.SubjectSelector{Users: p.Subject.Users, Groups: p.Subject.Groups},
// 			Resource:    &pb.ResourceSelector{Apps: p.Resource.Apps, Paths: p.Resource.Paths, Methods: p.Resource.Methods},
// 			Enabled:     p.Enabled,
// 			Version:     uint64(p.Version),
// 		}

// 		// Convert conditions
// 		if p.Conditions.MFA != nil {
// 			rule.Conditions = &pb.PolicyConditions{
// 				Mfa: &pb.MFACondition{
// 					Required: p.Conditions.MFA.Required,
// 					MinLevel: p.Conditions.MFA.MinLevel,
// 				},
// 			}
// 		}
// 		if p.Conditions.Device != nil {
// 			if rule.Conditions == nil { rule.Conditions = &pb.PolicyConditions{} }
// 			rule.Conditions.Device = &pb.DeviceCondition{Postures: p.Conditions.Device.Postures}
// 		}
// 		if p.Conditions.Network != nil {
// 			if rule.Conditions == nil { rule.Conditions = &pb.PolicyConditions{} }
// 			nc := p.Conditions.Network
// 			rule.Conditions.Network = &pb.NetworkCondition{
// 				AllowedCountries: nc.AllowedCountries,
// 				BlockedCountries: nc.BlockedCountries,
// 				AllowedCidrs:     nc.AllowedCIDRs,
// 				BlockedCidrs:     nc.BlockedCIDRs,
// 				BlockTor:         nc.BlockTor,
// 			}
// 		}
// 		if p.Conditions.Time != nil {
// 			if rule.Conditions == nil { rule.Conditions = &pb.PolicyConditions{} }
// 			rule.Conditions.Time = &pb.TimeCondition{ScheduleName: p.Conditions.Time.ScheduleName}
// 		}

// 		bundle.Rules = append(bundle.Rules, rule)
// 	}

// 	return bundle, version, nil
// }

// func (d *PolicyDistributor) signBundle(bundle *proto.PolicyBundle, version uint64) (*proto.SignedPayload, error) {
// 	// Serialize to protobuf bytes
// 	payload, err := proto.Mar(bundle)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Sign: ECDSA over (version || payload)
// 	toSign := append(binary.BigEndian.AppendUint64(nil, version), payload...)
// 	sig, err := d.signer.Sign(toSign)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &proto.SignedPayload{
// 		Version:     version,
// 		Payload:     payload,
// 		Signature:   sig,
// 		IssuedAt:    uint64(time.Now().Unix()),
// 		ExpiresAt:   uint64(time.Now().Add(24 * time.Hour).Unix()),
// 		PayloadType: "policy_bundle",
// 	}, nil
// }



// // ============================================================
// // 4. DELTA COMPUTATION (internal/cpstream/delta.go)
// // ============================================================
// // Computes the difference between two policy versions for efficient
// // delta distribution.
// // ============================================================

// func (d *PolicyDistributor) computeDelta(ctx context.Context, tenantID uuid.UUID, fromVersion, toVersion uint64) (*pb.SignedPayload, error) {
// 	if fromVersion >= toVersion {
// 		return nil, fmt.Errorf("fromVersion %d >= toVersion %d", fromVersion, toVersion)
// 	}

// 	// Query the mutation log for changes between the two versions
// 	mutations, err := d.policyStore.GetMutationsSince(ctx, tenantID, int64(fromVersion))
// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(mutations) == 0 {
// 		return nil, fmt.Errorf("no mutations found between %d and %d", fromVersion, toVersion)
// 	}

// 	// Build delta
// 	delta := &pb.PolicyDelta{
// 		FromVersion: fromVersion,
// 		ToVersion:   toVersion,
// 	}

// 	for _, m := range mutations {
// 		op := pb.PolicyMutation_OP_UNSPECIFIED
// 		switch m.Op {
// 		case "UPSERT":
// 			op = pb.PolicyMutation_OP_UPSERT
// 		case "DELETE":
// 			op = pb.PolicyMutation_OP_DELETE
// 		}

// 		mutation := &pb.PolicyMutation{
// 			Op:       op,
// 			PolicyId: m.RuleID,
// 		}

// 		if op == pb.PolicyMutation_OP_UPSERT {
// 			// Reconstruct the PolicyRule from the snapshot
// 			rule, err := d.ruleFromSnapshot(m.Snapshot)
// 			if err != nil {
// 				return nil, err
// 			}
// 			mutation.Rule = rule
// 		}

// 		delta.Mutations = append(delta.Mutations, mutation)
// 	}

// 	// Serialize and sign the delta
// 	payload, err := proto.Marshal(delta)
// 	if err != nil {
// 		return nil, err
// 	}

// 	toSign := append(binary.BigEndian.AppendUint64(nil, toVersion), payload...)
// 	sig, err := d.signer.Sign(toSign)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &pb.SignedPayload{
// 		Version:     toVersion,
// 		Payload:     payload,
// 		Signature:   sig,
// 		IssuedAt:    uint64(time.Now().Unix()),
// 		ExpiresAt:   uint64(time.Now().Add(24 * time.Hour).Unix()),
// 		PayloadType: "policy_delta",
// 	}, nil
// }


// func (d *PolicyDistributor) ruleFromSnapshot(snapshot map[string]interface{}) (*pb.PolicyRule, error) {
// 	// Convert the JSONB snapshot back to a PolicyRule protobuf
// 	// This is the inverse of the mutation log snapshot creation
// 	// Implementation: marshal to JSON, then unmarshal into PolicyRule
// 	data, err := json.Marshal(snapshot)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var rule pb.PolicyRule
// 	if err := protojson.Unmarshal(data, &rule); err != nil {
// 		return nil, err
// 	}
// 	return &rule, nil
// }


