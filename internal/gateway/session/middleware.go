package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/redis/go-redis/v9"
)

// Middleware for the Auth
func SessionMiddleware(registry *registry.Registry, session *SessionManager, redisClient *redis.Client, cfg *config.Config, errHandler *web.ErrorHandler, logger *slog.Logger) func(http.Handler) http.Handler {
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
				errHandler.ErrorPage(w, http.StatusNotFound, "NOT FOUND", "The application you requested could not be found.", "")
				return 
			}

			// if !connector.IsRoutable() {
			// 	logger.Error("Connection Failed")
			// 	http.Error(w, "Connection NOT FOUND", http.StatusNotFound)
			// 	return
			// }

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
				errHandler.ErrorPage(w, http.StatusNotFound, "App Not Found", "The application you requested could not be found.", "")
				return
			}

			//Add Connector And App ID to the Context
			// Inject claims into context for downstream handlers.
			ctx := context.WithValue(r.Context(), ConnectorID, connector.ConnectorID)
			ctx = context.WithValue(ctx, AppID, foundApp.Id)
			ctx = context.WithValue(ctx, AppIsPublic, foundApp.IsPublic)



			//Check if public no session needed
			if foundApp.IsPublic {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			identity, sessionID, err := session.Get(r)

			if err == nil {
				ctx := context.WithValue(ctx, Identity, identity)
				ctx = context.WithValue(ctx, SessionID, sessionID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			//No Session: Initialte auth flow
			state, err := crypto.GenerateOnlyToken(32)
			if err != nil {
				logger.Error("Generate State Failed", slog.String("error", err.Error()))
				errHandler.ErrorPage(w, http.StatusInternalServerError, "Internal Error", "The gateway encountered an unexpected error.", "")
				return
			}

			//Set the present app url as redirect uri
			redirectData, _ := json.Marshal(map[string]string{
				"redirect_uri": fmt.Sprintf("http://%s%s", r.Host, r.URL.RequestURI()),
			})

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			if err := redisClient.Set(ctx, fmt.Sprintf("oauth_state:%s", state), redirectData, 10*time.Minute).Err(); err != nil {
				logger.Error("Store State Failed", slog.String("error", err.Error()))
				errHandler.ErrorPage(w, http.StatusInternalServerError, "Internal Error", "The gateway encountered an unexpected error.", "")
				return
			}

			//Set the params
			params := url.Values{}
			params.Set("aid", foundApp.Id)
			params.Set("gid", cfg.GatewayID)

			fmt.Println(connector.TenantID, foundApp.Id)
			//Set the state in the cookies
			http.SetCookie(w, &http.Cookie{
				Name:  "state",
				Value: state,
				Path:  "/",
				// Domain: cfg.GatewayUrl,
				Domain:   ".ashrix.io",
				MaxAge:   int(600),
				HttpOnly: true,
				Secure:   cfg.CookieSecure,
				SameSite: http.SameSiteLaxMode,
			})

			//Build the cp auth url
			authUrl := fmt.Sprintf("%s/authorize/providers?%s", cfg.CPURL, params.Encode())

			http.Redirect(w, r, authUrl, http.StatusTemporaryRedirect)
		})
	}
}

func isInternalPath(path string) bool {
	return path == "/_ashrix/health" || path == "/_ashrix/logout" ||
		strings.HasPrefix(path, "/_ashrix/auth/") || strings.HasPrefix(path, "/_ashrix/static/")
}



