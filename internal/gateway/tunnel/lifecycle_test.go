package tunnel

import (
	"context"
	"testing"
	"time"
)

func waitForContextDone(t *testing.T, ctx context.Context, description string) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func TestAttachTunnelClosesReplacedConnection(t *testing.T) {
	oldClient, oldServer := gatewayTestQUICPair(t)
	newClient, _ := gatewayTestQUICPair(t)
	tunnel := newTestTunnel()

	if tunnel.AttachTunnel("connector-a", oldClient, nil) == nil {
		t.Fatal("failed to attach initial connection")
	}
	if tunnel.AttachTunnel("connector-a", newClient, nil) == nil {
		t.Fatal("failed to attach replacement connection")
	}

	waitForContextDone(t, oldServer.Context(), "old peer connection to close after replacement")
	if got, ok := tunnel.GetConnectorTunnel("connector-a"); !ok || got != newClient {
		t.Fatal("replacement connection is not the active connector connection")
	}
}
