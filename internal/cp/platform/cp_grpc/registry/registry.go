package registry

import (
	"context"
	"sync"
	"time"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)



// GatewayConn represents an active gRPC connection to a gateway.
type GatewayConn struct {
	GatewayID   string
	TenantID    string       // For self-hosted: the single tenant. For hosted: "" (looked up per-request)
	// GatewayType GatewayType
	Stream      pb.ControlPlaneService_ConnectServer // The bidirectional gRPC stream
	ConnectedAt time.Time
	LastSeen    time.Time

	// Current state known from the gateway
	CurrentPolicyVersion uint64
	CurrentTrustVersion  uint64

	// Control
	Ctx    context.Context
	Cancel context.CancelFunc
}

// Send sends a CPMessage to this gateway over its active stream.
func (g *GatewayConn) Send(msg *pb.CPEnvelope) error {
	return g.Stream.Send(msg)
}

// GatewayRegistry tracks all active gateway connections.
type GatewayRegistry struct {
	// All connections by gateway_id
	byID map[string]*GatewayConn

	byTenant map[string]map[string]*GatewayConn // tenant_id -> gateway_id -> conn

	mu sync.RWMutex
}

func NewGatewayRegistry() *GatewayRegistry {
	return &GatewayRegistry{
		byID:               make(map[string]*GatewayConn),
		byTenant:           make(map[string]map[string]*GatewayConn),
	}
}


// Register adds a new gateway connection to the registry.
// Called when a gateway opens a ControlStream.
func (r *GatewayRegistry) Register(conn *GatewayConn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If this gateway_id already exists, close the old connection
	if old, ok := r.byID[conn.GatewayID]; ok {
		old.Cancel() // Signal the old stream handler to exit
		r.removeLocked(old)
	}

	r.byID[conn.GatewayID] = conn

	if r.byTenant[conn.TenantID] == nil {
		r.byTenant[conn.TenantID] = make(map[string]*GatewayConn)
	}
	r.byTenant[conn.TenantID][conn.GatewayID] = conn
}

// HandleHello is called when a hosted gateway sends its initial Hello message.
// It tells the CP which tenants this gateway instance serves.
func (r *GatewayRegistry) HandleHello(gatewayID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, ok := r.byID[gatewayID]
	if !ok {
		return
	}

	conn, ok = r.byTenant[conn.TenantID][gatewayID]
	if !ok {
		return
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

	//Delete from tenant mapping
	if tenantMap, exists := r.byTenant[conn.TenantID]; exists {
		delete(tenantMap, conn.GatewayID)
		if len(tenantMap) == 0 {
			delete(r.byTenant, conn.TenantID)
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

	if tenantMap, exists := r.byTenant[tenantID]; exists {
		for _, conn := range tenantMap {
			conns = append(conns, conn)
			seen[conn.GatewayID] = true
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

