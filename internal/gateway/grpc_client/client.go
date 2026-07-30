package gateway_grpc

import (
	"context"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SafeClient exposes unary gRPC methods for HTTP handlers.
// It reads the current conn atomically, so it never blocks on reconnects.
type SafeClient struct {
	cm *ConnectionManager
}

func NewSafeClient(cm *ConnectionManager) *SafeClient {
	return &SafeClient{cm: cm}
}

func (sc *SafeClient) ExchangeToken(ctx context.Context, req *pb.ExchangeTokenRequest, opts ...grpc.CallOption) (*pb.ExchangeTokenResponse, error) {
	conn := sc.cm.CurrentConn()
	if conn == nil {
		return nil, status.Error(codes.Unavailable, "control plane not connected")
	}
	return pb.NewControlPlaneServiceClient(conn).ExchangeToken (ctx, req, opts...)
}

