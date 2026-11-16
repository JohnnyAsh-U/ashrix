package tunnel

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func newTestTunnel() *QuicTunnel {
	return &QuicTunnel{
		connectors: map[string]*ConnectorSession{
			"connector-a": {ConnectorID: "connector-a", CertSerial: "01"},
		},
		routing: map[string]string{
			"app.example.test": "connector-a",
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestGetConnectorSessionBySubdomainReturnsSnapshot(t *testing.T) {
	tunnel := newTestTunnel()
	original := time.Now().Add(-time.Minute)
	tunnel.connectors["connector-a"].LastSeen = original

	snapshot, ok := tunnel.GetConnectorSessionBySubdomain("app.example.test")
	if !ok {
		t.Fatal("expected configured application route to resolve")
	}
	if snapshot.ConnectorID != "connector-a" {
		t.Fatalf("connector ID = %q, want connector-a", snapshot.ConnectorID)
	}

	snapshot.LastSeen = time.Now()
	if !tunnel.connectors["connector-a"].LastSeen.Equal(original) {
		t.Fatal("mutating returned snapshot changed registry state")
	}
}

func TestGetConnectorSessionBySubdomainRejectsUnknownRoute(t *testing.T) {
	if _, ok := newTestTunnel().GetConnectorSessionBySubdomain("missing.example.test"); ok {
		t.Fatal("unknown route unexpectedly resolved")
	}
}

func TestGetConnectorSessionBySubdomainRejectsMissingConnectorEntry(t *testing.T) {
	tunnel := newTestTunnel()
	delete(tunnel.connectors, "connector-a")

	if _, ok := tunnel.GetConnectorSessionBySubdomain("app.example.test"); ok {
		t.Fatal("route with missing connector entry unexpectedly resolved")
	}
}

func TestAttachTunnelRejectsUnknownConnector(t *testing.T) {
	if got := newTestTunnel().AttachTunnel("missing", nil, nil); got != nil {
		t.Fatal("AttachTunnel returned a session for an unknown connector")
	}
}

func TestAttachTunnelInitializesSessionState(t *testing.T) {
	tunnel := newTestTunnel()
	before := time.Now()

	conn := &quic.Conn{}
	snapshot := tunnel.AttachTunnel("connector-a", conn, nil)
	if snapshot == nil {
		t.Fatal("AttachTunnel returned nil for configured connector")
	}
	entry := tunnel.connectors["connector-a"]
	if !entry.tunnelAttached {
		t.Fatal("session was not marked attached")
	}
	if entry.Conn != conn {
		t.Fatal("active connection was not stored")
	}
	if entry.LastSeen.Before(before) || entry.tunnelConnectedAt.Before(before) {
		t.Fatal("session timestamps were not initialized")
	}
}

func TestDetachTunnelIgnoresStaleConnection(t *testing.T) {
	tunnel := newTestTunnel()
	current := &quic.Conn{}
	stale := &quic.Conn{}
	entry := tunnel.connectors["connector-a"]
	entry.Conn = current
	entry.tunnelAttached = true

	tunnel.DetachTunnel("connector-a", stale)

	if entry.Conn != current {
		t.Fatal("stale connection detached the current session")
	}
	if !entry.tunnelAttached {
		t.Fatal("stale connection changed attached state")
	}
}

func TestDetachTunnelClearsCurrentSession(t *testing.T) {
	tunnel := newTestTunnel()
	current := &quic.Conn{}
	entry := tunnel.connectors["connector-a"]
	entry.Conn = current
	entry.tunnelAttached = true

	tunnel.DetachTunnel("connector-a", current)

	if entry.Conn != nil {
		t.Fatal("current connection was not cleared")
	}
	if entry.MgmtStream != nil {
		t.Fatal("management stream was not cleared")
	}
	if entry.tunnelAttached {
		t.Fatal("session remains marked attached")
	}
}

func TestIsConnectorTunnelAttached(t *testing.T) {
	tunnel := newTestTunnel()
	if tunnel.IsConnectorTunnelAttached("connector-a") {
		t.Fatal("connector without a connection reported attached")
	}
	if tunnel.IsConnectorTunnelAttached("missing") {
		t.Fatal("unknown connector reported attached")
	}

	tunnel.connectors["connector-a"].Conn = &quic.Conn{}
	if !tunnel.IsConnectorTunnelAttached("connector-a") {
		t.Fatal("connector with a connection reported detached")
	}
}

func TestGetConnectorTunnel(t *testing.T) {
	tunnel := newTestTunnel()
	if _, ok := tunnel.GetConnectorTunnel("connector-a"); ok {
		t.Fatal("connector without a connection unexpectedly returned one")
	}
	if _, ok := tunnel.GetConnectorTunnel("missing"); ok {
		t.Fatal("unknown connector unexpectedly returned a connection")
	}

	conn := &quic.Conn{}
	tunnel.connectors["connector-a"].Conn = conn
	got, ok := tunnel.GetConnectorTunnel("connector-a")
	if !ok || got != conn {
		t.Fatal("GetConnectorTunnel did not return the active connection")
	}
}

func TestDetachUnknownConnectorDoesNotPanic(t *testing.T) {
	newTestTunnel().DetachTunnel("missing", &quic.Conn{})
}
