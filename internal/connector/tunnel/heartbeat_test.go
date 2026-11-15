package tunnel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

func TestRunHeartbeatsSendsHeartbeatAndAcceptsAcknowledgement(t *testing.T) {
	client, server := testQUICPair(t)
	clientStream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	serverStream := mustAcceptStream(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	worker := &TunnelClient{log: testLogger()}
	go func() {
		done <- worker.runHeartbeatsWithTiming(ctx, clientStream, 10*time.Millisecond, 100*time.Millisecond)
	}()

	decoder := json.NewDecoder(serverStream)
	var heartbeat pb.ControlMessage
	if err := decoder.Decode(&heartbeat); err != nil {
		cancel()
		t.Fatal(err)
	}
	if heartbeat.MsgType != pb.MsgType_HEARTBEAT {
		cancel()
		t.Fatalf("message type = %v, want HEARTBEAT", heartbeat.MsgType)
	}
	if err := json.NewEncoder(serverStream).Encode(&pb.ControlMessage{MsgType: pb.MsgType_HEARTBEAT_ACK}); err != nil {
		cancel()
		t.Fatal(err)
	}

	// Acknowledgement resets the timeout; the loop should continue sending.
	var second pb.ControlMessage
	if err := decoder.Decode(&second); err != nil {
		cancel()
		t.Fatal(err)
	}
	if second.MsgType != pb.MsgType_HEARTBEAT {
		cancel()
		t.Fatalf("second message type = %v, want HEARTBEAT", second.MsgType)
	}
	cancel()
	_ = clientStream.Close()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("runHeartbeats error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat loop did not stop after cancellation")
	}
}

func TestRunHeartbeatsTimesOutWithoutAcknowledgement(t *testing.T) {
	client, server := testQUICPair(t)
	clientStream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	serverStream := mustAcceptStream(t, server)
	defer serverStream.CancelRead(0)
	defer clientStream.CancelRead(0)

	done := make(chan error, 1)
	worker := &TunnelClient{log: testLogger()}
	go func() {
		done <- worker.runHeartbeatsWithTiming(context.Background(), clientStream, 5*time.Millisecond, 40*time.Millisecond)
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "acknowledgement timed out") {
			t.Fatalf("runHeartbeats error = %v, want acknowledgement timeout", err)
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat loop did not time out")
	}
	// The acknowledgement reader is blocked on stream I/O after timeout.
	// Closing the connection releases that reader goroutine.
	_ = client.CloseWithError(0, "test cleanup")
}

func TestRunHeartbeatsReturnsWhenPeerClosesStream(t *testing.T) {
	client, server := testQUICPair(t)
	clientStream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	serverStream := mustAcceptStream(t, server)
	if err := serverStream.Close(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	worker := &TunnelClient{log: testLogger()}
	go func() {
		done <- worker.runHeartbeatsWithTiming(context.Background(), clientStream, time.Second, time.Second)
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "read heartbeat acknowledgement") {
			t.Fatalf("runHeartbeats error = %v, want acknowledgement read error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat loop did not stop after peer stream closure")
	}
}
