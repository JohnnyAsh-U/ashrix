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

	sendMu sync.Mutex

	ackMu sync.Mutex

	ackWaiters map[int64]chan error
}

func NewGatewayConn(
	ctx context.Context,
	cancel context.CancelFunc,
	stream pb.ControlPlaneService_ConnectServer,
	gatewayID string,
	tenantID string,
	policyVersion uint64,
) *GatewayConn {

	return &GatewayConn{
		GatewayID: gatewayID,
		TenantID:  tenantID,
		Stream:    stream,

		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),

		CurrentPolicyVersion: policyVersion,

		Ctx:    ctx,
		Cancel: cancel,

		ackWaiters: make(map[int64]chan error),
	}
}

// Send sends a CPMessage to this gateway over its active stream.
func (c *GatewayConn) Send(
	envelope *pb.CPEnvelope,
) error {

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	return c.Stream.Send(envelope)
}

func (c *GatewayConn) RegisterAck(
	seq int64,
) <-chan error {

	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	ch := make(chan error, 1)

	c.ackWaiters[seq] = ch

	return ch
}


func (c *GatewayConn) ResolveAck(
	seq int64,
	err error,
) {

	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	ch, exists :=
		c.ackWaiters[seq]

	if !exists {
		return
	}

	delete(
		c.ackWaiters,
		seq,
	)

	ch <- err
	close(ch)
}


func (c *GatewayConn) ResolveThrough(
	seq int64,
	err error,
) {

	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	for waitingSeq, ch :=
		range c.ackWaiters {

		if waitingSeq > seq {
			continue
		}

		delete(
			c.ackWaiters,
			waitingSeq,
		)

		ch <- err
		close(ch)
	}
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

