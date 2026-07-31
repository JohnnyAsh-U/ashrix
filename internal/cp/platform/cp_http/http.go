package cp_http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/app"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/auth"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/gateway"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/org"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/redis"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Server wraps the HTTP server and all dependencies.
type Server struct {
	http   *http.Server
	log    *slog.Logger
	signer pki.CASigner
}

// New wires together all dependencies and builds the router.
func InitializeHttpServer(BaseDir string, cfg *config.Config, dbQueries *store.Queries, redisStore *redis.RedisStore, log *slog.Logger, signer pki.CASigner) *Server {

	//Mailer config
	mailer := utils.NewMailerService(cfg)
	//Jwt Utils
	jwtUtils := utils.NewJwtUtils(
		cfg.JwtAccessSecret,
		cfg.JwtRefreshSecret,
		cfg.JwtAccessTTL,
		cfg.JwtRefreshTTL,
	)
	// Router
	r := chi.NewRouter()

	// ── Global middleware ──────────────────────────────────────
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(middleware.LoggingMiddleware(log))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	//Apps routes
	appRepo := app.NewPostgresRepository(dbQueries)
	appService := app.NewService(appRepo)
	appHandler := app.NewAppHandler(appService)

	//Org routes
	orgRepo := org.NewPostgresRepository(dbQueries)
	orgService := org.NewService(orgRepo)
	orgHandler := org.NewOrgHandler(orgService)

	//Gateway routes
	gatewayRepo := gateway.NewPostgresRepository(dbQueries)
	gatewayService := gateway.NewService(gatewayRepo)
	gatewayHandler := gateway.NewGatewayHandler(gatewayService, signer)

	//IDP Routes
	idpRepo := identity.NewPostgresRepository(dbQueries)
	idpSession, err := identity.NewIDPSession(BaseDir, cfg.PKIConfig.PKIUnlockSecret, "ashrix", redisStore.Client())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	idpService := identity.NewIDPService(
		idpRepo,
		// appRepo,
		// orgRepo,
		gatewayRepo,
		idpSession,
		redisStore.Client(),
		cfg,
		[]byte(cfg.PKIConfig.IDPSecretEncryptionKey),
	)
	idpHandler := identity.NewIDPHandler(idpService, log)

	//Auth routes
	authRepo := auth.NewPostgresRepository(dbQueries)
	authService := auth.NewService(
		authRepo,
		orgService,
		mailer,
		cfg,
		jwtUtils,
		cfg.AppUrl,
	)
	authHandler := auth.NewAuthHandler(
		authService,
		cfg.JwtAccessSecret,
	)

	//Connectors routes
	connectorRepo := connector.NewPostgresRepository(dbQueries)
	connectorService := connector.NewService(connectorRepo)
	connectorHandler := connector.NewConnectorHandler(connectorService, signer)

	//Docs - date this in prod
	r.Get("/docs/*", httpSwagger.Handler(
		httpSwagger.URL("/docs/doc.json"),
	))

	r.Route("/api/v1", func(r chi.Router) {

		r.Group(func(r chi.Router) {
			r.Route("/authorize", idpHandler.Routes)
		})

		r.Group(func(r chi.Router) {
			r.Route("/auth", authHandler.Routes)
			r.Route("/internal/gateways", gatewayHandler.WithoutAuthRoutes)
			r.Route("/internal/connectors", connectorHandler.WithoutAuthRoutes)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware([]byte(cfg.JwtAccessSecret)))
			r.Route("/gateways", gatewayHandler.WithAuthRoutes)
			r.Route("/orgs", orgHandler.Routes)
			r.Route("/connectors", connectorHandler.WithAuthRoutes)
			r.Route("/apps", appHandler.Routes)
		})
	})

	r.Get("/healthz", handleHealth)

	httpSrv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{http: httpSrv, log: log}
}

func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
