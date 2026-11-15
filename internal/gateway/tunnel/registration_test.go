package tunnel

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"testing"
	"time"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

func gatewayTestQUICPair(t *testing.T) (*quic.Conn, *quic.Conn) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
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
	const alpn = "ashrix-gateway-test"
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
		c, err := listener.Accept(ctx)
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- c
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

func runRegistrationTest(t *testing.T, payload string) error {
	t.Helper()
	client, server := gatewayTestQUICPair(t)
	stream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tunnel := newTestTunnel()
	done := make(chan error, 1)
	go func() { done <- tunnel.handleIncomingConnection(context.Background(), server) }()
	if _, err := io.WriteString(stream, payload); err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("registration handler did not terminate")
		return nil
	}
}

func TestHandleIncomingConnectionRejectsMalformedRegistration(t *testing.T) {
	err := runRegistrationTest(t, "{not-json}\n")
	if err == nil {
		t.Fatal("malformed registration unexpectedly accepted")
	}
}

func TestHandleIncomingConnectionRequiresRegisterAsFirstMessage(t *testing.T) {
	payload, err := json.Marshal(pb.ControlMessage{MsgType: pb.MsgType_HEARTBEAT, ConnectorId: "connector-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runRegistrationTest(t, string(payload)+"\n"); err == nil {
		t.Fatal("heartbeat was accepted as the first management message")
	}
}

func TestHandleIncomingConnectionRejectsUnknownConnector(t *testing.T) {
	payload, err := json.Marshal(pb.ControlMessage{MsgType: pb.MsgType_REGISTER, ConnectorId: "unknown-connector"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runRegistrationTest(t, string(payload)+"\n"); err == nil {
		t.Fatal("unknown connector registration unexpectedly accepted")
	}
}
