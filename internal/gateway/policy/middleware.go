package policy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/posture"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type decisionContextKey struct{}



func PolicyMiddleware(engine *PolicyEngine, cfg *config.Config, log  *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appIsPublic := session.AppIsPublicFromCtx(r.Context())
			// Skip policy check for exempt paths
			if isInternalPath(r.URL.Path) || appIsPublic {
				next.ServeHTTP(w, r)
				return
			}

			// 1. Read session from context (set by SessionMiddleware)
			sess := session.IdentityFromCtx(r.Context())
			
			// 2. Read posture from context (set by PostureMiddleware)
			posture, _ := posture.FromContext(r.Context())
			// 3. Extract resource from request


			appID := session.AppIDFromCtx(r.Context())


			// 4. Build AuthorizationContext — ZERO I/O
			authCtx := AuthorizationContext{
				Principal: Principal{
					UserID:   sess.UserId,
					Name: sess.Name,
					Email:    sess.Email,
					Groups:   sess.Groups,
				},
				Device: DeviceContext{
					Posture: posture.Status,
					ID:      "", // no agent yet
				},
				Network: NetworkContext{
					SourceIP:  posture.SourceIP,
					Country:   posture.GeoCountry,
					IsTorExit: posture.IsTorExit,
				},
				Resource: Resource{
					AppID:  appID,
					Path:   r.URL.Path,
					Method: r.Method,
				},
				Time:      time.Now(),
				TenantID:  sess.TenantId,
				GatewayID: cfg.GatewayID,
				RequestID: middleware.GetReqID(r.Context()),
			}

			// 5. Evaluate — pure function, no I/O
			decision := engine.Evaluate(authCtx)

			// 6. Log every decision (success or failure)
			logPolicyDecision(r.Context(), decision, authCtx, log)

			// 7. Enforce
			if decision.Denied() {
				// Return structured error with debugging headers
				w.Header().Set("X-Ashrix-Decision", "DENY")
				w.Header().Set("X-Ashrix-Decision-Reason", decision.Reason)
				if decision.PolicyID != "" {
					w.Header().Set("X-Ashrix-Policy-ID", decision.PolicyID)
				}
				w.Header().Set("X-Ashrix-Policy-Version", fmt.Sprintf("%d", decision.PolicyVersion))

				http.Error(w, "access denied", http.StatusForbidden)
				return
			}

			// 8. Store decision in context for downstream (proxy can log which policy matched)
			ctx := context.WithValue(r.Context(), decisionContextKey{}, decision)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}


func isInternalPath(path string) bool {
	return path == "/health" || path == "/logout" ||
		strings.HasPrefix(path, "/_auth/")
}


func DecisionFromContext(ctx context.Context) (*Decision, bool) {
	dp, ok := ctx.Value(decisionContextKey{}).(*Decision)
	return dp, ok
}


func logPolicyDecision(ctx context.Context, decision Decision, authCtx AuthorizationContext, log *zap.Logger) {
	// Structured JSON log for observability
	logData := map[string]interface{}{
		"event":          "policy.decision",
		"request_id":     authCtx.RequestID,
		"gateway_id":     authCtx.GatewayID,
		"tenant_id":      authCtx.TenantID,
		"user_id":        authCtx.Principal.UserID,
		"app_id":         authCtx.Resource.AppID,
		"path":           authCtx.Resource.Path,
		"method":         authCtx.Resource.Method,
		"effect":         decision.Effect,
		"policy_id":      decision.PolicyID,
		"reason":         decision.Reason,
		"policy_version": decision.PolicyVersion,
		"country":        authCtx.Network.Country,
		"is_tor":         authCtx.Network.IsTorExit,
		"evaluated_at":   decision.EvaluatedAt,
	}
	// Use your structured logger (zap, slog, etc.)
	log.Info("policy decision", zap.Any("data", logData))
}
