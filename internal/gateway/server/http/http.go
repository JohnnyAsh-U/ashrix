package http_proxy

import (
	"fmt"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	gateway_grpc "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/grpc_client"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/posture"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http *http.Server
	log  *zap.Logger
}

func NewProxyServer(cfg *config.Config, grpcClient *gateway_grpc.SafeClient, registry *registry.Registry, redisClient *redis.Client, log *zap.Logger) *ProxyServer {

	//Policy Engine takes Store as args to load the policies into the engine
	engine := policy.NewEngine()

	sessions := session.NewSessionManager(redisClient, cfg.SessionTTL, cfg.CookieSecure)
	rateLimiter := session.NewRedisLimiter(redisClient, 10, time.Minute)
	handler := NewHandler(
		log, registry, sessions, rateLimiter, redisClient, grpcClient, cfg,
	)

	// 1. Initialize posture dependencies
	geoReader, err := posture.NewMaxMindReader("/var/lib/ashrix/GeoLite2-City.mmdb")
	if err != nil {
		log.Error("Georeader DB ERROR:", zap.Error(err))
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
	r.Use(logging.AccessLogMiddleware(log)) // <-- add here
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(SecurityHeadersMiddleware(DefaultSecurityConfig())) // <-- updated
	r.Use(chimiddleware.Timeout(30 * time.Second))

	r.Use(session.SessionMiddleware(registry, sessions, redisClient, cfg, log))
	r.Use(posture.PostureMiddleware(collector, log))
	r.Use(RateLimiterMiddleware(DefaultRateLimiterConfig(redisClient), log)) // ← after session
	r.Use(policy.PolicyMiddleware(engine, cfg, log))

	r.Get("/health", handler.Health)
	r.Get("/logout", handler.Logout)
	r.Get("/_auth/callback", handler.Callback)
	r.Get("/*", handler.ProxyHandler)



	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		// TLSConfig will be set by the main loop using GatewayPKI
	}

	return &ProxyServer{
		http: srv,
		log:  log,
	}
}

func (s *ProxyServer) Start() error {
	s.log.Info("Gateway Http Proxy Server starting", zap.String("addr", s.http.Addr))
	// In production, this uses s.http.ListenAndServeTLS("", "")
	// with the Gateway's certificate identity.
	return s.http.ListenAndServe()
}
