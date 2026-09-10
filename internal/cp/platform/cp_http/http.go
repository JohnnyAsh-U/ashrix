package cp_http

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/app"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/auth"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/repositories"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/events"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/gateway"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/logs"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/org"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/crypto"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/dispatcher"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/redis"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/policy"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Server wraps the HTTP server and all dependencies.
type Server struct {
	http     *http.Server
	log      *slog.Logger
	CASigner pki.CASigner
}

// New wires together all dependencies and builds the router.
func InitializeHttpServer(
	BaseDir string,
	cfg *config.Config,
	respositories *repositories.Repositories,
	redisStore *redis.RedisStore,
	policyDistributor *policy.PolicyDistributor,
	log *slog.Logger,
	CASigner pki.CASigner,
	gatewayRegistry *registry.GatewayRegistry,
	bundleSigner crypto.BundleSigning,
) *Server {

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
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(middleware.LoggingMiddleware(log))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.Metadata)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	//GatewayDispatcher
	eventsDispatcher := events.NewGatewayDispatcher(respositories.Event, gatewayRegistry, log)

	//Apps routes
	appService := app.NewService(respositories.App, respositories.Event, respositories.Connector, eventsDispatcher)
	appHandler := app.NewAppHandler(appService)

	//Org routes
	orgService := org.NewService(respositories.Org)
	orgHandler := org.NewOrgHandler(orgService)

	//IDP Routes
	idpSession, err := identity.NewIDPSession(BaseDir, cfg.PKIConfig.PKIUnlockSecret, "ashrix", redisStore.Client(), log, bundleSigner)
	if err != nil {
		os.Exit(1)
	}
	idpService := identity.NewIDPService(
		respositories.IDP,
		respositories.App,
		respositories.Event,
		respositories.Gateway,
		idpSession,
		redisStore.Client(),
		cfg,
		[]byte(cfg.PKIConfig.IDPSecretEncryptionKey),
		eventsDispatcher,
	)
	idpHandler := identity.NewIDPHandler(idpService, log)

	//Auth routes
	authService := auth.NewService(
		respositories.Auth,
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

	//Gateway routes
	gatewayService := gateway.NewService(respositories.Gateway, respositories.PKICA, respositories.Event, eventsDispatcher)
	gatewayHandler := gateway.NewGatewayHandler(gatewayService, CASigner)

	//Connectors routes
	connectorService := connector.NewService(respositories.Connector, respositories.PKICA, respositories.Event, respositories.Gateway, eventsDispatcher)
	connectorHandler := connector.NewConnectorHandler(connectorService, CASigner)

	updateStaleGatewayConnector(context.Background(), respositories.Gateway, respositories.Connector, log)

	// Policy routes
	policyService := policy.NewService(respositories.Policy, policyDistributor)
	policyHandler := policy.NewPolicyHandler(policyService)

	// Logs routes
	logsService := logs.NewService(respositories.Log)
	logsHandler := logs.NewLogsHandler(logsService)

	r.Handle("/static/*", idpHandler.StaticHandler())

	//Docs - date this in prod
	r.Get("/docs/*", httpSwagger.Handler(
		httpSwagger.URL("/docs/doc.json"),
	))

	r.Route("/authorize", idpHandler.IdentityAuthRoutes)

	r.Route("/api/v1", func(r chi.Router) {

		r.Group(func(r chi.Router) {
			r.Route("/auth", authHandler.Routes)
			r.Route("/internal/gateways", gatewayHandler.WithoutAuthRoutes)
			r.Route("/internal/connectors", connectorHandler.WithoutAuthRoutes)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware([]byte(cfg.JwtAccessSecret)))
			r.Route("/idp-configs", idpHandler.IdentityRoutes)
			r.Route("/gateways", gatewayHandler.WithAuthRoutes)
			r.Route("/orgs", orgHandler.Routes)
			r.Route("/connectors", connectorHandler.WithAuthRoutes)
			r.Route("/apps", appHandler.Routes)
			r.Route("/policies", policyHandler.Routes)
			r.Route("/logs", logsHandler.WithAuthRoutes)
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

func updateStaleGatewayConnector(ctx context.Context, gatewayRepo gateway.Repository, connectorRepo connector.Repository, log *slog.Logger) {
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		log.Info("Started Checking And Updating Stale Gateways and Connector")
		for range ticker.C {
			gateway.CheckAndUpdateGatewayStatus(ctx, gatewayRepo, log)
			connector.CheckAndUpdateConnectorStatus(ctx, connectorRepo, log)
		}
	}()
}
