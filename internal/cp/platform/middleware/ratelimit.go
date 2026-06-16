package middleware

// import (
// 	"net/http"
// 	"time"
// 	"github.com/go-chi/httprate"
// )

// func RateLimitMiddleware() func(http.Handler) http.Handler {
// 	return httprate.Limit(100, 1*time.Minute, httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
// 		return r.RemoteAddr, nil
// 	}))
// }