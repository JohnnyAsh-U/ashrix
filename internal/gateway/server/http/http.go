package http_proxy

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/grpc"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/internal/telemetry"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http *http.Server
	log  *slog.Logger
}

func NewProxyServer(
	cfg *config.Config,
	connectorRegistry *registry.Registry,
	log *slog.Logger,
	rtr *router.Router,
	// grpcServer *grpc.GRPCServer,
	accessLogger *logging.AccessLogger,
) *ProxyServer {

	// errHandler := web.NewErrorHandler()
	// staticFileHandler := web.NewStaticFileHandler()
	// rateLimiterConfig := DefaultRateLimiterConfig()

	handler := NewHandler(
		log,
		rtr,
		// errHandler,
	)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(logging.AccessLogMiddleware(accessLogger, cfg.Gateway.ID, log))
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	// r.Use(session.AppAccessMiddleware(connectorRegistry, cfg, errHandler, log))
	r.Use(SecurityHeadersMiddleware(DefaultSecurityConfig()))

	// r.Handle("/_ashrix/static/*", staticFileHandler)

	r.Get("/_ashrix/health", handler.Health)
	r.HandleFunc("/*", handler.ProxyHandler)

	// // ------------------------------------------------------------
	// // Shared HTTP + gRPC handler
	// // ------------------------------------------------------------

	// rootHandler := http.HandlerFunc(
	// 	func(w http.ResponseWriter, req *http.Request) {

	// 		// ----------------------------------------------------
	// 		// gRPC
	// 		// ----------------------------------------------------

	// 		if grpcServer.IsGRPCRequest(req) {
	// 			// Defense in depth.

	// 			// Even though "grpc-mtls" caused the TLS layer
	// 			// to require a client certificate, we still
	// 			// explicitly verify it here.
	// 			if !grpcServer.HasVerifiedClientCertificate(req) {
	// 				log.Warn("gRPC request rejected: missing verified client certificate", slog.String("remote_addr", req.RemoteAddr))
	// 				http.Error(w, "mTLS client certificate required", http.StatusUnauthorized)
	// 				return
	// 			}
	// 			// IMPORTANT:
	// 			//
	// 			// Do NOT run the normal Chi middleware chain.
	// 			//
	// 			// gRPC has its own protocol semantics and
	// 			// long-lived HTTP/2 streams.
	// 			// TLS was performed by net/http.
	// 			//
	// 			// grpc-go therefore doesn't automatically have
	// 			// credentials.TLSInfo in peer.AuthInfo.
	// 			//
	// 			// Inject it into the request context.
	// 			ctx := grpcServer.ContextWithTLSInfo(req.Context(), req.TLS)

	// 			req = req.WithContext(ctx)
	// 			grpcServer.Handler().ServeHTTP(w, req)
	// 			return
	// 		}

	// 		// ----------------------------------------------------
	// 		// Normal HTTPS application traffic
	// 		// ----------------------------------------------------

	// 		r.ServeHTTP(w, req)
	// 	},
	// )

	return &ProxyServer{
		http: &http.Server{
			Addr:              "",
			Handler:           telemetry.HTTPMiddleware(cfg.Gateway.ID)(r),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		},

		log: log,
	}
}

// Start serves HTTP and gRPC on the supplied TLS listener.
func (s *ProxyServer) Start(listener net.Listener) error {
	s.log.Info("Gateway HTTP + gRPC server starting", slog.String("addr", listener.Addr().String()))
	return s.http.Serve(listener)
}

func (s *ProxyServer) Shutdown(ctx context.Context) error {
	s.log.Info("Gateway HTTP + gRPC server shutting down")
	return s.http.Shutdown(ctx)
}

func (s *ProxyServer) Stop() error {
	s.log.Info("Gateway HTTP + gRPC server stopping")
	return s.http.Close()
}
