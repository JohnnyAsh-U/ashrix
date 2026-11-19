package integration_test

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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	connectorconfig "github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
	connectortunnel "github.com/JohnnyAsh-U/ashrix-api/internal/connector/tunnel"
	gatewayconfig "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	gatewaycrypto "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	gatewayproxy "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/proxy"
	gatewaytunnel "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/tunnel"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"

	// pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/gorilla/websocket"
	"github.com/quic-go/quic-go"
)

const (
	integrationConnectorID = "integration-connector"
	integrationGatewayName = "gateway.integration.test"
	ALPN                   = "ashrix/1"
)

type integrationStack struct {
	gateway       *gatewaytunnel.QuicTunnel
	gatewayPKI    *gatewaycrypto.LocalGatewayPKI
	gatewayCancel context.CancelFunc
	gatewayDone   chan error
	connectorStop context.CancelFunc
	connectorDone chan error
	proxy         *gatewayproxy.ProxyServer
	accessLogger  *logging.AccessLogger
	httpAddress   string
	httpsAddress  string
	quicAddress   string
}

func TestEndToEndHTTPAccessThroughGatewayAndConnector(t *testing.T) {
	var observed strings.Builder
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintf(&observed, "%s %s?%s %s", r.Method, r.URL.Path, r.URL.RawQuery, string(body))
		w.Header().Set("X-Upstream", "documents")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("document accepted"))
	}))
	defer app.Close()

	stack := startIntegrationStack(t, map[string]string{"docs": app.URL}, true)
	client := &http.Client{Timeout: 5 * time.Second}
	request, err := http.NewRequest(http.MethodPost, "http://"+stack.httpAddress+"/ingest?source=integration", strings.NewReader("payload-123"))
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "docs.integration.test"
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request through gateway: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%q", response.StatusCode, http.StatusCreated, body)
	}
	if got := string(body); got != "document accepted" {
		t.Fatalf("response body = %q", got)
	}
	if got := response.Header.Get("X-Upstream"); got != "documents" {
		t.Fatalf("upstream response header = %q", got)
	}
	if got := observed.String(); got != "POST /ingest?source=integration payload-123" {
		t.Fatalf("upstream observed %q", got)
	}
}

func TestEndToEndRoutesMultipleApplicationsOnOneConnector(t *testing.T) {
	makeApp := func(marker string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, marker)
		}))
	}
	docs := makeApp("documents-backend")
	wiki := makeApp("wiki-backend")
	defer docs.Close()
	defer wiki.Close()

	stack := startIntegrationStack(t, map[string]string{"docs": docs.URL, "wiki": wiki.URL}, true)
	for host, want := range map[string]string{
		"docs.integration.test": "documents-backend",
		"wiki.integration.test": "wiki-backend",
	} {
		request, _ := http.NewRequest(http.MethodGet, "http://"+stack.httpAddress+"/", nil)
		request.Host = host
		response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
		if err != nil {
			t.Fatalf("request for %s: %v", host, err)
		}
		got, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || string(got) != want {
			t.Fatalf("host %s: status=%d body=%q, want %q", host, response.StatusCode, got, want)
		}
	}
}

func TestGatewayReturnsNotFoundForUnknownHostAndServesHealth(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") }))
	defer app.Close()
	stack := startIntegrationStack(t, map[string]string{"docs": app.URL}, true)
	client := &http.Client{Timeout: 5 * time.Second}

	request, _ := http.NewRequest(http.MethodGet, "http://"+stack.httpAddress+"/", nil)
	request.Host = "unknown.integration.test"
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown host status = %d, want 404", response.StatusCode)
	}

	response, err = client.Get("http://" + stack.httpAddress + "/_ashrix/health")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"status": "ok"`) {
		t.Fatalf("health response: status=%d body=%q", response.StatusCode, body)
	}
}

func TestGatewayReturnsServiceUnavailableWhenConnectorIsOffline(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "should not be reached") }))
	defer app.Close()
	stack := startIntegrationStack(t, map[string]string{"docs": app.URL}, false)
	request, _ := http.NewRequest(http.MethodGet, "http://"+stack.httpAddress+"/", nil)
	request.Host = "docs.integration.test"
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("offline connector status = %d, want 503", response.StatusCode)
	}
}

func TestGatewayHTTPSListenerServesTLS(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "secure-route") }))
	defer app.Close()
	stack := startIntegrationStack(t, map[string]string{"docs": app.URL}, true)
	request, _ := http.NewRequest(http.MethodGet, "https://"+stack.httpsAddress+"/", nil)
	request.Host = "docs.integration.test"
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: integrationInsecureTLSConfig()}}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTPS request: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(body) != "secure-route" {
		t.Fatalf("HTTPS response: status=%d body=%q", response.StatusCode, body)
	}
}

func startIntegrationStack(t *testing.T, targets map[string]string, startConnector bool) *integrationStack {
	t.Helper()
	tmp := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ca, err := pki.GenerateCA("Ashrix integration test CA")
	if err != nil {
		t.Fatal(err)
	}

	gatewayKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gatewayCSRDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gateway"}}, gatewayKey)
	if err != nil {
		t.Fatal(err)
	}
	gatewayCertPEM, _, err := ca.IssueGatewayCertificate("gateway", integrationGatewayName, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: gatewayCSRDER}), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	gatewayCertPath := filepath.Join(tmp, "gateway.crt")
	gatewayKeyPath := filepath.Join(tmp, "gateway.key")
	if err := os.WriteFile(gatewayCertPath, gatewayCertPEM, 0600); err != nil {
		t.Fatal(err)
	}
	gatewayKeyDER, err := x509.MarshalECPrivateKey(gatewayKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gatewayKeyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: gatewayKeyDER}), 0600); err != nil {
		t.Fatal(err)
	}

	connectorKey, err := identity.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	connectorCSR, err := identity.GenerateCSR(connectorKey, integrationConnectorID)
	if err != nil {
		t.Fatal(err)
	}
	connectorCertPEM, connectorCert, err := ca.IssueCertificate(integrationConnectorID, connectorCSR, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "connector.crt"), connectorCertPEM, 0600); err != nil {
		t.Fatal(err)
	}
	connectorKeyDER, err := x509.MarshalPKCS8PrivateKey(connectorKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "connector.key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: connectorKeyDER}), 0600); err != nil {
		t.Fatal(err)
	}

	serverPKI, err := gatewaycrypto.NewLocalGatewayPKI(gatewayCertPath, gatewayKeyPath, ca.Certificate, integrationGatewayName)
	if err != nil {
		t.Fatal(err)
	}
	// The connector initializer accepts the CA certificate as DER, not PEM.
	connectorPKI, err := startup.NewLocalPKIInitialiser(connectorCert, connectorKey, string(ca.Certificate.Raw), integrationGatewayName)
	if err != nil {
		t.Fatal(err)
	}

	quicAddress := reserveUDPAddress(t)
	gatewayCfg := &gatewayconfig.Config{
		Gateway: gatewayconfig.Gateway{ID: "integration-gateway"},
		Listeners: gatewayconfig.Listeners{
			QUIC: gatewayconfig.QUICListener{Address: quicAddress},
		},
		Connectors: []gatewayconfig.Connector{{ID: integrationConnectorID, CertificateSerial: pki.SerialString(connectorCert.SerialNumber)}},
		// Applications: ,
	}
	appIDs := make([]string, 0, len(targets))
	for appID := range targets {
		appIDs = append(appIDs, appID)
	}
	// Keep deterministic app ordering so connector and gateway configurations match.
	for i := 0; i < len(appIDs); i++ {
		for j := i + 1; j < len(appIDs); j++ {
			if appIDs[j] < appIDs[i] {
				appIDs[i], appIDs[j] = appIDs[j], appIDs[i]
			}
		}
	}
	connectorApplications := make([]connectorconfig.Application, 0, len(appIDs))
	for _, id := range appIDs {
		domain := id + ".integration.test"
		gatewayCfg.Applications = append(gatewayCfg.Applications, gatewayconfig.Application{Name: id, Domain: domain, Connector: integrationConnectorID})
		connectorApplications = append(connectorApplications, connectorconfig.Application{Name: id, Target: strings.TrimPrefix(targets[id], "http://")})
	}

	gateway, err := gatewaytunnel.NewQuicTunnel(gatewayCfg, serverPKI.GetTLSConfig(), logger)
	if err != nil {
		t.Fatal(err)
	}
	gatewayCtx, gatewayCancel := context.WithCancel(context.Background())
	gatewayDone := make(chan error, 1)
	go func() { gatewayDone <- gateway.Start(gatewayCtx) }()
	stack := &integrationStack{gateway: gateway, gatewayCancel: gatewayCancel, gatewayDone: gatewayDone, quicAddress: quicAddress, gatewayPKI: serverPKI}
	t.Cleanup(func() {
		if stack.connectorStop != nil {
			stack.connectorStop()
			select {
			case <-stack.connectorDone:
			case <-time.After(3 * time.Second):
				t.Error("connector did not stop")
			}
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if stack.proxy != nil {
			if err := stack.proxy.ShutdownProxyServers(shutdownCtx); err != nil {
				t.Errorf("shutdown HTTP proxy: %v", err)
			}
		}
		if stack.accessLogger != nil {
			stack.accessLogger.Close()
		}
		stack.gatewayCancel()
		stack.gateway.Stop()
		select {
		case <-stack.gatewayDone:
		case <-time.After(3 * time.Second):
			t.Error("gateway did not stop")
		}
	})

	httpAddress := reserveTCPAddress(t)
	httpsAddress := reserveTCPAddress(t)
	gatewayCfg.Listeners.HTTP = gatewayconfig.HTTPListener{Address: httpAddress}
	gatewayCfg.Listeners.HTTPS = gatewayconfig.HTTPSListener{Address: httpsAddress, Certificate: gatewayCertPath, PrivateKey: gatewayKeyPath}
	if err := os.MkdirAll(filepath.Join(tmp, "logs"), 0700); err != nil {
		t.Fatal(err)
	}
	gatewayCfg.Logging.Dir = filepath.Join(tmp, "logs")
	accessLogger := logging.NewAccessLogger(gatewayCfg)
	proxyHandler := gatewayproxy.NewProxyHandler(gatewayCfg, logger, gateway, accessLogger)
	proxyServer, _ := gatewayproxy.NewProxyServers(gatewayCfg, logger, proxyHandler)
	if err := proxyServer.StartProxyServers(); err != nil {
		t.Fatal(err)
	}

	stack.proxy = proxyServer
	stack.accessLogger = accessLogger
	stack.httpAddress = httpAddress
	stack.httpsAddress = httpsAddress
	if startConnector {
		connectorCfg := &connectorconfig.Config{
			Connector:    connectorconfig.Connector{ID: integrationConnectorID},
			Gateway:      connectorconfig.Gateway{Address: quicAddress, ServerName: integrationGatewayName},
			Applications: connectorApplications,
		}
		connector := connectortunnel.NewTunnel(connectorCfg, connectorPKI.TLSConfig(), logger)
		connectorCtx, connectorCancel := context.WithCancel(context.Background())
		stack.connectorStop = connectorCancel
		stack.connectorDone = make(chan error, 1)
		go func() { stack.connectorDone <- connector.Run(connectorCtx) }()
		waitFor(t, 5*time.Second, func() bool { return gateway.IsConnectorTunnelAttached(integrationConnectorID) }, "connector registration")
	}

	return stack
}

func reserveUDPAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func reserveTCPAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool, description string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}

func integrationInsecureTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} // Test-only: exercises the HTTPS listener without coupling to its hostname.
}

func TestConnectorDisconnectRemovesGatewaySession(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") }))
	defer app.Close()
	stack := startIntegrationStack(t, map[string]string{"docs": app.URL}, true)
	if !stack.gateway.IsConnectorTunnelAttached(integrationConnectorID) {
		t.Fatal("connector should be attached before cancellation")
	}
	stack.connectorStop()
	select {
	case <-stack.connectorDone:
	case <-time.After(3 * time.Second):
		t.Fatal("connector Run did not return after cancellation")
	}
	stack.connectorStop = nil // cleanup must not wait on the already-consumed completion signal
	waitFor(t, 3*time.Second, func() bool { return !stack.gateway.IsConnectorTunnelAttached(integrationConnectorID) }, "gateway session detach")
}

func TestEndToEndConcurrentRequestsAndLargePayload(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Received-Bytes", fmt.Sprint(len(body)))
		_, _ = w.Write(body)
	}))
	defer app.Close()
	stack := startIntegrationStack(t, map[string]string{"docs": app.URL}, true)

	payload := bytes.Repeat([]byte("a7Z"), 512*1024)
	const requests = 8
	var wg sync.WaitGroup
	errs := make(chan error, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request, err := http.NewRequest(http.MethodPost, "http://"+stack.httpAddress+"/upload", bytes.NewReader(payload))
			if err != nil {
				errs <- err
				return
			}
			request.Host = "docs.integration.test"
			response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
			if err != nil {
				errs <- err
				return
			}
			defer response.Body.Close()
			got, err := io.ReadAll(response.Body)
			if err != nil {
				errs <- err
				return
			}
			if response.StatusCode != http.StatusOK || !bytes.Equal(got, payload) {
				errs <- fmt.Errorf("status=%d returned_bytes=%d", response.StatusCode, len(got))
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestEndToEndWebSocketUpgradeAndBidirectionalMessages(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			kind, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(kind, append([]byte("echo:"), message...)); err != nil {
				return
			}
		}
	}))
	defer app.Close()
	stack := startIntegrationStack(t, map[string]string{"docs": app.URL}, true)

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
		NetDialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, stack.httpAddress)
		},
	}
	conn, response, err := dialer.Dial("ws://docs.integration.test/socket", nil)
	if err != nil {
		if response != nil {
			t.Fatalf("websocket handshake failed: %v (status %d)", err, response.StatusCode)
		}
		t.Fatalf("websocket handshake failed: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello-over-quic")); err != nil {
		t.Fatal(err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(message) != "echo:hello-over-quic" {
		t.Fatalf("websocket response = %q", message)
	}
}

func TestGatewayRejectsConnectorCertificateFromUntrustedCA(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "unused") }))
	defer app.Close()
	stack := startIntegrationStack(t, map[string]string{"docs": app.URL}, false)
	rogueCA, err := pki.GenerateCA("Untrusted integration CA")
	if err != nil {
		t.Fatal(err)
	}
	rogueKey, err := identity.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	rogueCSR, err := identity.GenerateCSR(rogueKey, integrationConnectorID)
	if err != nil {
		t.Fatal(err)
	}
	_, rogueCert, err := rogueCA.IssueCertificate(integrationConnectorID, rogueCSR, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// Trust the real gateway CA for server authentication, but present a client certificate
	// issued by a different CA. The gateway must reject the QUIC TLS handshake.
	// The rogue client skips verification of the gateway's server certificate only in
	// this negative test; the gateway still validates the presented client certificate.
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
		NextProtos:         []string{ALPN},
		Certificates:       []tls.Certificate{{Certificate: [][]byte{rogueCert.Raw}, PrivateKey: rogueKey, Leaf: rogueCert}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := quic.DialAddr(ctx, stack.quicAddress, clientTLS, &quic.Config{})

	if err != nil {
		t.Logf("Client observed TLS rejection: %v", err)
		return
	}
	defer conn.CloseWithError(0, "test complete")

	select {
	case <-conn.Context().Done():
		t.Log("Connection closed after server-side processing")
	case <-time.After(2 * time.Second):
		t.Fatalf("connection remained open investigate server authentitcation")
	}
}
