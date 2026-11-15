package tunnel

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func gatewayLifecycleCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func waitForContextDone(t *testing.T, ctx context.Context, description string) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func TestStartReturnsWhenContextIsCancelled(t *testing.T) {
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{gatewayLifecycleCertificate(t)},
		NextProtos:   []string{"ashrix-lifecycle-test"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	tunnel := newTestTunnel()
	tunnel.listener = listener
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tunnel.Start(ctx) }()
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Start() error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start() did not return after context cancellation")
	}

	if _, err := listener.Accept(context.Background()); err == nil {
		t.Fatal("listener remained open after Start returned")
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

func TestStopClosesListenerAndActiveConnections(t *testing.T) {
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{gatewayLifecycleCertificate(t)},
		NextProtos:   []string{"ashrix-lifecycle-test"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err := quic.DialAddr(context.Background(), listener.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"ashrix-lifecycle-test"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server, err := listener.Accept(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.CloseWithError(0, "test complete")
		_ = server.CloseWithError(0, "test complete")
		_ = listener.Close()
	})

	tunnel := newTestTunnel()
	tunnel.listener = listener
	tunnel.Stop()

	waitForContextDone(t, client.Context(), "client connection to close after Stop")
	if _, err := listener.Accept(context.Background()); err == nil {
		t.Fatal("listener accepted a connection after Stop")
	}
}

func TestStartTracksAndCleansUpConnectionDuringStop(t *testing.T) {
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{gatewayLifecycleCertificate(t)},
		NextProtos:   []string{"ashrix-lifecycle-test"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tunnel := newTestTunnel()
	tunnel.listener = listener

	startDone := make(chan error, 1)
	go func() { startDone <- tunnel.Start(context.Background()) }()

	client, err := quic.DialAddr(context.Background(), listener.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"ashrix-lifecycle-test"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.CloseWithError(0, "test complete") })

	deadline := time.Now().Add(3 * time.Second)
	for {
		tunnel.mu.RLock()
		// active := len(tunnel.activeConns)
		tunnel.mu.RUnlock()
		// if active == 1 {
		// 	break
		// }
		if time.Now().After(deadline) {
			t.Fatal("Start did not register the accepted connection")
		}
		time.Sleep(10 * time.Millisecond)
	}

	tunnel.Stop()
	select {
	case err := <-startDone:
		if err == nil {
			t.Fatal("Start returned nil after listener shutdown")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not finish after Stop closed the active connection")
	}

	tunnel.mu.RLock()
	// remaining := len(tunnel.activeConns)
	tunnel.mu.RUnlock()
	// if remaining != 0 {
	// 	t.Fatalf("active connection map contains %d entries after shutdown, want 0", remaining)
	// }
}
