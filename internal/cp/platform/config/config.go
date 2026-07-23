package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config for PKI
type BackendType string

const (
	BackendDisk BackendType = "disk"
	BackendKMS  BackendType = "kms"
)

type PKIConfig struct {
	Backend BackendType

	// Disk
	BasePath        string
	PKIUnlockSecret string

	// KMS
	KMSToken string
	KMSUrl   string

	//Idp secret encrytion key
	IDPSecretEncryptionKey string

	//IdentityJWT Keys
	
}

// Config holds all runtime configuration for the control plane.
// All values come from environment variables.
// Never hardcode secrets or URLs.
type Config struct {
	// Server
	HTTPAddr string // e.g. 0.0.0.0:8080
	GRPCAddr string // e.g. 0.0.0.0:9443
	ENV      string // development | production

	// Database
	DatabaseURL     string // postgres://user:pass@host:5432/ashrix?sslmode=require
	DatabaseMaxConn int32  //max number of db conn

	// Crypto
	// EncryptionKey is used for AES-256-GCM encryption of IdP client secrets.
	// Must be exactly 32 bytes, base64-encoded in the environment.
	EncryptionKey string

	// Session
	SessionTTL time.Duration // default: 8h

	// Ashrix base domain
	// Used to build app URLs: {app}.{org}.{BaseDomain}
	BaseDomain string // e.g. ashrix.io

	// Email (for admin invites and alerts)
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	//Jwt Config
	JwtAccessSecret  string
	JwtAccessTTL     time.Duration
	JwtRefreshSecret string
	JwtRefreshTTL    time.Duration

	// Auth Config
	ResetTokenDuration time.Duration
	SetupTokenDuration time.Duration
	OtpTokenDuration   time.Duration
	AppUrl string

	//Redis
	RedisAddr string
	RedisDB int
	RedisPassword string
	RedisPoolSize int

	// PKI
	PKIConfig *PKIConfig
}

// Load reads config from environment and validates required fields.
// Returns an error listing all missing/invalid values — not just the first.
func Load() (*Config, []error) {
	var errs []error

	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("required env var %s is not set", key))
		}
		return v
	}

	optional := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	mustDuration := func(s string) time.Duration {
		d, err := time.ParseDuration(s)
		if err != nil {
			panic("invalid duration: " + s)
		}
		return d
	}

	mustInt := func(s string) int {
		n, err := strconv.Atoi(s)
		if err != nil {
			panic("invalid int: " + s)
		}
		return n
	}

	pkiConfig := &PKIConfig{
		Backend:         BackendType(optional("PKI_BACKEND", "disk")),
		BasePath:        optional("PKI_BASE_PATH", "~/.ashrix/pki"),
		PKIUnlockSecret: optional("PKI_UNLOCK_SECRET", "ashrix-pki-unlock-must-be-32-bytes-long"),
		KMSToken:        optional("PKI_KMS_TOKEN", ""),
		KMSUrl:          optional("PKI_KMS_URL", "http://localhost:8200"),
		IDPSecretEncryptionKey: optional("IDP_SECRET_KEY", "ashrix-pki-unlock-must-be-32-bytes-long"),
	}

	cfg := &Config{
		HTTPAddr:         optional("HTTP_ADDR", "0.0.0.0:8080"),
		GRPCAddr:         optional("GRPC_ADDR", "0.0.0.0:9443"),
		ENV:              optional("ENV", "development"),
		DatabaseURL:      require("DATABASE_URL"),
		DatabaseMaxConn:  int32(mustInt(optional("DB_MAX_CONN", "15"))),
		EncryptionKey:    require("ENCRYPTION_KEY"),
		SessionTTL:       mustDuration(optional("SESSION_TTL", "8h")),
		BaseDomain:       require("BASE_DOMAIN"),
		SMTPHost:         optional("SMTP_HOST", ""),
		SMTPUser:         optional("SMTP_USER", ""),
		SMTPPassword:     optional("SMTP_PASSWORD", ""),
		SMTPFrom:         optional("SMTP_FROM", "noreply@ashrix.io"),
		SMTPPort:         int(mustInt(optional("SMTP_PORT", "587"))),
		JwtAccessSecret:  require("JWT_ACCESS_SECRET"),
		JwtAccessTTL:     mustDuration(optional("JWT_ACCESS_TTL", "100m")),
		JwtRefreshSecret: require("JWT_REFRESH_SECRET"),
		JwtRefreshTTL:    mustDuration(optional("JWT_REFRESH_TTL", "720h")),
		ResetTokenDuration: mustDuration(optional("RESET_TOKEN_DURATION", "1h")),
		SetupTokenDuration: mustDuration(optional("SETUP_TOKEN_DURATION", "1h")),
		OtpTokenDuration:   mustDuration(optional("OTP_TOKEN_DURATION", "5m")),
		AppUrl: optional("APP_URL", "http://localhost:8080"),

		RedisAddr: optional("REDIS_ADDR", "localhost:6379"),
		RedisDB: int(mustInt(optional("REDIS_DB", "0"))),
		RedisPassword: optional("REDIS_PASSWORD", ""),
		RedisPoolSize: int(mustInt(optional("REDIS_POOL_SIZE", "2"))),

		PKIConfig:        pkiConfig,
	}

	return cfg, errs
}
