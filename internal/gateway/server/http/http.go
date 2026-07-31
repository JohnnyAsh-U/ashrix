package http_proxy

import (
	"context"
	"encoding/json"
	"strings"

	// "crypto"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	gateway_grpc "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/grpc_client"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http        *http.Server
	redisClient *redis.Client
	log         *zap.Logger
	grpcClient  *gateway_grpc.SafeClient
}

func NewProxyServer(cfg *config.Config, grpcClient *gateway_grpc.SafeClient, registry *registry.Registry, redisClient *redis.Client, log *zap.Logger) *ProxyServer {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	sessions := session.NewSessionManager(redisClient, cfg.SessionTTL, cfg.CookieSecure)

	rateLimiter := session.NewRedisLimiter(redisClient, 10, time.Minute)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})

	r.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
		sessions.Destroy(w, r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})

	r.Get("/_auth/callback", func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.RemoteAddr
		if !rateLimiter.Allow(r.Context(), clientIP) {
			http.Error(w, "Too Many Request", http.StatusTooManyRequests)
			return
		}

		//Read the user cookie
		stateCookie, err := r.Cookie("state")
		if err != nil || stateCookie.Value == "" {
			http.Error(w, "Missing Parameter", http.StatusBadRequest)
			return
		}

		//Verify and consule state
		stateKey := fmt.Sprintf("oauth_state:%s", stateCookie)
		stateData, err := redisClient.GetDel(r.Context(), stateKey).Result()
		if err != nil {
			http.Error(w, "Missing Parameter", http.StatusBadRequest)
			return
		}

		var statePayload map[string]string
		json.Unmarshal([]byte(stateData), &statePayload)
		redirectURI := statePayload["redirect_uri"]

		//Exchange token with CP via mTLS grpc
		tokenStateFromCP := r.URL.Query().Get("state")

		tokenHash := crypto.HashToken(tokenStateFromCP)

		protoIdentity, err := grpcClient.ExchangeToken(r.Context(), &gen.ExchangeTokenRequest{
			TokenHash:   tokenHash,
			GatewayName: cfg.GatewayName,
		})

		if err != nil {
			http.Error(w, "Invalid Token", http.StatusBadRequest)
			return
		}

		if err := sessions.Create(w, r, protoIdentity); err != nil {
			log.Error("Session Creation Failed", zap.String("err", err.Error()))
			http.Error(w, "Internal Error", http.StatusNotFound)
			return
		}

		log.Info("Session Creation")
		http.Redirect(w, r, redirectURI, http.StatusTemporaryRedirect)
	})

	r.Group(func(r chi.Router) {
		r.Use(requireAuth(registry, sessions, redisClient, cfg, log))

		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {

			// Defensive nil checks to avoid runtime panics
			if registry == nil {
				if log != nil {
					log.Error("registry is nil in proxy handler")
				}
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if log == nil {
				// No logger available; fail safely
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			// 1. Extract hostname
			// 2. Identify Target Org/App
			// 3. Verify Session Cookie
			// 4. Check Policy
			// 5. Proxy to Connector gRPC stream
			entry, ok := registry.GetByConnectorID("4b9c6b29-997b-4eb2-bdc5-e35413731ccd")
			if !ok || entry == nil {
				log.Warn("no connector for this subdomain")
				http.Error(w, "Application not found", http.StatusNotFound)
				return
			}

			if entry.TunnelSession == nil {
				log.Warn("connector entry missing TunnelSession", zap.String("connector_id", entry.ConnectorID))
				http.Error(w, "connector unavailable", http.StatusBadGateway)
				return
			}

			stream, err := entry.TunnelSession.OpenStream()
			if err != nil {
				log.Warn("Failed to open tunnel stream", zap.String("connector_id", entry.ConnectorID), zap.Error(err))
				http.Error(w, "tunnel error", http.StatusBadGateway)
				return
			}
			defer stream.Close()

			//Write request envelope + body to stream
			envelope := gen.RequestHeader{
				Method:     r.Method,
				AppId:      "367595d8-30c4-4b87-8660-ec1862d8c138",
				Path:       r.URL.Path,
				Query:      r.URL.RawQuery,
				UserId:     "user-123",
				UserEmail:  "user@example.com",
				Headers:    flattenHeaders(r.Header),
				RequestId:  "req-456",
				BodyLength: r.ContentLength,
			}

			if err := writeEvelope(stream, &envelope); err != nil {
				log.Error("Failed to write request envelope", zap.Error(err))
				http.Error(w, "failed to write request envelope", http.StatusInternalServerError)
				return
			}

			if r.ContentLength > 0 {
				if _, err := io.Copy(stream, r.Body); err != nil {
					log.Error("Failed to write request body", zap.Error(err))
					http.Error(w, "failed to write request body", http.StatusInternalServerError)
					return
				}
			}

			//Read response envelope + body from stream
			var respEnvelope gen.ResponseHeader
			if err := readEnvelope(stream, &respEnvelope); err != nil {
				log.Error("Failed to read response envelope", zap.Error(err))
				http.Error(w, "failed to read response envelope", http.StatusInternalServerError)
				return
			}

			// Validate status code from connector
			status := int(respEnvelope.StatusCode)
			if status < 100 || status > 599 {
				log.Warn("invalid status code from connector, mapping to 502", zap.Int32("status", respEnvelope.StatusCode))
				status = http.StatusBadGateway
			}

			for key, value := range respEnvelope.Headers {
				w.Header().Set(key, value)
			}

			// Write status header
			w.WriteHeader(status)

			// Determine whether the client expects/accepts a response body.
			allowBody := true
			if r.Method == http.MethodHead {
				allowBody = false
			}
			if status >= 100 && status < 200 {
				allowBody = false
			}
			if status == http.StatusNoContent || status == http.StatusNotModified {
				allowBody = false
			}

			// If body is allowed, stream it to the client; otherwise drain the stream
			// so the connector can finish writing/close the stream without blocking.
			if allowBody {
				if _, err := io.Copy(w, stream); err != nil {
					log.Error("Failed to write response body", zap.Error(err))
				}
			} else {
				// Drain at most 1MB of the discarded body with a 2-second timeout to prevent hangs
				drainDone := make(chan struct{})
				go func() {
					_, _ = io.Copy(io.Discard, io.LimitReader(stream, 1024*1024))
					close(drainDone)
				}()
				select {
				case <-drainDone:
				case <-time.After(2 * time.Second):
					log.Warn("Draining response body timed out, closing stream")
				}
			}

			// w.WriteHeader(http.StatusNotImplemented)
			// w.Write([]byte("Ashrix Gateway: Proxy logic pending connector integration"))
		})
	})

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
		// redisClient: ,
	}
}

func (s *ProxyServer) Start() error {
	s.log.Info("Gateway Http Proxy Server starting", zap.String("addr", s.http.Addr))
	// In production, this uses s.http.ListenAndServeTLS("", "")
	// with the Gateway's certificate identity.
	return s.http.ListenAndServe()
}

// Middleware for the Auth
func requireAuth(registry *registry.Registry, session *session.SessionManager, redisClient *redis.Client, cfg *config.Config, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			fmt.Println(r.Host, r.URL.RequestURI())
			fmt.Println("From Require Auth")
			//Get the subdomain from the url
			parts := strings.Split(r.Host, ".")
			var subdomain string
			if len(parts) > 2 {
				subdomain = strings.Join(parts[:len(parts)-2], ".")
			}

			//Get the Connector Registry by subdomain
			connector, exists := registry.GetBySubdomain(subdomain)
			if !exists {
				logger.Error("Connection NOT FOUND")
				http.Error(w, "NOT FOUND", http.StatusNotFound)
				return
			}

			if !connector.IsRoutable() {
				logger.Error("Connection Failed")
				http.Error(w, "Connection NOT FOUND", http.StatusNotFound)
				return
			}

			var foundApp *gen.ConnectorApps

			//find the app,
			for _, app := range connector.Apps {
				if app.Subdomain == subdomain {
					foundApp = app
					break
				}
			}

			if foundApp == nil {
				logger.Error("No App")
				http.Error(w, "App Not Found", http.StatusNotFound)
				return
			}

			//Check if public no session needed
			if foundApp.IsPublic {
				next.ServeHTTP(w, r)
				return
			}

			identity, err := session.Get(r)

			if err == nil {
				ctx := context.WithValue(r.Context(), "identity", identity)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			//No Session: Initialte auth flow
			state, err := crypto.GenerateToken(32)
			if err != nil {
				logger.Error("Generate State Failed", zap.String("error", err.Error()))
				http.Error(w, "Internal Error", http.StatusInternalServerError)
				return
			}

			//Set the present app url as redirect uri
			redirectData, _ := json.Marshal(map[string]string{
				"redirect_uri": r.URL.RequestURI(),
			})

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			if err := redisClient.Set(ctx, fmt.Sprintf("oauth_state:%s", state), redirectData, 10*time.Minute).Err(); err != nil {
				logger.Error("Store State Failed", zap.String("error", err.Error()))
				http.Error(w, "Internal Error", http.StatusInternalServerError)
				return
			}

			//Set the headers
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-ID", connector.TenantID)
			w.Header().Set("X-App-ID", foundApp.Id)
			w.Header().Set("X-Gateway-URI", cfg.GatewayUrl)

			//Set the state in the cookies
			http.SetCookie(w, &http.Cookie{
				Name:     "state",
				Value:    state,
				Path:     "/",
				MaxAge:   int(600),
				HttpOnly: true,
				Secure:   cfg.CookieSecure,
				SameSite: http.SameSiteLaxMode,
			})

			//Build the cp auth url
			authUrl := fmt.Sprintf("%s/api/v1/authorize/providers", cfg.CPURL)

			http.Redirect(w, r, authUrl, http.StatusTemporaryRedirect)
		})
	}
}
