package session

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

func TestAppAccessMiddlewarePublicAndTokenProtectedApps(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reg := registry.New(logger)
	apps := []*proto.ConnectorApps{
		{Id: "public-app", Name: "Public", Subdomain: "public", IsPublic: true},
		{Id: "private-app", Name: "Private", Subdomain: "private"},
	}
	reg.AttachManagement("connector-1", "tenant-1", apps, &registry.ManagementSession{}, nil, "active")

	token := "a-long-static-application-token"
	digest := sha256.Sum256([]byte(token))
	cfg := &config.Config{AppTokenHashes: map[string]string{
		"private-app": hex.EncodeToString(digest[:]),
	}}
	middleware := AppAccessMiddleware(reg, cfg, web.NewErrorHandler(), logger)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-App-ID", AppIDFromCtx(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name       string
		host       string
		authority  string
		wantStatus int
		wantAppID  string
	}{
		{name: "public app", host: "public.example.test", wantStatus: http.StatusNoContent, wantAppID: "public-app"},
		{name: "private app without token", host: "private.example.test", wantStatus: http.StatusUnauthorized},
		{name: "private app invalid token", host: "private.example.test", authority: "Bearer invalid", wantStatus: http.StatusUnauthorized},
		{name: "private app valid token", host: "private.example.test", authority: "Bearer " + token, wantStatus: http.StatusNoContent, wantAppID: "private-app"},
		{name: "unknown app", host: "missing.example.test", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://"+test.host+"/", nil)
			req.Host = test.host
			if test.authority != "" {
				req.Header.Set("Authorization", test.authority)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if got := recorder.Header().Get("X-Test-App-ID"); got != test.wantAppID {
				t.Fatalf("forwarded app ID = %q, want %q", got, test.wantAppID)
			}
		})
	}
}
