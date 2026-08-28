package grpc

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type GRPCServer struct {
	server *grpc.Server
	addr   string
	log    *slog.Logger
}


func NewGRPCServer(cfg *config.Config, tlsConfig *tls.Config, log *slog.Logger, registry *registry.Registry) *GRPCServer {
	// Enforce mTLS by requiring client certificates
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	creds := credentials.NewTLS(tlsConfig)
	s := grpc.NewServer(grpc.Creds(creds))

	// TODO: Register your gRPC handlers here
	gen.RegisterConnectorServiceServer(s, &Server{registry: registry, log: log})

	return &GRPCServer{
		server: s,
		addr:   fmt.Sprintf(":%s", cfg.GRPCPort),
		log:    log,
	}
}

func (s *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port %s: %w", s.addr, err)
	}

	s.log.Info("Gateway gRPC Server starting", slog.String("addr", s.addr))
	return s.server.Serve(lis)
}

func (s *GRPCServer) Stop() {
	s.log.Info("Gateway gRPC Server stopping")
	s.server.GracefulStop()
}
