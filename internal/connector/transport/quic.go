package transport

import (
	// "context"
	// "crypto/tls"
	// "fmt"
	"github.com/quic-go/quic-go"
	// "github.com/gorilla/websocket"
	// "github.com/hashicorp/yamux"
	"go.uber.org/zap"
)

type quicSession struct {
	conn quic.Conn
	log *zap.Logger
	done chan struct{}
}

