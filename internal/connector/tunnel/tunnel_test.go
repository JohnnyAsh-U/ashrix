package tunnel

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

func TestHandleSessionRegistersAndStopsOnContextCancellation(t *testing.T) {
	client, server := testQUICPair(t)
	worker := &TunnelClient{
		log:    testLogger(),
		config: &config.Config{Connector: config.Connector{ID: "connector-a"}, Applications: []config.Application{{Name: "docs"}}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.handleSession(ctx, client) }()

	managementStream := mustAcceptStream(t, server)
	var registration pb.ControlMessage
	if err := json.NewDecoder(managementStream).Decode(&registration); err != nil {
		cancel()
		t.Fatal(err)
	}
	if registration.MsgType != pb.MsgType_REGISTER {
		cancel()
		t.Fatalf("message type = %v, want REGISTER", registration.MsgType)
	}
	if registration.ConnectorId != "connector-a" {
		cancel()
		t.Fatalf("connector ID = %q, want connector-a", registration.ConnectorId)
	}
	if len(registration.ConnectorApps) != 1 || registration.ConnectorApps[0].Id != "docs" {
		cancel()
		t.Fatalf("registered apps = %#v, want docs", registration.ConnectorApps)
	}

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("handleSession error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session did not stop after context cancellation")
	}
}

func TestHandleSessionStopsWhenGatewayDisconnects(t *testing.T) {
	client, server := testQUICPair(t)
	worker := &TunnelClient{
		log:    testLogger(),
		config: &config.Config{Connector: config.Connector{ID: "connector-a"}},
	}
	done := make(chan error, 1)
	go func() { done <- worker.handleSession(context.Background(), client) }()

	managementStream := mustAcceptStream(t, server)
	var registration pb.ControlMessage
	if err := json.NewDecoder(managementStream).Decode(&registration); err != nil {
		t.Fatal(err)
	}
	if err := server.CloseWithError(0, "gateway shutdown"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		// A remote close may surface as either a stream or connection error;
		// the lifecycle requirement is that the session terminates promptly.
	case <-time.After(2 * time.Second):
		t.Fatal("session did not stop after gateway disconnect")
	}
}
