package tunnel

import (
	"context"
	"crypto/x509"
	"fmt"

	// "fmt"
	"log/slog"
	"sync"
	"time"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/flow"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/quic"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

type TunnelSession interface {
	OpenStream(ctx context.Context, req *pb.StreamFrame) (Stream, error)
	Close() error
}

// ConnectorEntry holds everything the gateway knows about
// one currently-connected connector.
// Lives entirely in memory. Rebuilt on every reconnect.
type ConnectorEntry struct {
	ConnectorID string
	Apps        []*pb.ConnectorApps

	//Data plane - Quic primary
	TunnelSession   TunnelSession // live QUIC session — nil until tunnel connects
	TunnelConnAt    time.Time
	tunnelAttached  bool
	ActiveStreams   int64
}

// Registry holds all live connector state for this gateway process.
// Safe for concurrent use — many goroutines read and write it
// simultaneously (CP stream handler, connector server, HTTP proxy).
//

type Registry struct {
	mu              sync.RWMutex
	connectors      map[string]*ConnectorEntry // connector_id → entry
	routing         map[string]string          // subdomain → connector_id
	authConnectors  map[string]string // connector_id -> status
	log             *slog.Logger
}

func NewTunnelRegistry(log *slog.Logger) *Registry {
	return &Registry{
		connectors:      make(map[string]*ConnectorEntry),
		routing:         make(map[string]string), //Subdomain to connectors
		authConnectors:  make(map[string]string),
		log:             log,
	}
}



func (r *Registry) SetAuthorizedConnectors(connectors map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authConnectors = connectors
}

func (r *Registry) IsConnectorAuthorized(connectorID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.authConnectors == nil {
		return "", false
	}
	status, exists := r.authConnectors[connectorID]
	return status, exists
}

func (r *Registry) getOrCreateLocked(connectorID string) *ConnectorEntry {
	if entry, ok := r.connectors[connectorID]; ok {
		return entry
	}
	entry := &ConnectorEntry{ConnectorID: connectorID}
	r.connectors[connectorID] = entry
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
	cert *x509.Certificate,
	transport string,
	apps []*pb.ConnectorApps,
) *ConnectorEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreateLocked(connectorID)
	oldTunnel := entry.TunnelSession

	fmt.Println(connectorID)
	fmt.Println(transport)

	entry.Apps = apps
	entry.TunnelSession = session
	entry.TunnelConnAt = time.Now()
	entry.tunnelAttached = true

	r.rebuildRoutingLocked(entry)


	// Displace old tunnel session outside lock
	if oldTunnel != nil && oldTunnel != session {
		r.log.Info("Displacing old tunnel session",
			"connector_id", connectorID,
			"reason",
		)
		_ = oldTunnel.Close()
	}
	return entry
}


// DetachTunnel is called when the QUIC connection dies.
func (r *Registry) DetachTunnel(connectorID string, session TunnelSession) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.connectors[connectorID]
	if !ok || entry.TunnelSession != session {
		return // Ignore stale detach from a displaced session
	}

	entry.TunnelSession = nil
	entry.tunnelAttached = false
	r.removeIfFullyDetachedLocked(connectorID, entry)
}

// removeIfFullyDetachedLocked cleans up the entry (and routing table)
// ONLY when BOTH planes are gone. If one plane is still live, we keep
// the entry so the still-connected plane keeps working.
func (r *Registry) removeIfFullyDetachedLocked(connectorID string, entry *ConnectorEntry) {
	if entry.tunnelAttached {
		return
	}
	for _, app := range entry.Apps {
		delete(r.routing, app.Name)
	}
	delete(r.connectors, connectorID)
}

func (r *Registry) rebuildRoutingLocked(entry *ConnectorEntry) {
	for _, app := range entry.Apps {
		r.routing[app.Name] = entry.ConnectorID
	}
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

func (r *Registry) GetByAppID(appID string) (*ConnectorEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, entry := range r.connectors {
		for _, app := range entry.Apps {
			if app.Id == appID {
				return entry, true
			}
		}
	}
	return nil, false
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


func (r *Registry) ActiveConnectors() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.connectors)
}

func (r *Registry) ForceCloseConnector(connectorID string) {
	r.mu.RLock()
	entry, ok := r.connectors[connectorID]

	if !ok {
		r.mu.RUnlock()
		return
	}

	tunnel := entry.TunnelSession
	r.mu.RUnlock()

	r.log.Warn("Force disconnecting connector",
		"connector_id", connectorID,
	)

	if tunnel != nil {
		_ = tunnel.Close()
	}
}

// Close And Detach Management And Tunnel Session
func (r *Registry) CloseAllConnectorConnection() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.connectors {
		if entry.TunnelSession != nil {
			_ = entry.TunnelSession.Close()
			entry.tunnelAttached = false
			entry.TunnelSession = nil
		}
	}
}



func (e *ConnectorEntry) IsTunnelAttached() bool {
	return e.TunnelSession != nil
}


func (r *Registry) GetTunnelSession(connectorID string) (TunnelSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.connectors[connectorID]
	if !ok {
		return nil, false
	}
	return entry.TunnelSession, entry.TunnelSession != nil
}

