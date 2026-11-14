package proxy

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/tunnel"
)

// Middleware for the Auth
func AppAccessMiddleware(quicTunnel *tunnel.QuicTunnel, cfg *config.Config, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Skip posture collection for health checks and auth callbacks
			if isInternalPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			//Get the domain host name
			host, _, _ := strings.Cut(r.Host, ":") //To remove port if any

			//Get the Connector Registry by subdomain
			connector, exists := quicTunnel.GetConnectorSessionBySubdomain(host)
			if !exists {
				logger.Error("Connection NOT FOUND")
				http.Error(w, "No found", 400)
				return
			}

			accessCtx := logging.AccessContextFromContext(r.Context())
			if accessCtx != nil {
				accessCtx.ConnectorID = connector.ConnectorID
			}

			var foundApp *config.Application

			//find the app,
			for _, app := range cfg.Applications {
				if host == app.Domain {
					foundApp = &app
					break
				}
			}

			if foundApp == nil {
				logger.Error("No App", "host", host)
				http.Error(w, "No found", 404)
				return
			}

			if accessCtx != nil {
				accessCtx.AppID = foundApp.Name
				accessCtx.AppName = foundApp.Name
			}

			ctx := context.WithValue(r.Context(), ConnectorID, connector.ConnectorID)
			ctx = context.WithValue(ctx, AppID, foundApp.Name)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isInternalPath(path string) bool {
	return path == "/_ashrix/health" || path == "/_ashrix/logout" ||
		strings.HasPrefix(path, "/_ashrix/auth/") || strings.HasPrefix(path, "/_ashrix/static/")
}
