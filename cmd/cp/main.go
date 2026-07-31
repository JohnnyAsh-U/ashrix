package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/JohnnyAsh-U/ashrix-api/cmd/cp/docs"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_http"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/redis"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/logger"
	"github.com/joho/godotenv"
)

// @title Ashrix Access API
// @version 1.0
// @description Secure Application Access
// @host localhost:8001
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Today — dev / early prod
	_ = godotenv.Load() // load .env in dev; in prod, env vars are set by the environment (e.g. Vault Agent)

	//Load Config Variable from env
	cfg, errs := config.Load()
	if errs != nil {
		fmt.Println("Error loading config:", errs)
		os.Exit(1)
	}

	// Initialize context
    // ctx, cancel := context.WithCancel(context.Background())
    // defer cancel()

	// Structured logger — JSON in production, text in dev
	log := logger.New(cfg.ENV)

	// Connect to Postgres
	db, err := database.Connect(context.Background(), cfg, log)
	if err != nil {
		log.Error("failed to connect to database", slog.String("err", err.Error()))
		os.Exit(1)
	}

	defer db.Close()
	log.Info("Database connected")

	//Initializing the dbQueries
	dbQueries := store.New(db)

	 // Initialize Redis
    redisStore, err := redis.NewRedisStore(
        context.Background(),
        cfg.RedisAddr,
        cfg.RedisPassword,
        cfg.RedisDB,
        cfg.RedisPoolSize,
    )

	log.Info("Redis connected")

    if err != nil {
        log.Error("Failed to connect to Redis: %w", slog.String("err", err.Error()))
    }
    defer redisStore.Close()

	//Initializing Directory for Ashrix
	home, err := os.UserHomeDir()
	if err != nil {
		log.Error("Failed to Get Home Directory: %w", slog.String("err", err.Error()))
	}
	BaseDir := filepath.Join(home, ".ashrix")
	log.Info("Ashrix base directory Initialized")
	if err := os.MkdirAll(BaseDir, 0700); err != nil {
		log.Error("Creating JWT directory: %w", slog.String("err",err.Error()))
		os.Exit(1) // Fail hard if Dir initialization fails
	}
	

	// Initializing PKI Root CA and Intermediate CA
	// Pass db.Queries to the PKI signer for database-backed CA certificate management
	signer, err := pki.NewSigner(BaseDir, cfg.PKIConfig, dbQueries)
	if err != nil {
		log.Error("Failed to Initialized PKI", slog.String("err", err.Error()))
		os.Exit(1) // Fail hard if PKI initialization fails
	}

	//Initialzing CP Key and Cert
	cppki, cperr := crypto.ControlPlanePKIIntializer(
		BaseDir,
		cfg.PKIConfig.PKIUnlockSecret,
		signer,
	)

	if cperr != nil {
		log.Error("Failed to inintialize CP PKI")
	}

	//Runing gRPC and Http concurrently
	quit := make(chan os.Signal, 2)

	// Build and start HTTP server
	httpServer := cp_http.InitializeHttpServer(BaseDir, cfg, dbQueries,redisStore, log, signer)

	// Build and start gRPC server
	grpcServer := cp_grpc.InitializeGRPCServer(cfg, cppki, log, redisStore.Client())

	// Graceful shutdown on SIGINT / SIGTERM
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("Control Plane Http Server Starting..", slog.String("addr", cfg.HTTPAddr))
		if err := httpServer.Start(); err != nil {
			log.Error("server error", slog.String("err", err.Error()))
			os.Exit(1)
		}
	}()

	go func() {
		log.Info("Control Plane gRPC Server Starting..", slog.String("addr", cfg.GRPCAddr))
		if err := grpcServer.Start(); err != nil {
			log.Error("gRPC server error", slog.String("err", err.Error()))
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("Shutting Down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("shutdown error", slog.String("err", err.Error()))
	}

	log.Info("Stopping gRPC server...")
	grpcServer.Stop()

	log.Info("shutdown complete")

	fmt.Println("Welcome to Ashrix Access")
}
