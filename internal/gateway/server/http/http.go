package http_proxy

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	gateway_grpc "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/grpc_client"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/posture"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/grpc"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http *http.Server
	log  *slog.Logger
}

func NewProxyServer(
	cfg *config.Config,
	grpcClient *gateway_grpc.SafeClient,
	connectorRegistry *registry.Registry,
	redisClient *redis.Client,
	sessions *session.SessionManager,
	log *slog.Logger,
	activeStreams *registry.ActiveStreamRegistry,
	engine *policy.PolicyEngine,
	rtr *router.Router,
	grpcServer *grpc.GRPCServer,
	accessLogger *logging.AccessLogger,
) *ProxyServer {

	rateLimiter := session.NewRedisLimiter(redisClient, 10, time.Minute)
	errHandler := web.NewErrorHandler()
	staticFileHandler := web.NewStaticFileHandler()

	handler := NewHandler(
		log,
		connectorRegistry,
		sessions,
		rateLimiter,
		redisClient,
		grpcClient,
		activeStreams,
		cfg,
		rtr,
		errHandler,
	)

	// 1. Initialize posture dependencies
	geoReader, err := posture.NewMaxMindReader("/home/johnnyash/Downloads/GeoLite2-City.mmdb")
	if err != nil {
		log.Error("Georeader DB ERROR:", slog.Any("error", err))
	}

	torChecker := posture.NewCachedTorChecker(
		posture.NewDNSBasedTorChecker(),
		1*time.Hour,
	)

	// Optional: static IP reputation list
	repDB, _ := posture.NewStaticReputationDB([]string{
		"192.0.2.0/24", // Example blocked range
	})


	collector := posture.NewCollector(geoReader, torChecker, repDB)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(logging.AccessLogMiddleware(accessLogger, cfg.GatewayID, log))
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(session.SessionMiddleware(connectorRegistry, sessions, redisClient, cfg, errHandler, log))
	r.Use(SecurityHeadersMiddleware(DefaultSecurityConfig()))
	r.Use(posture.PostureMiddleware(collector, log))
	r.Use(RateLimiterMiddleware(DefaultRateLimiterConfig(redisClient), log, errHandler)) // ← after session
	r.Use(policy.PolicyMiddleware(engine, cfg, log, errHandler))

	r.Handle("/_ashrix/static/*", staticFileHandler)

	r.Get("/_ashrix/health", handler.Health)
	r.Get("/_ashrix/logout", handler.Logout)
	r.Get("/_ashrix/auth/callback", handler.Callback)
	r.HandleFunc("/*", handler.ProxyHandler)

	// ------------------------------------------------------------
	// Shared HTTP + gRPC handler
	// ------------------------------------------------------------

	rootHandler := http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {

			// ----------------------------------------------------
			// gRPC
			// ----------------------------------------------------

			if grpcServer.IsGRPCRequest(req) {
				// Defense in depth.

				// Even though "grpc-mtls" caused the TLS layer
				// to require a client certificate, we still
				// explicitly verify it here.
				if !grpcServer.HasVerifiedClientCertificate(req) {
					log.Warn("gRPC request rejected: missing verified client certificate", slog.String("remote_addr", req.RemoteAddr))
					http.Error(w, "mTLS client certificate required", http.StatusUnauthorized)
					return
				}
				// IMPORTANT:
				//
				// Do NOT run the normal Chi middleware chain.
				//
				// gRPC has its own protocol semantics and
				// long-lived HTTP/2 streams.
				// TLS was performed by net/http.
				//
				// grpc-go therefore doesn't automatically have
				// credentials.TLSInfo in peer.AuthInfo.
				//
				// Inject it into the request context.
				ctx := grpcServer.ContextWithTLSInfo(req.Context(), req.TLS)

				req = req.WithContext(ctx)
				grpcServer.Handler().ServeHTTP(w, req)
				return
			}

			// ----------------------------------------------------
			// Normal HTTPS application traffic
			// ----------------------------------------------------

			r.ServeHTTP(w, req)
		},
	)

	return &ProxyServer{
		http: &http.Server{
			Addr:              "",
			Handler:           rootHandler,
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
