package session

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

func AppAccessMiddleware(reg *registry.Registry, cfg *config.Config, errorHandler *web.ErrorHandler, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/_ashrix/health" || strings.HasPrefix(r.URL.Path, "/_ashrix/static/") {
				next.ServeHTTP(w, r)
				return
			}

			host := r.Host
			if hostOnly, _, err := strings.Cut(host, ":"); err {
				host = hostOnly
			}
			parts := strings.Split(host, ".")
			if len(parts) < 3 {
				errorHandler.ErrorPage(w, http.StatusNotFound, "App Not Found", "The application you requested could not be found.", "")
				return
			}
			subdomain := strings.Join(parts[:len(parts)-2], ".")

			connector, exists := reg.GetBySubdomain(subdomain)
			if !exists {
				errorHandler.ErrorPage(w, http.StatusNotFound, "App Not Found", "The application you requested could not be found.", "")
				return
			}

			var app *proto.ConnectorApps
			for _, candidate := range connector.Apps {
				if candidate.Subdomain == subdomain {
					app = candidate
					break
				}
			}
			if app == nil {
				errorHandler.ErrorPage(w, http.StatusNotFound, "App Not Found", "The application you requested could not be found.", "")
				return
			}

			accessCtx := logging.AccessContextFromContext(r.Context())
			if accessCtx != nil {
				accessCtx.ConnectorID = connector.ConnectorID
				accessCtx.AppID = app.Id
				accessCtx.AppName = app.Name
			}

			ctx := context.WithValue(r.Context(), ConnectorID, connector.ConnectorID)
			ctx = context.WithValue(ctx, AppID, app.Id)
			ctx = context.WithValue(ctx, AppIsPublic, app.IsPublic)
			ctx = context.WithValue(ctx, AppEnableSecurityHeaders, app.EnableSecurityHeaders)

			if !app.IsPublic && !validAppToken(r.Header.Get("Authorization"), cfg.AppTokenHashes[app.Id]) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="Ashrix"`)
				errorHandler.ErrorPage(w, http.StatusUnauthorized, "Unauthorized", "A valid application token is required.", "")
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validAppToken(header, expectedHash string) bool {
	if len(expectedHash) != sha256.Size*2 {
		return false
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	digest := sha256.Sum256([]byte(parts[1]))
	actualHash := hex.EncodeToString(digest[:])
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}
