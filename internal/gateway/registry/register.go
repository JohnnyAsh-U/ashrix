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
	// TenantID         string
	ManagementStream ManagementStream // live stream — nil if tunnel-only entry
	TunnelSession    TunnelSession    // live QUIC/gRPC session — nil until tunnel connects
	Apps             []*pb.AppDef
	Transport        string // "quic", "grpc", "websocket"
	State            string // "active", "suspended"
	ConnectedAt      time.Time
	LastHeartbeat    time.Time
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




// GetByConnectorID returns the entry for a connector, if connected.
// Used by CPStreamHandler to relay commands.
func (r *Registry) GetByConnectorID(id string) (*ConnectorEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.connectors[id]
	return entry, ok
}

// GetBySubdomain returns the connector serving a given app subdomain.
// Used by the HTTP proxy handler to route user requests.
func (r *Registry) GetBySubdomain(subdomain string) (*ConnectorEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connectorID, ok := r.routing[subdomain]
	if !ok {
		return nil, false
	}
	entry, ok := r.connectors[connectorID]
	return entry, ok
}


// UpdateHeartbeat refreshes the last-seen timestamp for a connector.
func (r *Registry) UpdateHeartbeat(connectorID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.connectors[connectorID]; ok {
		entry.LastHeartbeat = time.Now()
	}
}

// SetState updates a connector's state (e.g. "active" → "suspended").
// Does NOT touch the stream — caller is responsible for actually
// sending the suspend command separately. This just updates the
// registry's view of truth, used for policy checks and status reporting.
func (r *Registry) SetState(connectorID, state string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.connectors[connectorID]; ok {
		entry.State = state
	}
}


// IsAlive checks whether a connector's heartbeat is recent enough to trust.
func (r *Registry) IsAlive(connectorID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.connectors[connectorID]
	if !ok {
		return false
	}
	return time.Since(entry.LastHeartbeat) < 60*time.Second
}

// All returns a snapshot slice of all currently connected connectors.
// Used for gateway heartbeat to CP (active_connectors count)
// and status/health endpoints.
//
// Returns a COPY of the slice header, not the underlying entries —
// callers must not mutate returned entries directly. This matters:
// see the vulnerability note below.
func (r *Registry) All() []*ConnectorEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*ConnectorEntry, 0, len(r.connectors))
	for _, e := range r.connectors {
		out = append(out, e)
	}
	return out
}


// Count returns the number of currently connected connectors.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.connectors)
}



