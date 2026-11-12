package http_proxy

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	// "github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// Middleware for the Auth
func AppAccessMiddleware(registry *registry.Registry, cfg *config.Config, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Skip posture collection for health checks and auth callbacks
			if isInternalPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

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
				http.Error(w, "No found", 400)
				return
			}

			accessCtx := logging.AccessContextFromContext(r.Context())
			if accessCtx != nil {
				accessCtx.ConnectorID = connector.ConnectorID
			}

			// if !connector.IsRoutable() {
			// 	logger.Error("Connection Failed")
			// 	http.Error(w, "Connection NOT FOUND", http.StatusNotFound)
			// 	return
			// }

			var foundApp *gen.ConnectorApps

			//find the app,
			for _, app := range connector.Apps {
				if app.Name == subdomain {
					foundApp = app
					break
				}
			}

			if foundApp == nil {
				logger.Error("No App")
				http.Error(w, "No found", 400)
				return
			}

			if accessCtx != nil {
				accessCtx.AppID = foundApp.Id
				accessCtx.AppName = foundApp.Name
			}

			//Add Connector And App ID to the Context
			// Inject claims into context for downstream handlers.
			ctx := context.WithValue(r.Context(), ConnectorID, connector.ConnectorID)
			ctx = context.WithValue(ctx, AppID, foundApp.Id)
		
			//Check if public no session needed
			// if foundApp.IsPublic {
				next.ServeHTTP(w, r.WithContext(ctx))
			// 	return
			// }

			
			
			
		
		})
	}
}

func isInternalPath(path string) bool {
	return path == "/_ashrix/health" || path == "/_ashrix/logout" ||
		strings.HasPrefix(path, "/_ashrix/auth/") || strings.HasPrefix(path, "/_ashrix/static/")
}
