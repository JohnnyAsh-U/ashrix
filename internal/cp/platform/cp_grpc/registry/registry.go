package registry

import (
	"context"
	// "fmt"
	"sync"
	"time"

	// "google.golang.org/grpc"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// GatewayType distinguishes Ashrix-hosted from self-hosted gateways.
type GatewayType string

const (
	GatewayTypeHosted   GatewayType = "ashrix_hosted"   // Ashrix-managed, multi-tenant
	GatewayTypeSelfHosted GatewayType = "self_hosted" // Customer-managed, single-tenant
)

// GatewayConn represents an active gRPC connection to a gateway.
type GatewayConn struct {
	GatewayID   string
	TenantID    string       // For self-hosted: the single tenant. For hosted: "" (looked up per-request)
	GatewayType GatewayType
	Stream      pb.ControlPlaneService_ConnectServer // The bidirectional gRPC stream
	ConnectedAt time.Time
	LastSeen    time.Time

	// Current state known from the gateway
	CurrentPolicyVersion uint64
	CurrentTrustVersion  uint64

	// Control
	ctx    context.Context
	cancel context.CancelFunc
}

// Send sends a CPMessage to this gateway over its active stream.
func (g *GatewayConn) Send(msg *pb.CPEnvelope) error {
	return g.Stream.Send(msg)
}

// GatewayRegistry tracks all active gateway connections.
type GatewayRegistry struct {
	// All connections by gateway_id
	byID map[string]*GatewayConn

	// Hosted gateways indexed by tenant (a hosted gateway may serve multiple tenants)
	hostedByTenant map[string]map[string]*GatewayConn // tenant_id -> gateway_id -> conn

	// Self-hosted gateways by tenant (one gateway per tenant typically)
	selfHostedByTenant map[string]*GatewayConn // tenant_id -> conn

	mu sync.RWMutex
}

func NewGatewayRegistry() *GatewayRegistry {
	return &GatewayRegistry{
		byID:               make(map[string]*GatewayConn),
		hostedByTenant:     make(map[string]map[string]*GatewayConn),
		selfHostedByTenant: make(map[string]*GatewayConn),
	}
}


// Register adds a new gateway connection to the registry.
// Called when a gateway opens a ControlStream.
func (r *GatewayRegistry) Register(conn *GatewayConn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If this gateway_id already exists, close the old connection
	if old, ok := r.byID[conn.GatewayID]; ok {
		old.cancel() // Signal the old stream handler to exit
		r.removeLocked(old)
	}

	r.byID[conn.GatewayID] = conn

	switch conn.GatewayType {
	case GatewayTypeSelfHosted:
		// Self-hosted gateway serves exactly one tenant
		r.selfHostedByTenant[conn.TenantID] = conn

	case GatewayTypeHosted:
		// Hosted gateway: we need to know which tenants it serves.
		// This comes from the gateway's bootstrap config or from a database lookup.
		// For now, we add it to a "pending" list and resolve tenants later
		// when the gateway sends its Hello message.
		// See: HandleHello() below
	}
}

// HandleHello is called when a hosted gateway sends its initial Hello message.
// It tells the CP which tenants this gateway instance serves.
func (r *GatewayRegistry) HandleHello(gatewayID string, tenants []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, ok := r.byID[gatewayID]
	if !ok {
		return
	}

	for _, tenantID := range tenants {
		if r.hostedByTenant[tenantID] == nil {
			r.hostedByTenant[tenantID] = make(map[string]*GatewayConn)
		}
		r.hostedByTenant[tenantID][gatewayID] = conn
	}
}


// Unregister removes a gateway connection.
// Called when the stream dies or the gateway disconnects.
func (r *GatewayRegistry) Unregister(gatewayID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, ok := r.byID[gatewayID]; ok {
		r.removeLocked(conn)
	}
}

func (r *GatewayRegistry) removeLocked(conn *GatewayConn) {
	delete(r.byID, conn.GatewayID)

	if conn.GatewayType == GatewayTypeSelfHosted {
		delete(r.selfHostedByTenant, conn.TenantID)
		return
	}

	// For hosted gateways, remove from all tenant mappings
	for tenantID, gateways := range r.hostedByTenant {
		delete(gateways, conn.GatewayID)
		if len(gateways) == 0 {
			delete(r.hostedByTenant, tenantID)
		}
	}
}


// GetConnectionsForTenant returns all connected gateways that serve a given tenant.
// This includes:
//   - All hosted gateways configured for this tenant
//   - The self-hosted gateway for this tenant (if any)
func (r *GatewayRegistry) GetConnectionsForTenant(tenantID string) []*GatewayConn {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var conns []*GatewayConn
	seen := make(map[string]bool)

	// Hosted gateways for this tenant
	if hosted, ok := r.hostedByTenant[tenantID]; ok {
		for _, conn := range hosted {
			if !seen[conn.GatewayID] {
				conns = append(conns, conn)
				seen[conn.GatewayID] = true
			}
		}
	}

	// Self-hosted gateway for this tenant
	if self, ok := r.selfHostedByTenant[tenantID]; ok {
		if !seen[self.GatewayID] {
			conns = append(conns, self)
		}
	}

	return conns
}

// GetConnection returns a single gateway connection by ID.
func (r *GatewayRegistry) GetConnection(gatewayID string) (*GatewayConn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	conn, ok := r.byID[gatewayID]
	return conn, ok
}

// Count returns the total number of connected gateways.
func (r *GatewayRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

