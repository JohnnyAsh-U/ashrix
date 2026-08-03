package policy

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"time"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
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
	bundleCache map[string]*proto.SignedPayload
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
		bundleCache: make(map[string]*proto.SignedPayload),
	}
}


// LatestVersion returns the latest known policy version for a tenant.
func (d *PolicyDistributor) LatestVersion(tenantID string) uint64 {
	// Query from database or in-memory tracker
	// Implementation depends on your version tracking
	return 0 // placeholder
}
// Distribute is called after a policy is created/updated/deleted.
// It compiles the new bundle and sends it to all gateways for the tenant.
func (d *PolicyDistributor) Distribute(ctx context.Context, tenantID string) error {
	// 1. Compile the latest policy bundle for this tenant
	bundle, version, err := d.compileBundle(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("compile bundle for tenant %s: %w", tenantID, err)
	}

	// 2. Sign the bundle
	signed, err := d.signBundle(bundle, version)
	if err != nil {
		return fmt.Errorf("sign bundle: %w", err)
	}

	// 3. Cache the signed bundle
	cacheKey := fmt.Sprintf("%s:%d", tenantID, version)
	d.mu.Lock()
	d.bundleCache[cacheKey] = signed
	d.mu.Unlock()

	// 4. Find all connected gateways for this tenant
	gateways := d.registry.GetConnectionsForTenant(tenantID)
	if len(gateways) == 0 {
		d.log.Info("no connected gateways for tenant %s, bundle %d queued for later delivery", tenantID, version)
		// The bundle will be delivered when gateways reconnect
		return nil
	}

	// 5. Send to each gateway (concurrently)
	var wg sync.WaitGroup
	for _, gw := range gateways {
		wg.Add(1)
		go func(conn *GatwayConn) {
			defer wg.Done()
			if err := d.pushToGateway(ctx, conn, tenantID, signed); err != nil {
				d.log.Info("failed to push bundle to gateway %s: %v", conn.GatewayID, err)
			}
		}(gw)
	}
	wg.Wait()

	return nil
}

// PushToGateway sends the latest policy bundle to a specific gateway.
// Used when a gateway reconnects and is behind on policy.
func (d *PolicyDistributor) PushToGateway(conn *GatewayConn, tenantID string) error {
	ctx := context.Background()

	// Compile latest bundle
	bundle, version, err := d.compileBundle(ctx, tenantID)
	if err != nil {
		return err
	}

	signed, err := d.signBundle(bundle, version)
	if err != nil {
		return err
	}

	return d.pushToGateway(ctx, conn, tenantID, signed)
}

func (d *PolicyDistributor) pushToGateway(ctx context.Context, conn *GatewayConn, tenantID string, signed *pb.SignedPayload) error {
	// Determine if we should send a delta or full snapshot
	currentVersion := conn.CurrentPolicyVersion
	targetVersion := signed.Version

	if currentVersion > 0 && currentVersion < targetVersion {
		// Try delta first
		delta, err := d.computeDelta(ctx, tenantID, currentVersion, targetVersion)
		if err == nil && delta != nil {
			msg := &pb.CPMessage{
				Payload: &pb.CPMessage_SignedDelta{
					SignedDelta: delta,
				},
			}
			return conn.Send(msg)
		}
		// Fall through to snapshot if delta computation fails
	}

	// Send full snapshot
	msg := &pb.CPMessage{
		Payload: &pb.CPMessage_SignedPayload{
			SignedPayload: signed,
		},
	}
	return conn.Send(msg)
}

func (d *PolicyDistributor) compileBundle(ctx context.Context, tenantID string) (*proto.PolicyBundle, uint64, error) {
	// Read all active policies for the tenant
	policies, err := d.policyStore.ListByOrg(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	// Get the latest mutation version for this tenant
	var version uint64
	// ... query from policy_versions table or max(policies.version)

	bundle := &proto.PolicyBundle{
		Version:  version,
		TenantId: tenantID,
		IssuedAt: uint64(time.Now().Unix()),
	}

	for _, p := range policies {
		rule := &pb.PolicyRule{
			PolicyId:    p.PolicyID,
			TenantId:    p.TenantID,
			Name:        p.Name,
			Effect:      pb.Effect(pb.Effect_value[string(p.Effect)]),
			Priority:    int32(p.Priority),
			Subject:     &pb.SubjectSelector{Users: p.Subject.Users, Groups: p.Subject.Groups},
			Resource:    &pb.ResourceSelector{Apps: p.Resource.Apps, Paths: p.Resource.Paths, Methods: p.Resource.Methods},
			Enabled:     p.Enabled,
			Version:     uint64(p.Version),
		}

		// Convert conditions
		if p.Conditions.MFA != nil {
			rule.Conditions = &pb.PolicyConditions{
				Mfa: &pb.MFACondition{
					Required: p.Conditions.MFA.Required,
					MinLevel: p.Conditions.MFA.MinLevel,
				},
			}
		}
		if p.Conditions.Device != nil {
			if rule.Conditions == nil { rule.Conditions = &pb.PolicyConditions{} }
			rule.Conditions.Device = &pb.DeviceCondition{Postures: p.Conditions.Device.Postures}
		}
		if p.Conditions.Network != nil {
			if rule.Conditions == nil { rule.Conditions = &pb.PolicyConditions{} }
			nc := p.Conditions.Network
			rule.Conditions.Network = &pb.NetworkCondition{
				AllowedCountries: nc.AllowedCountries,
				BlockedCountries: nc.BlockedCountries,
				AllowedCidrs:     nc.AllowedCIDRs,
				BlockedCidrs:     nc.BlockedCIDRs,
				BlockTor:         nc.BlockTor,
			}
		}
		if p.Conditions.Time != nil {
			if rule.Conditions == nil { rule.Conditions = &pb.PolicyConditions{} }
			rule.Conditions.Time = &pb.TimeCondition{ScheduleName: p.Conditions.Time.ScheduleName}
		}

		bundle.Rules = append(bundle.Rules, rule)
	}

	return bundle, version, nil
}

func (d *PolicyDistributor) signBundle(bundle *pb.PolicyBundle, version uint64) (*pb.SignedPayload, error) {
	// Serialize to protobuf bytes
	payload, err := proto.Marshal(bundle)
	if err != nil {
		return nil, err
	}

	// Sign: ECDSA over (version || payload)
	toSign := append(binary.BigEndian.AppendUint64(nil, version), payload...)
	sig, err := d.signer.Sign(toSign)
	if err != nil {
		return nil, err
	}

	return &proto.SignedPayload{
		Version:     version,
		Payload:     payload,
		Signature:   sig,
		IssuedAt:    uint64(time.Now().Unix()),
		ExpiresAt:   uint64(time.Now().Add(24 * time.Hour).Unix()),
		PayloadType: "policy_bundle",
	}, nil
}


