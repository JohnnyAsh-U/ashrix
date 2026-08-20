package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/auth"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	AdminID string `json:"admin_id"`
	OrgID   string `json:"org_id"`
	AdminEmail string `json:"email"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// ---- Context keys ----

type ctxKey string

const (
	CtxAdminID ctxKey = "admin_id"
	CtxOrgID   ctxKey = "org_id"
	CtxAdminEmail ctxKey = "email"
	CtxRole    ctxKey = "role"
)

// AdminIDFromCtx retrieves the authenticated admin ID from context.
// Returns empty string if not set — callers must handle this.
func AdminIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(CtxAdminID).(string)
	return id
}

// OrgIDFromCtx retrieves the authenticated org ID from context.
func OrgIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(CtxOrgID).(string)
	return id
}

// Role retrieve the authenticated Role from the context
func RoleFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(CtxRole).(string)
	return id
}

// Email retrieve the authenticated Email from the context
func EmailFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(CtxAdminEmail).(string)
	return id
}



func AuthMiddleware(jwtKey []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				dto.SendError(w, dto.NewUnauthorizedError("Not Authorized"))
				return
			}
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := ParseAccessToken(tokenString, jwtKey)
			if err != nil {
				dto.SendError(w, dto.NewForbiddenError("Invalid Token"))
				return
			}

			// Inject claims into context for downstream handlers.
			ctx := context.WithValue(r.Context(), CtxAdminID, claims.AdminID)
			ctx = context.WithValue(ctx, CtxOrgID, claims.OrgID)
			ctx = context.WithValue(ctx, CtxRole, claims.Role)
			ctx = context.WithValue(ctx, CtxAdminEmail, claims.AdminEmail)

			//Inject the clientIP and UserAgent
			

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireOrgAccess ensures the authenticated admin belongs to the org
// referenced in the URL parameter. Prevents cross-org resource access.
//
// Usage:
//
//	r.With(middleware.RequireOrgAccess).Get("/v1/orgs/{orgId}/apps", ...)
func RequireOrgAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := OrgIDFromCtx(r.Context())
		if orgID == "" {
			dto.SendError(w, dto.NewForbiddenError("Not Authorized For Admin without Org"))
			return
		}

		// The org ID in the URL must match the one in the token.
		// SECURITY: Never trust URL params for authorization decisions —
		// the token is the source of truth.
		urlOrgID := r.PathValue("orgId")
		if urlOrgID != "" && urlOrgID != orgID {
			// Return 404, not 403 — don't confirm the org exists to this caller.
			dto.SendError(w, dto.NewNotFoundError("Not Found"))
			return
		}

		next.ServeHTTP(w, r)
	})
}



func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{})

	for _, role := range allowedRoles {
		allowed[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromCtx(r.Context())
			if role == "" {
				dto.SendError(w, dto.NewForbiddenError("No Role"))
				return
			}

			if _, ok := allowed[role]; !ok {
				dto.SendError(w, dto.NewForbiddenError("Not Permitted"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}


func ParseAccessToken(tokenStr string, jwtKey []byte) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}

	return claims, nil
}
