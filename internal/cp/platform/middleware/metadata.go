package middleware

import (
    "context"
    "net"
    "net/http"
    "strings"
)

type contextKey string

const (
    UserAgentKey contextKey = "userAgent"
    ClientIPKey  contextKey = "clientIP"
)

func UserAgent(ctx context.Context) string {
    if ua, ok := ctx.Value(UserAgentKey).(string); ok {
        return ua
    }
    return ""
}

func ClientIP(ctx context.Context) string {
    if ip, ok := ctx.Value(ClientIPKey).(string); ok {
        return ip
    }
    return ""
}

func Metadata(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ua := r.UserAgent()
        ip := clientIP(r)

        ctx := context.WithValue(r.Context(), UserAgentKey, ua)
        ctx = context.WithValue(ctx, ClientIPKey, ip)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func clientIP(r *http.Request) string {
    // If behind a reverse proxy
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        parts := strings.Split(xff, ",")
        return strings.TrimSpace(parts[0])
    }

    if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
        return xrip
    }

    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err == nil {
        return host
    }

    return r.RemoteAddr
}
