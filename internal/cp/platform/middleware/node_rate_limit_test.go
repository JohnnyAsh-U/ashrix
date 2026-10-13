package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestNodeAttemptRateLimitPerNode(t *testing.T) {
	store := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: store.Addr()})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	middleware := NodeAttemptRateLimit(client, logger, "connector", func(r *http.Request, _ []byte) string {
		return r.Header.Get("X-Node-ID")
	})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for range maxNodeAttemptsPerHour {
		req := httptest.NewRequest(http.MethodPost, "/internal/connectors/renew", strings.NewReader(`{"ok":true}`))
		req.RemoteAddr = "192.0.2.10:12345"
		req.Header.Set("X-Node-ID", "node-1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		require.Equal(t, http.StatusOK, response.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/connectors/renew", strings.NewReader(`{"ok":true}`))
	req.RemoteAddr = "192.0.2.10:12345"
	req.Header.Set("X-Node-ID", "node-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	require.Equal(t, http.StatusTooManyRequests, response.Code)
	require.NotEmpty(t, response.Header().Get("Retry-After"))
}

func TestNodeAttemptRateLimitPerIPAndReplaysBody(t *testing.T) {
	store := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: store.Addr()})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	middleware := NodeAttemptRateLimit(client, logger, "gateway", func(r *http.Request, _ []byte) string {
		return r.Header.Get("X-Node-ID")
	})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, `{"ok":true}`, string(body))
		w.WriteHeader(http.StatusOK)
	}))

	for attempt := range maxIPAttemptsPerMinute {
		req := httptest.NewRequest(http.MethodPost, "/internal/gateways/enroll", strings.NewReader(`{"ok":true}`))
		req.RemoteAddr = "192.0.2.20:12345"
		req.Header.Set("X-Node-ID", "node-"+strconv.Itoa(attempt))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		require.Equal(t, http.StatusOK, response.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/gateways/enroll", strings.NewReader(`{"ok":true}`))
	req.RemoteAddr = "192.0.2.20:12345"
	req.Header.Set("X-Node-ID", "node-21")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	require.Equal(t, http.StatusTooManyRequests, response.Code)
	require.NotEmpty(t, response.Header().Get("Retry-After"))
}
