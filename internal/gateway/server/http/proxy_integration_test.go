package http_proxy

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/management"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/tunnel"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	quic_server "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/quic"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

const testConnectorID = "connector-integration-test"

type proxyHarness struct {
	ctx            context.Context
	quicServer     *quic_server.QUICServer
	gatewayCancel  context.CancelFunc
	gatewayDone    chan error
	connectorAddr  string
	clientTLS      *tls.Config
	registry       *registry.Registry
	apps           []*proto.ConnectorApps
	logger         *slog.Logger
	connector      transport.Session
	connectorStop  context.CancelFunc
	connectorDone  chan error
	proxy          *httptest.Server
	httpClient     *http.Client
	stopOnce       sync.Once
	stopGatewayErr error
	stopTunnelErr  error
}

func newTestApp(tb testing.TB, appID string) (*proto.ConnectorApps, *httptest.Server) {
	tb.Helper()
	downloadBody := bytes.Repeat([]byte{appID[0]}, 1<<20)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-App", appID)
		w.Header().Set("X-Test-Request-Length", strconv.FormatInt(r.ContentLength, 10))
		switch r.URL.Path {
		case "/upload":
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = io.WriteString(w, "ok")
		case "/download":
			w.Header().Set("Content-Length", strconv.Itoa(len(downloadBody)))
			_, _ = w.Write(downloadBody)
		default:
			_, _ = io.Copy(w, r.Body)
		}
	}))
	tb.Cleanup(upstream.Close)

	return &proto.ConnectorApps{
		Id:        appID,
		Name:      appID,
		Subdomain: appID,
		Upstream:  strings.TrimPrefix(upstream.URL, "http://"),
		Protocol:  "http",
	}, upstream
}

func newProxyHarness(tb testing.TB, apps []*proto.ConnectorApps) *proxyHarness {
	tb.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	serverTLS, clientTLS, clientCert := testQUICCertificates(tb)
	port := reserveUDPPort(tb)

	connectorRegistry := registry.New(logger)
	connectorRegistry.AttachManagement(
		testConnectorID,
		"tenant-integration-test",
		apps,
		&registry.ManagementSession{},
		clientCert,
		"active",
	)

	cfg := &config.Config{QUICPort: strconv.Itoa(port)}
	quicServer := quic_server.NewQUICServer(cfg, serverTLS, logger, connectorRegistry,
		router.NewRouter(connectorRegistry))
	ctx, cancel := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- quicServer.Start(ctx)
	}()

	handler := NewHandler(
		logger,
		router.NewRouter(connectorRegistry),
		web.NewErrorHandler(),
	)
	proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), session.AppID, r.Header.Get("X-Test-App-ID"))
		handler.ProxyHandler(w, r.WithContext(ctx))
	}))

	h := &proxyHarness{
		ctx:           ctx,
		quicServer:    quicServer,
		gatewayCancel: cancel,
		gatewayDone:   serverDone,
		connectorAddr: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		clientTLS:     clientTLS,
		registry:      connectorRegistry,
		apps:          apps,
		logger:        logger,
		proxy:         proxy,
		httpClient:    proxy.Client(),
	}
	tb.Cleanup(func() { h.shutdown() })

	if err := h.connectConnector(); err != nil {
		tb.Fatalf("start connector QUIC session: %v", err)
	}
	return h
}

func (h *proxyHarness) connectConnector() error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	retry := time.NewTicker(20 * time.Millisecond)
	defer retry.Stop()

	var connector transport.Session
	for connector == nil {
		dialCtx, cancel := context.WithTimeout(h.ctx, 250*time.Millisecond)
		candidate, err := transport.DialQUIC(dialCtx, h.connectorAddr, h.clientTLS, h.logger)
		cancel()
		if err == nil {
			connector = candidate
			break
		}
		select {
		case serverErr := <-h.gatewayDone:
			return fmt.Errorf("gateway QUIC server stopped: %w", serverErr)
		default:
		}
		select {
		case <-h.ctx.Done():
			return h.ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("QUIC connection was not established: %w", err)
		case <-retry.C:
		}
	}

	connectorCtx, cancel := context.WithCancel(h.ctx)
	connectorDone := make(chan error, 1)
	connectorTunnel := tunnel.NewTunnel(h.logger, h.apps, &management.ManagementConn{})
	go func() {
		connectorDone <- connectorTunnel.AcceptLoop(connectorCtx, connector)
	}()
	h.connector = connector
	h.connectorStop = cancel
	h.connectorDone = connectorDone

	attachCtx, attachCancel := context.WithTimeout(h.ctx, 5*time.Second)
	defer attachCancel()
	return h.waitForTunnel(attachCtx, true)
}

func (h *proxyHarness) waitForTunnel(ctx context.Context, wantAttached bool) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, attached := h.registry.GetTunnelSession(testConnectorID)
		if attached == wantAttached {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (h *proxyHarness) disconnectConnector() error {
	if h.connectorStop != nil {
		h.connectorStop()
	}
	if h.connector != nil {
		_ = h.connector.Close()
	}
	select {
	case <-h.connectorDone:
	case <-time.After(5 * time.Second):
		return fmt.Errorf("connector accept loop did not stop")
	}
	ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	defer cancel()
	return h.waitForTunnel(ctx, false)
}

func (h *proxyHarness) request(ctx context.Context, appID, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.proxy.URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Test-App-ID", appID)
	return h.httpClient.Do(req)
}

func (h *proxyHarness) shutdown() {
	h.stopOnce.Do(func() {
		if h.proxy != nil {
			h.proxy.Close()
		}
		if h.httpClient != nil {
			h.httpClient.CloseIdleConnections()
		}
		if h.connectorStop != nil {
			h.connectorStop()
		}
		if h.connector != nil {
			_ = h.connector.Close()
		}
		h.gatewayCancel()
		h.quicServer.Stop()
		h.stopGatewayErr = receiveShutdownResult(h.gatewayDone, "gateway")
		if h.connectorDone != nil {
			h.stopTunnelErr = receiveShutdownResult(h.connectorDone, "connector")
		}
	})
}

func receiveShutdownResult(done <-chan error, component string) error {
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("%s did not stop before deadline", component)
	}
}

func reserveUDPPort(tb testing.TB) int {
	tb.Helper()
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("reserve UDP port: %v", err)
	}
	port := listener.LocalAddr().(*net.UDPAddr).Port
	if err := listener.Close(); err != nil {
		tb.Fatalf("release UDP port: %v", err)
	}
	return port
}

func testQUICCertificates(tb testing.TB) (*tls.Config, *tls.Config, *x509.Certificate) {
	tb.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatalf("generate test CA key: %v", err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Ashrix integration test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		tb.Fatalf("create test CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		tb.Fatalf("parse test CA certificate: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	issueLeaf := func(commonName string, dnsNames []string, usage x509.ExtKeyUsage) tls.Certificate {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			tb.Fatalf("generate %s key: %v", commonName, err)
		}
		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
		if err != nil {
			tb.Fatalf("generate %s serial: %v", commonName, err)
		}
		template := &x509.Certificate{
			SerialNumber: serial,
			Subject:      pkix.Name{CommonName: commonName},
			DNSNames:     dnsNames,
			NotBefore:    now.Add(-time.Minute),
			NotAfter:     now.Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		}
		der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
		if err != nil {
			tb.Fatalf("create %s certificate: %v", commonName, err)
		}
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			tb.Fatalf("encode %s key: %v", commonName, err)
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
		pair, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			tb.Fatalf("load %s key pair: %v", commonName, err)
		}
		return pair
	}

	serverCert := issueLeaf("gateway.integration.test", []string{"localhost"}, x509.ExtKeyUsageServerAuth)
	clientCert := issueLeaf(testConnectorID, nil, x509.ExtKeyUsageClientAuth)
	clientLeaf, err := x509.ParseCertificate(clientCert.Certificate[0])
	if err != nil {
		tb.Fatalf("parse connector certificate: %v", err)
	}
	return &tls.Config{
			MinVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{serverCert},
			ClientCAs:    roots,
		}, &tls.Config{
			MinVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      roots,
			ServerName:   "localhost",
			NextProtos:   []string{"ashrix-quic-v1"},
		}, clientLeaf
}

func TestGatewayAndConnectorQUICStartupForwardsHTTP(t *testing.T) {
	app, _ := newTestApp(t, "startup-app")
	h := newProxyHarness(t, []*proto.ConnectorApps{app})

	if h.connector.TransportName() != "quic" {
		t.Fatalf("transport = %q, want quic", h.connector.TransportName())
	}
	entry, ok := h.registry.GetByConnectorID(testConnectorID)
	if !ok || !entry.IsTunnelAttached() {
		t.Fatal("connector tunnel was not attached to the gateway registry")
	}

	response, err := h.request(context.Background(), app.Id, "/echo", []byte("startup payload"))
	if err != nil {
		t.Fatalf("forward request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(body) != "startup payload" || response.Header.Get("X-Test-App") != app.Id {
		t.Fatalf("unexpected upstream response: app=%q body=%q", response.Header.Get("X-Test-App"), body)
	}
}

func TestHTTPForwardingSupportsMultipleAppsAndLargeBodies(t *testing.T) {
	firstApp, _ := newTestApp(t, "alpha-app")
	secondApp, _ := newTestApp(t, "beta-app")
	h := newProxyHarness(t, []*proto.ConnectorApps{firstApp, secondApp})

	for _, app := range []*proto.ConnectorApps{firstApp, secondApp} {
		payload := bytes.Repeat([]byte{app.Id[0]}, 2<<20)
		response, err := h.request(context.Background(), app.Id, "/echo", payload)
		if err != nil {
			t.Fatalf("forward to %s: %v", app.Id, err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			t.Fatalf("read response from %s: %v", app.Id, readErr)
		}
		if response.StatusCode != http.StatusOK || response.Header.Get("X-Test-App") != app.Id || !bytes.Equal(body, payload) {
			t.Fatalf("unexpected response from %s: status=%d app=%q body=%d bytes", app.Id, response.StatusCode, response.Header.Get("X-Test-App"), len(body))
		}
	}
}

func TestConcurrentHTTPRequestsUseIndependentQUICStreams(t *testing.T) {
	app, _ := newTestApp(t, "concurrent-app")
	h := newProxyHarness(t, []*proto.ConnectorApps{app})

	const requestCount = 24
	const bodySize = 64 << 10
	errCh := make(chan error, requestCount)
	var workers sync.WaitGroup
	for i := 0; i < requestCount; i++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			payload := bytes.Repeat([]byte{byte(index)}, bodySize)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			response, err := h.request(ctx, app.Id, "/echo", payload)
			if err != nil {
				errCh <- fmt.Errorf("request %d: %w", index, err)
				return
			}
			body, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if readErr != nil {
				errCh <- fmt.Errorf("read response %d: %w", index, readErr)
				return
			}
			if response.StatusCode != http.StatusOK || !bytes.Equal(body, payload) {
				errCh <- fmt.Errorf("response %d mismatch: status=%d body=%d bytes declared=%d request=%q app=%q", index, response.StatusCode, len(body), response.ContentLength, response.Header.Get("X-Test-Request-Length"), response.Header.Get("X-Test-App"))
			}
		}(i)
	}
	workers.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestConnectorDisconnectionAndReconnectionRestoresForwarding(t *testing.T) {
	app, _ := newTestApp(t, "reconnect-app")
	h := newProxyHarness(t, []*proto.ConnectorApps{app})
	if err := h.disconnectConnector(); err != nil {
		t.Fatalf("disconnect connector: %v", err)
	}
	if err := h.connectConnector(); err != nil {
		t.Fatalf("reconnect connector: %v", err)
	}

	response, err := h.request(context.Background(), app.Id, "/echo", []byte("after reconnect"))
	if err != nil {
		t.Fatalf("forward after reconnect: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response after reconnect: %v", err)
	}
	if string(body) != "after reconnect" {
		t.Fatalf("response after reconnect = %q", body)
	}
}

func TestGatewayAndConnectorGracefulShutdown(t *testing.T) {
	app, _ := newTestApp(t, "shutdown-app")
	h := newProxyHarness(t, []*proto.ConnectorApps{app})
	h.shutdown()
	if h.stopGatewayErr != nil {
		t.Fatalf("gateway shutdown: %v", h.stopGatewayErr)
	}
	if h.stopTunnelErr != nil {
		t.Fatalf("connector shutdown: %v", h.stopTunnelErr)
	}
}
