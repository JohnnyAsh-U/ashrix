package proxy

import (
	"bufio"
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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/frame"
	gatewayconfig "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	gatewaytunnel "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/tunnel"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
	"google.golang.org/protobuf/proto"
)

func proxyTestQUICPair(t *testing.T) (*quic.Conn, *quic.Conn) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
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
	const alpn = "ashrix-proxy-test"
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{alpn}}, nil)
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
	client, err := quic.DialAddr(ctx, listener.Addr().String(), &tls.Config{InsecureSkipVerify: true, NextProtos: []string{alpn}}, nil)
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

func newProxyTestTunnel(t *testing.T) *gatewaytunnel.QuicTunnel {
	t.Helper()
	cfg := &gatewayconfig.Config{
		Listeners:  gatewayconfig.Listeners{QUIC: gatewayconfig.QUICListener{Address: "127.0.0.1:0"}},
		Connectors: []gatewayconfig.Connector{{ID: "connector-a"}},
	}
	qt, err := gatewaytunnel.NewQuicTunnel(cfg, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"ashrix-proxy-test"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(qt.Stop)
	return qt
}

func TestProxyRequestForwardsRequestAndReturnsUpstreamResponse(t *testing.T) {
	client, server := proxyTestQUICPair(t)
	qt := newProxyTestTunnel(t)
	qt.AttachTunnel("connector-a", server, nil)

	upstreamDone := make(chan error, 1)
	go func() {
		stream, err := client.AcceptStream(context.Background())
		if err != nil {
			upstreamDone <- err
			return
		}
		defer stream.Close()
		payload, err := frame.ReadFrame(stream)
		if err != nil {
			upstreamDone <- err
			return
		}
		var msg pb.StreamFrame
		if err := proto.Unmarshal(payload, &msg); err != nil {
			upstreamDone <- err
			return
		}
		if msg.ConnectorId != "connector-a" || msg.AppId != "app-a" || msg.Method != http.MethodPost || msg.Path != "/documents" || msg.Query != "version=2" {
			upstreamDone <- io.ErrUnexpectedEOF
			return
		}
		if msg.Headers["X-Ashrix-App-Id"].Values[0] != "app-a" || msg.Headers["X-Ashrix-Request-Id"].Values[0] == "spoofed" {
			upstreamDone <- io.ErrUnexpectedEOF
			return
		}
		upstreamReq, err := http.ReadRequest(bufio.NewReader(stream))
		if err != nil {
			upstreamDone <- err
			return
		}
		body, err := io.ReadAll(upstreamReq.Body)
		_ = upstreamReq.Body.Close()
		if err != nil {
			upstreamDone <- err
			return
		}
		if string(body) != "payload" {
			upstreamDone <- io.ErrUnexpectedEOF
			return
		}
		_, err = io.WriteString(stream, "HTTP/1.1 201 Created\r\nContent-Length: 7\r\nContent-Type: text/plain\r\n\r\ncreated")
		upstreamDone <- err
	}()

	r := httptest.NewRequest(http.MethodPost, "http://app.example/documents?version=2", strings.NewReader("payload"))
	r.Host = "app.example"
	r.Header.Set("X-Ashrix-App-Id", "spoofed")
	r.Header.Set("X-Ashrix-Request-Id", "spoofed")
	ctx := context.WithValue(r.Context(), AppID, "app-a")
	ctx = context.WithValue(ctx, ConnectorID, "connector-a")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	ProxyRequest(w, r, qt, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if w.Code != http.StatusCreated || w.Body.String() != "created" {
		t.Fatalf("response = %d %q, want 201 Created", w.Code, w.Body.String())
	}
	select {
	case err := <-upstreamDone:
		if err != nil {
			t.Fatalf("upstream processing: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upstream did not finish handling request")
	}
}

func TestProxyRequestReturnsServiceUnavailableWhenConnectorIsOffline(t *testing.T) {
	qt := newProxyTestTunnel(t)
	r := httptest.NewRequest(http.MethodGet, "http://app.example/", nil)
	ctx := context.WithValue(r.Context(), AppID, "app-a")
	ctx = context.WithValue(ctx, ConnectorID, "connector-a")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	ProxyRequest(w, r, qt, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestProxyRequestReturnsBadGatewayWhenTunnelCannotOpenStream(t *testing.T) {
	client, server := proxyTestQUICPair(t)
	qt := newProxyTestTunnel(t)
	qt.AttachTunnel("connector-a", server, nil)
	if err := server.CloseWithError(0, "simulate disconnected connector"); err != nil {
		t.Fatal(err)
	}
	_ = client

	r := httptest.NewRequest(http.MethodGet, "http://app.example/", nil)
	ctx := context.WithValue(r.Context(), AppID, "app-a")
	ctx = context.WithValue(ctx, ConnectorID, "connector-a")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	ProxyRequest(w, r, qt, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestIsWebSocketRequestRequiresUpgradeAndConnectionTokens(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		want   bool
	}{
		{"valid", http.Header{"Upgrade": []string{"websocket"}, "Connection": []string{"Upgrade"}}, true},
		{"case insensitive", http.Header{"Upgrade": []string{"WebSocket"}, "Connection": []string{"keep-alive, uPgRaDe"}}, true},
		{"missing connection", http.Header{"Upgrade": []string{"websocket"}}, false},
		{"missing upgrade", http.Header{"Connection": []string{"Upgrade"}}, false},
		{"wrong upgrade protocol", http.Header{"Upgrade": []string{"h2c"}, "Connection": []string{"Upgrade"}}, false},
		{"upgrade is only substring", http.Header{"Upgrade": []string{"websocket"}, "Connection": []string{"not-upgrade"}}, false},
		{"empty", make(http.Header), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Header: tt.header}
			if got := isWebSocketRequest(r); got != tt.want {
				t.Fatalf("isWebSocketRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
