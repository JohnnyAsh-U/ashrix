//go:build linux

package integration_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
)

type childProcess struct {
	cmd  *exec.Cmd
	done chan error
}

func TestGatewayAndConnectorBinariesWorkTogetherFromYAML(t *testing.T) {
	if testing.Short() {
		t.Skip("process-level integration test is skipped in short mode")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Dir(root) // go test runs with integration/ as the working directory.
	tmp := t.TempDir()
	gatewayBinary := filepath.Join(tmp, "ashrix-gateway")
	connectorBinary := filepath.Join(tmp, "ashrix-connector")
	build := func(output, packagePath string) {
		t.Helper()
		cmd := exec.Command("go", "build", "-o", output, packagePath)
		cmd.Dir = root
		if outputBytes, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", packagePath, err, outputBytes)
		}
	}
	build(gatewayBinary, "./cmd/gateway")
	build(connectorBinary, "./cmd/connector")

	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "process-e2e:%s:%s", r.Method, r.URL.Path)
	}))
	defer app.Close()

	gatewayHome := filepath.Join(tmp, "gateway-home")
	connectorHome := filepath.Join(tmp, "connector-home")
	gatewayConfigDir := filepath.Join(gatewayHome, ".ashrix")
	gatewayPKIDir := filepath.Join(gatewayConfigDir, "pki")
	connectorConfigDir := filepath.Join(connectorHome, ".ashrix", "connector")
	for _, dir := range []string{gatewayPKIDir, filepath.Join(gatewayConfigDir, "logs"), connectorConfigDir, filepath.Join(connectorConfigDir, "logs")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}

	ca, err := pki.GenerateCA("Ashrix process integration CA")
	if err != nil {
		t.Fatal(err)
	}
	caCertPath := filepath.Join(gatewayPKIDir, "ca.crt")
	caKeyPath := filepath.Join(gatewayPKIDir, "ca.key")
	if err := pki.SaveCA(ca, caCertPath, caKeyPath); err != nil {
		t.Fatal(err)
	}

	gatewayName := "gateway.process.integration.test"
	gatewayKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gatewayCSRDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gateway"}}, gatewayKey)
	if err != nil {
		t.Fatal(err)
	}
	gatewayCertPEM, _, err := ca.IssueGatewayCertificate("gateway", gatewayName, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: gatewayCSRDER}), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	gatewayCertPath := filepath.Join(gatewayPKIDir, "gateway.crt")
	gatewayKeyPath := filepath.Join(gatewayPKIDir, "gateway.key")
	gatewayKeyDER, err := x509.MarshalECPrivateKey(gatewayKey)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, gatewayCertPath, gatewayCertPEM, 0600)
	writeTestFile(t, gatewayKeyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: gatewayKeyDER}), 0600)

	connectorID := "process-connector"
	connectorKey, err := identity.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	connectorCSR, err := identity.GenerateCSR(connectorKey, connectorID)
	if err != nil {
		t.Fatal(err)
	}
	connectorCertPEM, connectorCert, err := ca.IssueCertificate(connectorID, connectorCSR, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	connectorKeyDER, err := x509.MarshalPKCS8PrivateKey(connectorKey)
	if err != nil {
		t.Fatal(err)
	}
	connectorCertPath := filepath.Join(connectorConfigDir, "connector.crt")
	connectorKeyPath := filepath.Join(connectorConfigDir, "connector.key")
	connectorCAPEMPath := filepath.Join(connectorConfigDir, "ca.crt")
	writeTestFile(t, connectorCertPath, connectorCertPEM, 0600)
	writeTestFile(t, connectorKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: connectorKeyDER}), 0600)
	writeTestFile(t, connectorCAPEMPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw}), 0644)

	httpAddress := reserveTCPAddress(t)
	httpsAddress := reserveTCPAddress(t)
	quicAddress := reserveUDPAddress(t)
	appTarget := strings.TrimPrefix(app.URL, "http://")
	gatewayYAML := fmt.Sprintf(`gateway:
  id: gateway-process-test
  server_name: %q
  certificate: %q
  private_key: %q
listeners:
  http:
    address: %q
  https:
    address: %q
    certificate: %q
    private_key: %q
  quic:
    address: %q
pki:
  ca_certificate: %q
  ca_private_key: %q
connectors:
  - id: %q
    certificate_serial: %q
applications:
  - name: docs
    domain: docs.process.integration.test
    connector: %q
logging:
  dir: %q
  level: info
  format: json
`, gatewayName, gatewayCertPath, gatewayKeyPath, httpAddress, httpsAddress, gatewayCertPath, gatewayKeyPath, quicAddress, caCertPath, caKeyPath, connectorID, pki.SerialString(connectorCert.SerialNumber), connectorID, filepath.Join(gatewayConfigDir, "logs"))
	writeTestFile(t, filepath.Join(gatewayConfigDir, "gateway.yaml"), []byte(gatewayYAML), 0600)

	connectorYAML := fmt.Sprintf(`connector:
  id: %q
  certificate: %q
  private_key: %q
  ca: %q
gateway:
  address: %q
  server_name: %q
applications:
  - name: docs
    target: %q
logging:
  dir: %q
  level: info
  format: json
`, connectorID, connectorCertPath, connectorKeyPath, connectorCAPEMPath, quicAddress, gatewayName, appTarget, filepath.Join(connectorConfigDir, "logs"))
	writeTestFile(t, filepath.Join(connectorConfigDir, "connector.yaml"), []byte(connectorYAML), 0600)

	gateway := startChild(t, gatewayBinary, gatewayHome)
	t.Cleanup(func() { stopChild(t, gateway) })
	waitHTTP(t, 8*time.Second, func() (*http.Response, error) {
		return (&http.Client{Timeout: time.Second}).Get("http://" + httpAddress + "/_ashrix/health")
	}, http.StatusOK)

	connector := startChild(t, connectorBinary, connectorHome)
	t.Cleanup(func() { stopChild(t, connector) })
	waitHTTP(t, 10*time.Second, func() (*http.Response, error) {
		request, _ := http.NewRequest(http.MethodGet, "http://"+httpAddress+"/from-process", nil)
		request.Host = "docs.process.integration.test"
		return (&http.Client{Timeout: time.Second}).Do(request)
	}, http.StatusOK)

	request, _ := http.NewRequest(http.MethodGet, "http://"+httpAddress+"/from-process", nil)
	request.Host = "docs.process.integration.test"
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "process-e2e:GET:/from-process" {
		t.Fatalf("process-level response = %q", body)
	}
}

func startChild(t *testing.T, binary, home string) *childProcess {
	t.Helper()
	cmd := exec.Command(binary, "start")
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", filepath.Base(binary), err)
	}
	child := &childProcess{cmd: cmd, done: make(chan error, 1)}
	go func() { child.done <- cmd.Wait() }()
	return child
}

func stopChild(t *testing.T, child *childProcess) {
	t.Helper()
	if child == nil || child.cmd.Process == nil {
		return
	}
	_ = child.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-child.done:
		if err != nil {
			t.Errorf("child process exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = child.cmd.Process.Kill()
		select {
		case <-child.done:
		case <-time.After(2 * time.Second):
			t.Error("child process did not exit after SIGTERM or SIGKILL")
		}
	}
}

func waitHTTP(t *testing.T, timeout time.Duration, request func() (*http.Response, error), wantStatus int) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		response, err := request()
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode == wantStatus {
				return
			}
			lastErr = fmt.Errorf("HTTP status %d, want %d", response.StatusCode, wantStatus)
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for HTTP status %d: %v", wantStatus, lastErr)
}

func writeTestFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}
