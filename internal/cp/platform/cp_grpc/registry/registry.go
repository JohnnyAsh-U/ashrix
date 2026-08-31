package registry

import (
	"context"
	"sync"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

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


// SendWithTimeout attempts to send a message on the stream with a context deadline/timeout.
// It returns an error if the send operation blocks past the context deadline.
func (g *GatewayConn) SendWithTimeout(ctx context.Context, msg *pb.CPEnvelope) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- g.Stream.Send(msg)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-g.Ctx.Done():
		return g.Ctx.Err()
	}
}

