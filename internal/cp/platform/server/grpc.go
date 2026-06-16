package server

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// GRPCServer wraps the gRPC server and its configuration.
type GRPCServer struct {
	srv  *grpc.Server
	log  *slog.Logger
	addr string
}

// InitializeGRPCServer sets up the gRPC server with TLS.
func InitializeGRPCServer(cfg *config.Config, cp crypto.ControlPlaneCrypto, log *slog.Logger) *GRPCServer {
	// Get TLS configuration from the crypto package
	tlsConfig := crypto.NewServerTLSConfig(cp)
	creds := credentials.NewTLS(tlsConfig)

	// Initialize the gRPC server with TLS credentials
	s := grpc.NewServer(
		grpc.Creds(creds),
	)

	// Note: You will need to register your service implementations here.
	// Example:
	// gen.RegisterBootstrapServiceServer(s, bootstrapHandler)
	// gen.RegisterIdentityServiceServer(s, identityHandler)

	return &GRPCServer{
		srv:  s,
		log:  log,
		addr: cfg.GRPCAddr,
	}
}

// Start opens the listener and begins serving requests.
func (s *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	// s.log.Info("gRPC server listening", "addr", s.addr)
	return s.srv.Serve(lis)
}

// Stop performs a graceful shutdown.
func (s *GRPCServer) Stop() {
	s.srv.GracefulStop()
}
