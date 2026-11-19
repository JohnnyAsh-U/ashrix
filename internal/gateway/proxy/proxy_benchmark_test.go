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
	"fmt"
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
)

func proxyBenchmarkQUICPair(t *testing.B) (*quic.Conn, *quic.Conn) {
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
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{alpn}}, &quic.Config{
		MaxIdleTimeout:     time.Minute,
		KeepAlivePeriod:    15 * time.Second,
		MaxIncomingStreams: 100000,
	})
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
	client, err := quic.DialAddr(ctx, listener.Addr().String(), &tls.Config{InsecureSkipVerify: true, NextProtos: []string{alpn}}, &quic.Config{
		MaxIdleTimeout:     time.Minute,
		KeepAlivePeriod:    15 * time.Second,
		MaxIncomingStreams: 100000,
	})
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

func newProxyBenchmarkTunnel(t *testing.B) *gatewaytunnel.QuicTunnel {
	t.Helper()
	cfg := &gatewayconfig.Config{
		Listeners:    gatewayconfig.Listeners{QUIC: gatewayconfig.QUICListener{Address: "127.0.0.1:0"}},
		Connectors:   []gatewayconfig.Connector{{ID: "connector-a"}},
		Applications: []gatewayconfig.Application{{Name: "app-a", Domain: "http://connector-a", Connector: "connector-a"}},
	}
	qt, err := gatewaytunnel.NewQuicTunnel(cfg, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"ashrix-proxy-test"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(qt.Stop)
	return qt
}

func newTestRequest(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	ctx := r.Context()
	ctx = context.WithValue(ctx, ConnectorID, "connector-a")
	ctx = context.WithValue(ctx, AppID, "app-a")
	r = r.WithContext(ctx)
	return r
}

func handleBenchmarkUpstream(
	stream *quic.Stream,
	errors chan<- error,
) {
	defer stream.Close()

	reportError := func(err error) {
		if err == nil {
			return
		}

		select {
		case errors <- err:
		default:
		}
	}

	// if err := stream.SetDeadline(
	//     time.Now().Add(5 * time.Second),
	// ); err != nil {
	//     reportError(err)
	//     return
	// }

	payload, err := frame.ReadFrame(stream)
	if err != nil {
		reportError(fmt.Errorf("read frame: %w", err))
		return
	}

	var msg pb.StreamFrame

	if err := frame.DecodeFrame(payload, &msg); err != nil {
		reportError(fmt.Errorf("decode frame: %w", err))
		return
	}

	reader := bufio.NewReader(stream)

	upstreamReq, err := http.ReadRequest(reader)
	if err != nil {
		reportError(fmt.Errorf("read request: %w", err))
		return
	}

	if upstreamReq.Body != nil {
		_, err = io.Copy(io.Discard, upstreamReq.Body)
		_ = upstreamReq.Body.Close()

		if err != nil {
			reportError(fmt.Errorf("read request body: %w", err))
			return
		}
	}

	_, err = io.WriteString(
		stream,
		"HTTP/1.1 201 Created\r\n"+
			"Content-Length: 7\r\n"+
			"Content-Type: text/plain\r\n"+
			"\r\n"+
			"created",
	)

	if err != nil {
		reportError(fmt.Errorf("write response: %w", err))
	}
}

func BenchmarkProxy(b *testing.B) {
	client, server := proxyBenchmarkQUICPair(b)

	qt := newProxyBenchmarkTunnel(b)
	session := qt.AttachTunnel("connector-a", server, nil)
	if session == nil {
		b.Fatal("failed to attach connector: connector-1 not registered")
	}

	registeredConn, ok := qt.GetConnectorTunnel("connector-a")
	if !ok {
		b.Fatal("connector-a not available after AttachTunnel")
	}

	if registeredConn != server {
		b.Fatal("registered connection does not match benchmark server connection")
	}
	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	ctx := b.Context()

	upstreamErrors := make(chan error, 64)
	acceptDone := make(chan struct{})

	// Accept incoming streams from the gateway.
	go func() {
		defer close(acceptDone)
		for {
			stream, err := client.AcceptStream(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				b.Log(err)
				return
			}

			go handleBenchmarkUpstream(
				stream,
				upstreamErrors,
			)
		}
	}()

	b.Cleanup(func() {
		<-acceptDone
	})

	warmReq := newTestRequest(
		http.MethodPost,
		"http://connector-a/test",
		strings.NewReader("payload"),
	)

	// Warmup: establish the complete request/response path.
	warmCtx, cancel := context.WithTimeout(
		warmReq.Context(),
		5*time.Second,
	)

	warmReq = warmReq.WithContext(warmCtx)

	warmW := httptest.NewRecorder()

	ProxyRequest(warmW, warmReq, qt, logger)

	resp := warmW.Result()

	_, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		cancel()
		b.Fatal(err)
	}

	resp.Body.Close()

	cancel()

	if warmW.Code != http.StatusCreated {
		b.Fatalf(
			"warmup failed: expected 201, got %d; body=%s",
			warmW.Code,
			warmW.Body.String(),
		)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		r := newTestRequest(
			http.MethodPost,
			"http://connector-a/test",
			strings.NewReader("payload"),
		)

		reqCtx, cancel := context.WithTimeout(
			r.Context(),
			5*time.Second,
		)

		r = r.WithContext(reqCtx)
		w := httptest.NewRecorder()

		ProxyRequest(w, r, qt, logger)

		resp := w.Result()

		_, err := io.Copy(io.Discard, resp.Body)
		if err != nil {
			cancel()
			b.Fatal(err)
		}

		resp.Body.Close()

		cancel()

		if w.Code != http.StatusCreated {
			b.Fatalf(
				"iteration %d: expected 201, got %d; body=%s",
				i,
				w.Code,
				w.Body.String(),
			)
		}
	}
}
