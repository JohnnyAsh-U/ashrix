package registry

import (
	"context"
	"io"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// DisconnectReason categorizes operational terminations for structured auditing.
type DisconnectReason string

const (
	DisconnectRevoked      DisconnectReason = "revoked"
	DisconnectUnauthorized DisconnectReason = "unauthorized"
	DisconnectReplaced     DisconnectReason = "replaced_by_new_connection"
	DisconnectAdmin        DisconnectReason = "admin_action"
)


//Management stream is the interface to push commands to a connected connector.
//satisfied automatically by the GatewayConnectorEnvelope from the proto

type ManagementSession struct {
	Stream gen.ConnectorService_ConnectServer
	Cancel context.CancelFunc
}

// Close cancels the context bound to this specific gRPC stream.
func (s *ManagementSession) Close() {
	if s.Cancel != nil {
		s.Cancel()
	}
}


//Tunnel Session is the minimal interface for the dataplane connection to a connector
//(Quic or gRPC tunnel). Kept separate from ManagementStream deliberately
//they are different streams, different lifecycles.

type TunnelSession interface {
	OpenStream(ctx context.Context) (Stream, error)
	Close() error
}

type Stream interface {
	io.Reader
	io.Writer


	Close() error
	// CancelRead(code uint64)
	// CancelWrite(code uint64)
}
