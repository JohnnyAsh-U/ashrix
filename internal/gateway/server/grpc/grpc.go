package grpc

import (
	// "crypto/tls"
	// "fmt"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"sync"

	// "net"
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	// "google.golang.org/grpc/credentials"
)

type GRPCServer struct {
	server *grpc.Server
	log    *slog.Logger

	stopOnce sync.Once
}

func NewGRPCServer(log *slog.Logger, registry *registry.Registry, tlsConfig *tls.Config, IsProdEnv bool) *GRPCServer {
	// IMPORTANT:
	//
	// We are using grpc.Server as an HTTP/2 handler.
	//
	// TLS/mTLS is handled by the shared TCP listener.
	//
	// Therefore grpc.Server itself does not need to own
	// the TLS configuration or TCP listener.
	// s := grpc.NewServer()

	opts := []grpc.ServerOption{}

	//When In Dev mode different port must be used and the tls
	//Config is handled here
	
	if !IsProdEnv {
		opts = append(
			opts,
			grpc.Creds(credentials.NewTLS(tlsConfig)),
			grpc.UnaryInterceptor(UnaryTLSInterceptor(log)),
			grpc.StreamInterceptor(StreamTLSInterceptor(log)),
		)
	}

	// When Prod Same Port is used for HTTP AND GRPC so the tls is 
	// handled in the tls.go for both http and grpc
	s := grpc.NewServer(opts...)

	// TODO: Register your gRPC handlers here
	gen.RegisterConnectorServiceServer(s, &Server{registry: registry, log: log})

	return &GRPCServer{
		server: s,
		log:    log,
	}
}

func (s *GRPCServer) ContextWithTLSInfo(ctx context.Context, state *tls.ConnectionState) context.Context {
	return InjectTLSInfo(ctx, state)
}

// Handler exposes grpc.Server as an http.Handler.
//
// This allows normal HTTPS and gRPC to share the same
// TCP listener.
func (s *GRPCServer) Handler() http.Handler {
	return s.server
}

// Serve allows the gRPC server to run on its own listener.
//
// This is used when HTTPS is disabled and gRPC gets its own
// TLS listener.
func (s *GRPCServer) Serve(listener net.Listener) error {
	return s.server.Serve(listener)
}

// IsGRPCRequest determines whether the HTTP request is
// a gRPC request.
func (s *GRPCServer) IsGRPCRequest(r *http.Request) bool {

	if r.ProtoMajor != 2 {
		return false
	}

	contentType := strings.ToLower(
		r.Header.Get("Content-Type"),
	)

	return strings.HasPrefix(
		contentType,
		"application/grpc",
	)
}

// HasVerifiedClientCertificate checks whether the TLS
// connection successfully authenticated a client certificate.
func (s *GRPCServer) HasVerifiedClientCertificate(
	r *http.Request,
) bool {

	if r.TLS == nil {
		return false
	}

	if len(r.TLS.PeerCertificates) == 0 {
		return false
	}

	if len(r.TLS.VerifiedChains) == 0 {
		return false
	}

	return true
}

// ClientCertificate returns the verified client certificate.
func (s *GRPCServer) ClientCertificate(r *http.Request) (*x509.Certificate, error) {

	if !s.HasVerifiedClientCertificate(r) {
		return nil, fmt.Errorf("verified client certificate not present")
	}

	return r.TLS.PeerCertificates[0], nil
}

// InjectTLSInfo puts the HTTP TLS state into the gRPC context.
//
// This is necessary when grpc.Server.ServeHTTP is used because
// net/http performed the TLS handshake rather than grpc-go.
func InjectTLSInfo(ctx context.Context, state *tls.ConnectionState) context.Context {

	if state == nil {
		return ctx
	}

	// grpc-go normally exposes TLS authentication through:
	//
	//     peer.FromContext(ctx)
	//
	//     p.AuthInfo.(credentials.TLSInfo)
	//
	// Since net/http performed the TLS handshake, we recreate
	// that information here.

	tlsInfo := credentials.TLSInfo{State: *state}
	p := &peer.Peer{AuthInfo: tlsInfo}

	return peer.NewContext(ctx, p)
}

// func (s *GRPCServer) Start() error {
// 	lis, err := net.Listen("tcp", s.addr)
// 	if err != nil {
// 		return fmt.Errorf("failed to listen on gRPC port %s: %w", s.addr, err)
// 	}

// 	s.log.Info("Gateway gRPC Server starting", slog.String("addr", s.addr))
// 	return s.server.Serve(lis)
// }

func (s *GRPCServer) Stop() {
	s.log.Info("Gateway gRPC Server stopping")
	s.server.GracefulStop()
}
