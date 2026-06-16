package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/auth"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/org"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Server wraps the HTTP server and all dependencies.
type Server struct {
	http *http.Server
	log  *slog.Logger
}

// New wires together all dependencies and builds the router.
func InitializeHttpServer(cfg *config.Config, db *pgxpool.Pool, log *slog.Logger) *Server {

	// Store layer — sqlc generated queries
	queries := store.New(db)
	//Mailer config
	mailer := utils.NewMailerService(cfg)
	//Jwt Utils
	jwtUtils := utils.NewJwtUtils(
		cfg.JwtAccessSecret,
		cfg.JwtRefreshSecret,
		cfg.JwtAccessTTL,
		cfg.JwtRefreshTTL,
	)
	fmt.Println(cfg.JwtAccessSecret, cfg.JwtRefreshSecret)

	// Router
	r := chi.NewRouter()

	// ── Global middleware ──────────────────────────────────────
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(middleware.LoggingMiddleware(log))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)

	//Org routes
	orgRepo := org.NewPostgresRepository(queries)
	orgService := org.NewService(orgRepo)
	orgHandler := org.NewOrgHandler(orgService)

	//Auth routes
	authRepo := auth.NewPostgresRepository(queries)
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

	r.Get("/healthz", handleHealth)

	//Docs - date this in prod
	r.Get("/docs/*", httpSwagger.Handler(
		httpSwagger.URL("/docs/doc.json"),
	))

	// Auth routes
	r.Route("/api/v1/auth", authHandler.Routes)

	//Orgs
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware([]byte(cfg.JwtAccessSecret)))

		r.Route("/orgs", orgHandler.Routes)
	})

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
