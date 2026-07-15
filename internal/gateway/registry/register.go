package registry

import (
	"sync"
	"time"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// ConnectorEntry holds everything the gateway knows about
// one currently-connected connector.
// Lives entirely in memory. Rebuilt on every reconnect.
type ConnectorEntry struct {
	ConnectorID      string
	Apps             []*pb.ConnectorApps

	// TenantID         string
	ManagementStream ManagementStream // live stream — nil if tunnel-only entry
	ManagementConnAt time.Time
	LastHeartbeat    time.Time
	managementAttached bool
	
	//Data plane - Quic primary
	TunnelSession    TunnelSession    // live QUIC/gRPC session — nil until tunnel connects
	TunnelTransport        string // "quic", "grpc", "websocket"
	TunnelConnAt time.Time
	tunnelAttached bool
	State            string // "active", "suspended"
}

// Registry holds all live connector state for this gateway process.
// Safe for concurrent use — many goroutines read and write it
// simultaneously (CP stream handler, connector server, HTTP proxy).
//
// This is intentionally the ONLY shared mutable state connecting
// the CP-facing and connector-facing halves of the gateway.
type Registry struct {
	mu         sync.RWMutex
	connectors map[string]*ConnectorEntry // connector_id → entry
	routing    map[string]string          // subdomain → connector_id
}

func New() *Registry {
	return &Registry{
		connectors: make(map[string]*ConnectorEntry),
		routing:    make(map[string]string),
	}
}


// getOrCreate returns the existing entry for a connector or creates
// a new bare one. Called by BOTH the gRPC management server and the
// QUIC tunnel server — whichever connects FIRST creates the entry,
// whichever connects SECOND attaches to the same entry.
//
// This is the direct answer to scenario C above: reconnecting one
// plane does NOT wipe the other plane's live reference.
func (r *Registry) getOrCreate(connectorID string) *ConnectorEntry {
	if entry, ok := r.connectors[connectorID]; ok {
		return entry
	}
	entry := &ConnectorEntry{ConnectorID: connectorID}
	r.connectors[connectorID] = entry
	return entry
}


// AttachManagement is called by the gRPC connector server when a
// connector's management stream registers successfully.
func (r *Registry) AttachManagement(
	connectorID string,
	apps []*pb.ConnectorApps,
	stream ManagementStream,
	state string,
) *ConnectorEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreate(connectorID)
	entry.Apps = apps
	entry.ManagementStream = stream
	entry.ManagementConnAt = time.Now()
	entry.LastHeartbeat = time.Now()
	entry.managementAttached = true
	entry.State = state

	r.rebuildRoutingLocked(entry)
	return entry
}


// AttachTunnel is called by the QUIC (or gRPC/WS fallback) tunnel
// server when a connector's data-plane connection registers.
//
// CRITICAL: this does NOT overwrite ManagementStream if it already
// exists. It only sets the tunnel-specific fields. This is what
// makes scenario A/C correct instead of accidentally destructive.
func (r *Registry) AttachTunnel(
	connectorID string,
	session TunnelSession,
	transport string,
) *ConnectorEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreate(connectorID)
	entry.TunnelSession = session
	entry.TunnelTransport = transport
	entry.TunnelConnAt = time.Now()
	entry.tunnelAttached = true

	return entry
}

// DetachManagement is called when the gRPC stream dies.
// Does NOT remove the entry if the tunnel is still live —
// only clears the management-specific fields.
func (r *Registry) DetachManagement(connectorID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.connectors[connectorID]
	if !ok {
		return
	}
	entry.ManagementStream = nil
	entry.managementAttached = false

	r.removeIfFullyDetachedLocked(connectorID, entry)
}

// DetachTunnel is called when the QUIC connection dies.
func (r *Registry) DetachTunnel(connectorID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.connectors[connectorID]
	if !ok {
		return
	}
	entry.TunnelSession = nil
	entry.tunnelAttached = false

	r.removeIfFullyDetachedLocked(connectorID, entry)
}

// removeIfFullyDetachedLocked cleans up the entry (and routing table)
// ONLY when BOTH planes are gone. If one plane is still live, we keep
// the entry so the still-connected plane keeps working.
func (r *Registry) removeIfFullyDetachedLocked(connectorID string, entry *ConnectorEntry) {
	if entry.managementAttached || entry.tunnelAttached {
		return
	}
	for _, app := range entry.Apps {
		delete(r.routing, app.Id)
	}
	delete(r.connectors, connectorID)
}

func (r *Registry) rebuildRoutingLocked(entry *ConnectorEntry) {
	for _, app := range entry.Apps {
		r.routing[app.Id] = entry.ConnectorID
	}
}


// ── Routability — THIS is the answer to scenario A ────────────────────
//
// A connector is only routable for HTTP traffic if BOTH planes are
// attached. Management-only (tunnel not yet up) must not receive
// traffic. Tunnel-only (management dropped) is a judgment call made
// explicit below, not accidental.
func (e *ConnectorEntry) IsRoutable() bool {
	return e.tunnelAttached && e.State != "suspended"
	// NOTE: deliberately NOT requiring managementAttached here.
	// See reasoning below — this is scenario B, decided explicitly.
}

func (r *Registry) GetBySubdomain(subdomain string) (*ConnectorEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connectorID, ok := r.routing[subdomain]
	if !ok {
		return nil, false
	}
	entry := r.connectors[connectorID]
	return entry, entry != nil
}


func (r *Registry) GetByConnectorID(id string) (*ConnectorEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.connectors[id]
	return entry, ok
}

func (r *Registry) UpdateHeartbeat(connectorID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.connectors[connectorID]; ok {
		entry.LastHeartbeat = time.Now()
	}
}

func (r *Registry) SetState(connectorID, state string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.connectors[connectorID]; ok {
		entry.State = state
	}
}


// ManagementIsStale checks if heartbeat is too old to trust —
// used to decide whether to keep routing traffic during a
// management-plane blip (scenario B).
func (e *ConnectorEntry) ManagementIsStale() bool {
	if !e.managementAttached {
		return true
	}
	return time.Since(e.LastHeartbeat) > 90*time.Second
}

func (r *Registry) All() []*ConnectorEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ConnectorEntry, 0, len(r.connectors))
	for _, e := range r.connectors {
		out = append(out, e)
	}
	return out
}



// Register adds or replaces a connector entry.
// Called by ConnectorServer when a connector's management
// stream is established (registration flow).
func (r *Registry) Register(entry *ConnectorEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.connectors[entry.ConnectorID] = entry

	for _, app := range entry.Apps {
		r.routing[app.Id] = entry.ConnectorID
		// Note: routing key should be subdomain in production —
		// using app.Id here as placeholder since AppDef proto
		// (from earlier tunnel.proto) doesn't have Subdomain yet.
		// See note at end of this response.
	}
}


// Unregister removes a connector entry.
// Called when the connector's management stream closes
// (disconnect, crash, or clean shutdown).
func (r *Registry) Unregister(connectorID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.connectors[connectorID]
	if !ok {
		return
	}

	for _, app := range entry.Apps {
		delete(r.routing, app.Id)
	}
	delete(r.connectors, connectorID)
}


// Count returns the number of currently connected connectors.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.connectors)
}



