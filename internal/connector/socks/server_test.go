package socks_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/socks"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"golang.org/x/crypto/bcrypt"
)

// mockSession implements transport.Session for unit testing
type mockSession struct {
	openFunc func(ctx context.Context) (transport.Stream, error)
}

func (m *mockSession) OpenStream(ctx context.Context) (transport.Stream, error) {
	if m.openFunc != nil {
		return m.openFunc(ctx)
	}
	return nil, io.ErrUnexpectedEOF
}

func (m *mockSession) AcceptStream(ctx context.Context) (transport.Stream, error) {
	return nil, io.ErrUnexpectedEOF
}

func (m *mockSession) Close() error {
	return nil
}

func (m *mockSession) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *mockSession) TransportName() string {
	return "mock"
}

// mockStream implements transport.Stream with net.Pipe backend
type mockStream struct {
	net.Conn
}

func (m *mockStream) CloseWrite() error {
	if cw, ok := m.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return nil
}


func TestSocksAuthenticationAndRelay(t *testing.T) {
	credStore, err := socks.NewCredentialStore()
	if err != nil {
		t.Fatalf("NewCredentialStore failed: %v", err)
	}

	passHash, _ := bcrypt.GenerateFromPassword([]byte("valid-password"), bcrypt.DefaultCost)
	err = credStore.Replace([]socks.Credential{
		{AppID: "app-client-1", PasswordHash: string(passHash)},
	})
	if err != nil {
		t.Fatalf("CredentialStore.Replace failed: %v", err)
	}

	// Create pipe for transport stream simulation (gateway <-> socks connector)
	gatewaySide, connectorSide := net.Pipe()

	sess := &mockSession{
		openFunc: func(ctx context.Context) (transport.Stream, error) {
			return &mockStream{Conn: connectorSide}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Listen on random TCP port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer ln.Close()

	srv := socks.NewServer(ln.Addr().String(), credStore, sess, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.ListenAndServe(ctx)
	}()

	// 1. Simulate Gateway side responding to OPEN frame
	go func() {
		payload, err := frame.ReadFrame(gatewaySide)
		if err != nil {
			return
		}
		var reqFrame proto.StreamFrame
		if err := frame.DecodeFrame(payload, &reqFrame); err != nil {
			return
		}

		if reqFrame.SourceId != "app-client-1" || reqFrame.DestAppName != "target-app.internal" {
			return
		}

		ack := proto.StreamFrame{
			RequestId: reqFrame.RequestId,
			Method:    "OPEN_OK",
		}
		_ = frame.WriteFrame(gatewaySide, &ack)

		// Gateway echo loop back to connector
		buf := make([]byte, 32*1024)
		for {
			n, err := gatewaySide.Read(buf)
			if n > 0 {
				_, _ = gatewaySide.Write(buf[:n])
			}
			if err != nil {
				gatewaySide.Close()
				return
			}
		}
	}()

	// 2. Client connects to SOCKS proxy
	clientConn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial SOCKS server: %v", err)
	}
	defer clientConn.Close()

	// Step A: Negotiate (SOCKS5, User/Pass auth method 0x02)
	_, err = clientConn.Write([]byte{0x05, 0x01, 0x02})
	if err != nil {
		t.Fatalf("failed to write greeting: %v", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, reply); err != nil || reply[0] != 0x05 || reply[1] != 0x02 {
		t.Fatalf("negotiation failed: reply=%v, err=%v", reply, err)
	}

	// Step B: Authenticate
	user := "app-client-1"
	pass := "valid-password"
	authMsg := []byte{0x01, byte(len(user))}
	authMsg = append(authMsg, []byte(user)...)
	authMsg = append(authMsg, byte(len(pass)))
	authMsg = append(authMsg, []byte(pass)...)

	if _, err := clientConn.Write(authMsg); err != nil {
		t.Fatalf("failed to write auth msg: %v", err)
	}
	authReply := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, authReply); err != nil || authReply[0] != 0x01 || authReply[1] != 0x00 {
		t.Fatalf("authentication failed: reply=%v, err=%v", authReply, err)
	}

	// Step C: CONNECT Request
	host := "target-app.internal"
	port := uint16(8080)
	connReq := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	connReq = append(connReq, []byte(host)...)
	portBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(portBuf, port)
	connReq = append(connReq, portBuf...)

	if _, err := clientConn.Write(connReq); err != nil {
		t.Fatalf("failed to write connect req: %v", err)
	}

	socksReply := make([]byte, 10)
	if _, err := io.ReadFull(clientConn, socksReply); err != nil || socksReply[1] != 0x00 {
		t.Fatalf("SOCKS CONNECT failed: reply=%v, err=%v", socksReply, err)
	}

	// Step D: Transmit 256KB of data & verify zero byte loss
	payloadData := make([]byte, 256*1024)
	_, _ = rand.Read(payloadData)

	go func() {
		_, _ = clientConn.Write(payloadData)
	}()

	recvBuf := make([]byte, len(payloadData))
	if _, err := io.ReadFull(clientConn, recvBuf); err != nil {
		t.Fatalf("failed to receive echo payload: %v", err)
	}

	if !bytes.Equal(payloadData, recvBuf) {
		t.Fatalf("data corruption or byte loss detected in SOCKS flow! Sent %d bytes, received corrupt data", len(payloadData))
	}
}

func TestSocksInvalidAuthRejection(t *testing.T) {
	credStore, err := socks.NewCredentialStore()
	if err != nil {
		t.Fatalf("NewCredentialStore failed: %v", err)
	}

	passHash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	_ = credStore.Replace([]socks.Credential{
		{AppID: "app-1", PasswordHash: string(passHash)},
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer ln.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := socks.NewServer(ln.Addr().String(), credStore, &mockSession{}, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = srv.ListenAndServe(ctx) }()

	clientConn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial SOCKS server: %v", err)
	}
	defer clientConn.Close()

	// Negotiate
	_, _ = clientConn.Write([]byte{0x05, 0x01, 0x02})
	reply := make([]byte, 2)
	_, _ = io.ReadFull(clientConn, reply)

	// Authenticate with WRONG password
	user := "app-1"
	pass := "WRONG-password"
	authMsg := []byte{0x01, byte(len(user))}
	authMsg = append(authMsg, []byte(user)...)
	authMsg = append(authMsg, byte(len(pass)))
	authMsg = append(authMsg, []byte(pass)...)

	_, _ = clientConn.Write(authMsg)
	authReply := make([]byte, 2)
	_, err = io.ReadFull(clientConn, authReply)
	if err != nil || authReply[1] == 0x00 {
		t.Fatalf("server accepted invalid password! authReply=%v", authReply)
	}
}
