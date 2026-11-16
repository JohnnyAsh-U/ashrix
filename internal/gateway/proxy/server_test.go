package proxy

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

	// "net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
)

func writeProxyServerCertificate(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func TestProxyServersStartServeHealthAndShutdown(t *testing.T) {
	certPath, keyPath := writeProxyServerCertificate(t, t.TempDir())
	cfg := &config.Config{}
	cfg.Gateway.ID = "gateway-test"
	cfg.Listeners.HTTP.Address = "127.0.0.1:0"
	cfg.Listeners.HTTPS.Address = "127.0.0.1:0"
	cfg.Listeners.HTTPS.Certificate, cfg.Listeners.HTTPS.PrivateKey = certPath, keyPath
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewProxyHandler(cfg, logger, nil, nil)
	servers, _ := NewProxyServers(cfg, logger, handler)
	if err := servers.StartProxyServers(); err != nil {
		t.Fatalf("StartProxyServers: %v", err)
	}

	httpURL := "http://" + servers.http.Addr + "/_ashrix/health"
	resp, err := http.Get(httpURL)
	if err != nil {
		t.Fatalf("HTTP health request: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != `{"status": "ok"}` {
		t.Fatalf("HTTP response = %d %q", resp.StatusCode, body)
	}

	tlsClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	httpsURL := "https://" + servers.https.Addr + "/_ashrix/health"
	resp, err = tlsClient.Get(httpsURL)
	if err != nil {
		t.Fatalf("HTTPS health request: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != `{"status": "ok"}` {
		t.Fatalf("HTTPS response = %d %q", resp.StatusCode, body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := servers.ShutdownProxyServers(ctx); err != nil {
		t.Fatalf("ShutdownProxyServers: %v", err)
	}
	tlsClient.CloseIdleConnections()
}
