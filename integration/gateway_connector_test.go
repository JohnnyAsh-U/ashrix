package integration_test

// import (
// 	"context"
// 	"crypto/ecdsa"
// 	"crypto/elliptic"
// 	"crypto/rand"
// 	"crypto/x509"
// 	"crypto/x509/pkix"
// 	"encoding/pem"
// 	"io"
// 	"log/slog"
// 	"net"
// 	"os"
// 	"path/filepath"
// 	"testing"
// 	"time"

// 	connectorconfig "github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
// 	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/identity"
// 	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
// 	connectortunnel "github.com/JohnnyAsh-U/ashrix-api/internal/connector/tunnel"
// 	gatewayconfig "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
// 	gatewaycrypto "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
// 	gatewaytunnel "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/tunnel"
// 	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
// )

// func TestGatewayAndConnectorEstablishAuthenticatedQUICSession(t *testing.T) {
// 	const connectorID = "integration-connector"
// 	const gatewayName = "gateway.integration.test"

// 	tmp := t.TempDir()
// 	ca, err := pki.GenerateCA("Ashrix integration test CA")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	gatewayKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	gatewayCSR, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gateway"}}, gatewayKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	gatewayCertPEM, _, err := ca.IssueGatewayCertificate("gateway", gatewayName, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: gatewayCSR}), time.Hour)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	gatewayCertPath := filepath.Join(tmp, "gateway.crt")
// 	gatewayKeyPath := filepath.Join(tmp, "gateway.key")
// 	if err := os.WriteFile(gatewayCertPath, gatewayCertPEM, 0600); err != nil {
// 		t.Fatal(err)
// 	}
// 	gatewayKeyDER, err := x509.MarshalECPrivateKey(gatewayKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if err := os.WriteFile(gatewayKeyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: gatewayKeyDER}), 0600); err != nil {
// 		t.Fatal(err)
// 	}

// 	connectorKey, err := identity.GeneratePrivateKey()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	connectorCSR, err := identity.GenerateCSR(connectorKey, connectorID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	connectorCertPEM, connectorCert, err := ca.IssueCertificate(connectorID, connectorCSR, time.Hour)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	connectorCertPath := filepath.Join(tmp, "connector.crt")
// 	connectorKeyPath := filepath.Join(tmp, "connector.key")
// 	if err := os.WriteFile(connectorCertPath, connectorCertPEM, 0600); err != nil {
// 		t.Fatal(err)
// 	}
// 	connectorKeyDER, err := x509.MarshalPKCS8PrivateKey(connectorKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if err := os.WriteFile(connectorKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: connectorKeyDER}), 0600); err != nil {
// 		t.Fatal(err)
// 	}

// 	serverPKI, err := gatewaycrypto.NewLocalGatewayPKI(gatewayCertPath, gatewayKeyPath, ca.Certificate, gatewayName)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	connectorPKI, err := startup.NewLocalPKIInitialiser(connectorCert, connectorKey, string(ca.Certificate.Raw), gatewayName)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	portListener, err := net.ListenPacket("udp", "127.0.0.1:0")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	addr := portListener.LocalAddr().String()
// 	if err := portListener.Close(); err != nil {
// 		t.Fatal(err)
// 	}

// 	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
// 	gatewayCfg := &gatewayconfig.Config{
// 		Listeners:    gatewayconfig.Listeners{QUIC: gatewayconfig.QUICListener{Address: addr}},
// 		Connectors:   []gatewayconfig.Connector{{ID: connectorID, CertificateSerial: pki.SerialString(connectorCert.SerialNumber)}},
// 		Applications: []gatewayconfig.Application{{Name: "docs", Domain: "docs.integration.test", Connector: connectorID}},
// 	}
// 	gateway, err := gatewaytunnel.NewQuicTunnel(gatewayCfg, serverPKI.GetTLSConfig(), logger)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	gatewayCtx, stopGateway := context.WithCancel(context.Background())
// 	gatewayDone := make(chan error, 1)
// 	go func() { gatewayDone <- gateway.Start(gatewayCtx) }()
// 	t.Cleanup(func() {
// 		stopGateway()
// 		gateway.Stop()
// 		select {
// 		case <-gatewayDone:
// 		case <-time.After(3 * time.Second):
// 			t.Error("gateway did not stop")
// 		}
// 	})

// 	connectorCfg := &connectorconfig.Config{
// 		Connector: connectorconfig.Connector{ID: connectorID},
// 		Gateway:   connectorconfig.Gateway{Address: addr, ServerName: gatewayName},
// 	}
// 	client := connectortunnel.NewTunnel(connectorCfg, connectorPKI.TLSConfig(), logger)
// 	connectorCtx, stopConnector := context.WithCancel(context.Background())
// 	connectorDone := make(chan error, 1)
// 	go func() { connectorDone <- client.Run(connectorCtx) }()
// 	t.Cleanup(func() {
// 		stopConnector()
// 		select {
// 		case <-connectorDone:
// 		case <-time.After(3 * time.Second):
// 			t.Error("connector did not stop")
// 		}
// 	})

// 	deadline := time.Now().Add(5 * time.Second)
// 	for time.Now().Before(deadline) {
// 		if gateway.IsConnectorTunnelAttached(connectorID) {
// 			return
// 		}
// 		select {
// 		case err := <-gatewayDone:
// 			t.Fatalf("gateway exited before connector registration: %v", err)
// 		default:
// 		}
// 		time.Sleep(10 * time.Millisecond)
// 	}
// 	t.Fatal("connector did not establish an authenticated QUIC session before timeout")
// }
