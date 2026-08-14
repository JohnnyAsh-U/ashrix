package cp_grpc

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/repositories"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/policy"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// GRPCServer wraps the gRPC server and its configuration.
type GRPCServer struct {
	srv  *grpc.Server
	log  *slog.Logger
	addr string
}

// InitializeGRPCServer sets up the gRPC server with TLS.
func InitializeGRPCServer(
	cfg *config.Config,
	cp crypto.ControlPlaneCrypto,
	log *slog.Logger,
	redisClient *redis.Client,
	registry *registry.GatewayRegistry,
	distributor *policy.PolicyDistributor,
	repositories *repositories.Repositories,
) *GRPCServer {
	// Get TLS configuration from the crypto package
	tlsConfig := crypto.NewServerTLSConfig(cp)
	creds := credentials.NewTLS(tlsConfig)

	// Initialize the gRPC server with TLS credentials
	s := grpc.NewServer(
		grpc.Creds(creds),
		grpc.ChainStreamInterceptor(
			StreamIdentityInterceptor,
		// 	interceptors.JwtInterceptor([]byte("secret")),
		),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              30 * time.Second,
			Timeout:           10 * time.Second,
		}),
	)

	proto.RegisterControlPlaneServiceServer(s, &cpServer{
		policyStore: repositories.Policy,
		gatewayRepo: repositories.Gateway,
		PkiCARepo : repositories.PKICA,
		connectorRepo : repositories.Connector,
		registry:    registry,
		distributor:      distributor,
		log:         log,
		redisClient: redisClient,
	})

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
