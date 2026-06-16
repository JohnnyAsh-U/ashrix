package middleware

import (
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
)

//Internal only middleware blocks any request that caries a JWT Bearer token
func InternalOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		auth := r.Header.Get("Authorization")

		//Reject if a bearer jwt is presented = internal endpoint
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			dto.SendError(w, dto.NewForbiddenError(nil))
			return
		}
		next.ServeHTTP(w,r)
	})
}