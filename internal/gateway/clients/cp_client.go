package client

import (
	"context"
	"crypto/tls"
	"fmt"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// CPClient wraps all gRPC connections to the Control Plane.
type CPClient struct {
	Bootstrap gen.BootstrapServiceClient
	Control   gen.ControlServiceClient
	Identity  gen.IdentityServiceClient
}

// NewBootstrapClient creates a client using system/public roots for initial registration.
func NewBootstrapClient(addr string) (gen.BootstrapServiceClient, error) {
	// Using default TLS (public CA)
	creds := credentials.NewTLS(&tls.Config{})
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return gen.NewBootstrapServiceClient(conn), nil
}

// NewMTLSClient creates the primary communication client once the gateway is bootstrapped.
func NewMTLSClient(addr string, tlsConfig *tls.Config) (*CPClient, error) {
	creds := credentials.NewTLS(tlsConfig)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}

	return &CPClient{
		Control:  gen.NewControlServiceClient(conn),
		Identity: gen.NewIdentityServiceClient(conn),
	}, nil
}

// PerformBootstrap handles the Phase 2.3 logic of requesting identity.
func (c *CPClient) PerformBootstrap(ctx context.Context, token, nodeID string) (*gen.BootstrapResponse, error) {
	// Generate CSR logic would go here
	req := &gen.BootstrapRequest{
		BootstrapToken: token,
		NodeId:         nodeID,
		NodeType:       "GATEWAY",
		// Csr: csrPEM,
	}
	return c.Bootstrap.Bootstrap(ctx, req)
}

// StartControlStream opens the persistent management pipe.
func (c *CPClient) StartControlStream(ctx context.Context) (gen.ControlService_ControlStreamClient, error) {
	stream, err := c.Control.ControlStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open control stream: %w", err)
	}
	return stream, nil
}
