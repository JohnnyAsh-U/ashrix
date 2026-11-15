package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLogMiddlewareRecordsRequestAndResponse(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Errorf("read request body: %v", err)
		}
		access := AccessContextFromContext(r.Context())
		access.AppID, access.AppName, access.ConnectorID = "docs", "Documents", "edge-01"
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})
	req := httptest.NewRequest(http.MethodPost, "http://docs.example.test/upload", strings.NewReader("payload"))
	req.Header.Set("User-Agent", "ashrix-test")
	rec := httptest.NewRecorder()
	AccessLogMiddleware(nil, "gateway-01", logger)(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || rec.Body.String() != "created" {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode access log: %v; output=%q", err, output.String())
	}
	for key, want := range map[string]any{"gateway_id": "gateway-01", "connector_id": "edge-01", "app_id": "docs", "app_name": "Documents", "method": "POST", "path": "/upload", "status": float64(http.StatusCreated), "bytes_in": float64(7), "bytes_out": float64(7), "user_agent": "ashrix-test"} {
		if entry[key] != want {
			t.Errorf("log field %s = %#v, want %#v", key, entry[key], want)
		}
	}
}

func TestAccessLogMiddlewareSkipsInternalPaths(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "http://gateway.example.test/_ashrix/health", nil)
	AccessLogMiddleware(nil, "gateway-01", logger)(next).ServeHTTP(httptest.NewRecorder(), req)
	if output.Len() != 0 {
		t.Fatalf("internal path unexpectedly logged: %s", output.String())
	}
}
