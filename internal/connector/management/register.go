package management

import (
	"context"
	"crypto/tls"
	"fmt"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

// managementConn holds the gRPC management stream.
// Separate from the tunnel transport — this is always gRPC.
type ManagementConn struct {
	Conn        *grpc.ClientConn
	Stream      pb.ConnectorService_ConnectClient
	GatewayUrl  string
	ConnectorID string
	Apps        []*pb.ConnectorApps
	StartedAt   time.Time
	log         *zap.Logger
}

// dialManagement opens the gRPC management connection to the gateway.
// This is ALWAYS gRPC — registration, heartbeat, cmd sync.
// Separate from the tunnel transport which may be QUIC or WebSocket.
func OpenStream(ctx context.Context, gatewayUrl string, connectorID string, tlsConfig *tls.Config, log *zap.Logger, apps []*pb.ConnectorApps) (*ManagementConn, error) {
	log.Info("dialing management plane", zap.String("addr", gatewayUrl))

	now := time.Now()

	conn, err := grpc.NewClient(gatewayUrl,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)

	if err != nil {
		return nil, fmt.Errorf("management dial: %w", err)
	}

	client := pb.NewConnectorServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("management stream: %w", err)
	}

	return &ManagementConn{
		Conn:        conn,
		Stream:      stream,
		log:         log,
		GatewayUrl:  gatewayUrl,
		ConnectorID: connectorID,
		StartedAt:   now,
		Apps: apps,
	}, nil
}

// register sends HelloMessage and waits for HelloAck.
func (c *ManagementConn) Register(ctx context.Context) error {
	c.log.Info("registering with gateway", zap.String("connector_id", c.ConnectorID))

	err := c.Stream.Send(&pb.ConnectorGatewayEnvelope{
		ConnectorId: c.ConnectorID,
		SentAt:      timestamppb.Now(),
		Payload: &pb.ConnectorGatewayEnvelope_Hello{
			Hello: &pb.ConnectorHello{
				ConnectorId: c.ConnectorID,
				Version:     "1.0.0",
				Transport:   "grpc",
				Apps: c.Apps,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	// 	// Wait for HelloAck with timeout
	ackCh := make(chan error, 1)
	go func() {
		msg, err := c.Stream.Recv()
		if err != nil {
			ackCh <- fmt.Errorf("recv hello ack: %w", err)
			return
		}
		switch p := msg.Payload.(type) {
		case *pb.GatewayConnectorEnvelope_HelloAck:
			c.log.Info("registration accepted",
				zap.String("session_id", p.HelloAck.SessionId),
				zap.String("server_version", p.HelloAck.ServerVersion),
			)
			ackCh <- nil
		case *pb.GatewayConnectorEnvelope_Reject:
			if p.Reject.Permanent {
				ackCh <- fmt.Errorf("gateway rejected permanently: %s", p.Reject.Reason)
			} else {
				ackCh <- fmt.Errorf("gateway rejected: %s", p.Reject.Reason)
			}
		default:
			ackCh <- fmt.Errorf("unexpected response to hello")
		}
	}()

	select {
	case err := <-ackCh:
		return err

	case <-time.After(10 * time.Second):
		return fmt.Errorf("hello ack timeout")

	case <-ctx.Done():
		return ctx.Err()
	}
}
