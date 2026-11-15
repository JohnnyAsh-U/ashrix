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
	"io"
	"log/slog"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/frame"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

func testQUICPair(t *testing.T) (*quic.Conn, *quic.Conn) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"ashrix-test"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	accepted := make(chan *quic.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept(ctx)
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()
	client, err := quic.DialAddr(ctx, listener.Addr().String(), &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"ashrix-test"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.CloseWithError(0, "test complete") })
	select {
	case server := <-accepted:
		t.Cleanup(func() { _ = server.CloseWithError(0, "test complete") })
		return client, server
	case err := <-acceptErr:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	return nil, nil
}

func TestHandleDataTunnelForwardsToConfiguredUpstream(t *testing.T) {
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	backendDone := make(chan error, 1)
	go func() {
		conn, err := upstream.Accept()
		if err != nil {
			backendDone <- err
			return
		}
		defer conn.Close()
		request, err := io.ReadAll(conn)
		if err != nil {
			backendDone <- err
			return
		}
		_, err = conn.Write(append([]byte("reply:"), request...))
		backendDone <- err
	}()

	client, server := testQUICPair(t)
	clientStream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	apps := []*pb.ConnectorApps{{Id: "docs", Name: "Documents", Upstream: upstream.Addr().String()}}
	worker := &TunnelClient{log: testLogger(), config: &config.Config{Connector: config.Connector{ID: "connector-a"}}, apps: apps}
	workerDone := make(chan struct{})
	serverStream := mustAcceptStream(t, server)
	go func() { worker.handleDataTunnel(context.Background(), serverStream); close(workerDone) }()

	if err := frame.WriteFrame(clientStream, &pb.StreamFrame{ConnectorId: "connector-a", AppId: "docs", Method: "POST", Path: "/upload"}); err != nil {
		t.Fatal(err)
	}
	if _, err := clientStream.Write([]byte("document-payload")); err != nil {
		t.Fatal(err)
	}
	if err := clientStream.Close(); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(clientStream)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(response), "reply:document-payload"; got != want {
		t.Fatalf("response = %q, want %q", got, want)
	}
	select {
	case err := <-backendDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("backend did not finish")
	}
	select {
	case <-workerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("tunnel worker did not finish")
	}
}

func mustAcceptStream(t *testing.T, conn *quic.Conn) *quic.Stream {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return stream
}

func TestHandleDataTunnelRejectsWrongConnector(t *testing.T) {
	client, server := testQUICPair(t)
	stream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	worker := &TunnelClient{log: testLogger(), config: &config.Config{Connector: config.Connector{ID: "connector-a"}}, apps: []*pb.ConnectorApps{{Id: "docs", Upstream: "127.0.0.1:1"}}}
	done := make(chan struct{})
	serverStream := mustAcceptStream(t, server)
	go func() { worker.handleDataTunnel(context.Background(), serverStream); close(done) }()
	if err := frame.WriteFrame(stream, &pb.StreamFrame{ConnectorId: "connector-b", AppId: "docs"}); err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not reject mismatched connector")
	}
}

func TestHandleDataTunnelRejectsUnknownApplication(t *testing.T) {
	client, server := testQUICPair(t)
	stream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	worker := &TunnelClient{log: testLogger(), config: &config.Config{Connector: config.Connector{ID: "connector-a"}}, apps: []*pb.ConnectorApps{{Id: "docs", Upstream: "127.0.0.1:1"}}}
	done := make(chan struct{})
	serverStream := mustAcceptStream(t, server)
	go func() { worker.handleDataTunnel(context.Background(), serverStream); close(done) }()
	if err := frame.WriteFrame(stream, &pb.StreamFrame{ConnectorId: "connector-a", AppId: "unknown"}); err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not reject unknown application")
	}
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
