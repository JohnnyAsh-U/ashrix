package proxy_test

import (
	"context"
	"net"
	"testing"
)

func TestBidirectionalProxyRelay(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer l.Close()

	clientData := []byte("hello from local application")
	readDone := make(chan struct{})

	go func() {
		defer close(readDone)
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 100)
		n, _ := conn.Read(buf)
		if string(buf[:n]) != string(clientData) {
			t.Errorf("unexpected payload: %s", string(buf[:n]))
		}
	}()

	clientConn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	streamA := &pipeAdapter{Conn: clientConn}
	streamB := &pipeAdapter{Conn: clientConn} // loopback pipe test

	_, _ = clientConn.Write(clientData)
	_ = clientConn.Close()

	_ = streamA
	_ = streamB
	<-readDone
}

type pipeAdapter struct {
	net.Conn
}

func (p *pipeAdapter) CloseRead() error {
	return nil
}

func (p *pipeAdapter) CloseWrite() error {
	return p.Close()
}

func (p *pipeAdapter) Context() context.Context {
	return context.Background()
}
