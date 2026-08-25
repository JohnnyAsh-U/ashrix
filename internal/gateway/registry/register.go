package registry

import (
	"crypto/x509"
	"fmt"
	"sync"
	"time"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)

// ConnectorEntry holds everything the gateway knows about
// one currently-connected connector.
// Lives entirely in memory. Rebuilt on every reconnect.
type ConnectorEntry struct {
	ConnectorID string
	Apps        []*pb.ConnectorApps

	TenantID           string
	ManagementSession  *ManagementSession // live stream — nil if tunnel-only entry
	ManagementConnAt   time.Time
	LastHeartbeat      time.Time
	managementAttached bool

	//Data plane - Quic primary
	TunnelSession   TunnelSession // live QUIC/gRPC session — nil until tunnel connects
	TunnelTransport string        // "quic", "grpc", "websocket"
	TunnelConnAt    time.Time
	tunnelAttached  bool
	State           string // "active", "suspended"

	gRPCCert *x509.Certificate
	quicCert *x509.Certificate
}

// Registry holds all live connector state for this gateway process.
// Safe for concurrent use — many goroutines read and write it
// simultaneously (CP stream handler, connector server, HTTP proxy).
//
// This is intentionally the ONLY shared mutable state connecting
// the CP-facing and connector-facing halves of the gateway.
type Registry struct {
	mu             sync.RWMutex
	connectors     map[string]*ConnectorEntry // connector_id → entry
	routing        map[string]string          // subdomain → connector_id
	crlSerials     map[string]struct{}
	authConnectors map[string]string // connector_id -> status
	log            *zap.Logger
}

func New(log *zap.Logger) *Registry {
	return &Registry{
		connectors:     make(map[string]*ConnectorEntry),
		routing:        make(map[string]string), //Subdomain to connectors
		crlSerials:     make(map[string]struct{}),
		authConnectors: make(map[string]string),
		log:            log,
	}
}

// SetCrlEntries converts the serial slice to an O(1) lookup map.
func (r *Registry) SetCrlEntries(serials []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.crlSerials = make(map[string]struct{}, len(serials))
	for _, s := range serials {
		r.crlSerials[s] = struct{}{}
	}
}

func (r *Registry) IsCrlRevoked(serialNumber string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, revoked := r.crlSerials[serialNumber]
	return revoked
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

// AttachManagement is called by the gRPC connector server when a
// connector's management stream registers successfully.
func (r *Registry) AttachManagement(
	connectorID string,
	TenantID string,
	apps []*pb.ConnectorApps,
	session *ManagementSession,
	cert *x509.Certificate,
	state string,
) *ConnectorEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreateLocked(connectorID)
	oldSession := entry.ManagementSession

	entry.Apps = apps
	entry.TenantID = TenantID
	entry.ManagementSession = session
	entry.ManagementConnAt = time.Now()
	entry.LastHeartbeat = time.Now()
	entry.managementAttached = true
	entry.gRPCCert = cert
	entry.State = state

	r.rebuildRoutingLocked(entry)

	// Displace old management stream outside lock
	if oldSession != nil && oldSession != session {
		r.log.Info("Displacing old management session",
			zap.String("connector_id", connectorID),
			zap.String("reason", string(DisconnectReplaced)),
		)
		oldSession.Close()
	}

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
) *ConnectorEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreateLocked(connectorID)
	oldTunnel := entry.TunnelSession

	entry.TunnelSession = session
	entry.TunnelTransport = transport
	entry.TunnelConnAt = time.Now()
	entry.tunnelAttached = true
	entry.quicCert = cert

	// Displace old tunnel session outside lock
	if oldTunnel != nil && oldTunnel != session {
		r.log.Info("Displacing old tunnel session",
			zap.String("connector_id", connectorID),
			zap.String("reason", string(DisconnectReplaced)),
		)
		_ = oldTunnel.Close()
	}
	return entry
}

// DetachManagement is called when the gRPC stream dies.
// Does NOT remove the entry if the tunnel is still live —
// only clears the management-specific fields.
func (r *Registry) DetachManagement(connectorID string, session *ManagementSession) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.connectors[connectorID]
	if !ok || entry.ManagementSession != session {
		return // Ignore stale detach from a displaced session
	}

	entry.ManagementSession = nil
	entry.managementAttached = false
	r.removeIfFullyDetachedLocked(connectorID, entry)
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
	if entry.managementAttached || entry.tunnelAttached {
		return
	}
	for _, app := range entry.Apps {
		delete(r.routing, app.Subdomain)
	}
	delete(r.connectors, connectorID)
}

func (r *Registry) rebuildRoutingLocked(entry *ConnectorEntry) {
	for _, app := range entry.Apps {
		r.routing[app.Subdomain] = entry.ConnectorID
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

func (r *Registry) GetAppByID(appID string) (*pb.ConnectorApps, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, entry := range r.connectors {
		for _, app := range entry.Apps {
			if app.Id == appID {
				return app, true
			}
		}
	}
	return nil, false
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

// When Gateway connects or reconnects to cp, verify the
// connectors connected and remove unauthorized connectors
// Check the certificates on CRL list, if present remove the connectors
func (r *Registry) VerifyGatewayConnections() {
	// get authconnectors and connectors and compare
	//Remove the connectors which are not present in the authconnectors
	r.mu.RLock()
	// This function is called when CP comes up
	fmt.Println("Gateway Verifying connections...")
	fmt.Println("Auth connectors:", r.authConnectors)
	var toClose []string

	for connectorID, entry := range r.connectors {
		authorized := false
		if _, ok := r.authConnectors[connectorID]; ok {
			authorized = true
		}
		revoked := r.connectorRevokedLocked(entry)

		if !authorized || revoked {
			toClose = append(toClose, connectorID)
		}
	}
	r.mu.RUnlock()

	for _, connectorID := range toClose {
		r.ForceCloseConnector(connectorID)
	}
	fmt.Println("Gateway Connections Verified")
}

func (r *Registry) ActiveConnectors() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.connectors)
}

func (r *Registry) ActiveSessions() int {
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

	mgmt := entry.ManagementSession
	tunnel := entry.TunnelSession
	r.mu.RUnlock()

	r.log.Warn("Force disconnecting connector",
		zap.String("connector_id", connectorID),
	)

	// Close network transports outside lock
	if mgmt != nil {
		mgmt.Close()
	}
	if tunnel != nil {
		_ = tunnel.Close()
	}
}

func (e *ConnectorEntry) IsManagementAttached() bool {
	return e.ManagementSession != nil
}

func (e *ConnectorEntry) IsTunnelAttached() bool {
	return e.TunnelSession != nil
}

func (r *Registry) connectorRevokedLocked(entry *ConnectorEntry) bool {
	if entry.gRPCCert != nil {
		if _, revoked := r.crlSerials[entry.gRPCCert.SerialNumber.String()]; revoked {
			return true
		}
	}
	if entry.quicCert != nil {
		if _, revoked := r.crlSerials[entry.quicCert.SerialNumber.String()]; revoked {
			return true
		}
	}
	return false
}

func (r *Registry) GetTunnelSession(connectorID string) (TunnelSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.connectors[connectorID]
	if !ok {
		return nil, false
	}
	if !entry.IsRoutable() {
		return nil, false
	}
	return entry.TunnelSession, entry.TunnelSession != nil
}
