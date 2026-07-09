package http_proxy

import (
	"fmt"
	"io"

	// "io"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http *http.Server
	log  *zap.Logger
}

func NewProxyServer(cfg *config.Config, registry *registry.Registry, log *zap.Logger) *ProxyServer {
	mux := http.NewServeMux()

	// Central proxy handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Defensive nil checks to avoid runtime panics
		if registry == nil {
			if log != nil {
				log.Error("registry is nil in proxy handler")
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if log == nil {
			// No logger available; fail safely
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// 1. Extract hostname
		// 2. Identify Target Org/App
		// 3. Verify Session Cookie
		// 4. Check Policy
		// 5. Proxy to Connector gRPC stream
		entry, ok := registry.GetByConnectorID("4b9c6b29-997b-4eb2-bdc5-e35413731ccd")
		if !ok || entry == nil {
			log.Warn("no connector for this subdomain")
			http.Error(w, "Application not found", http.StatusNotFound)
			return
		}

		if entry.TunnelSession == nil {
			log.Warn("connector entry missing TunnelSession", zap.String("connector_id", entry.ConnectorID))
			http.Error(w, "connector unavailable", http.StatusBadGateway)
			return
		}

		stream, err := entry.TunnelSession.OpenStream()
		if err != nil {
			log.Warn("Failed to open tunnel stream", zap.String("connector_id", entry.ConnectorID), zap.Error(err))
			http.Error(w, "tunnel error", http.StatusBadGateway)
			return
		}
		defer stream.Close()

		//Write request envelope + body to stream
		envelope := gen.RequestHeader{
			Method:     r.Method,
			AppId:      "ECCLESIX",
			Path:       r.URL.Path,
			Query:      r.URL.RawQuery,
			UserId:     "user-123",
			UserEmail:  "user@example.com",
			Headers:    flattenHeaders(r.Header),
			RequestId:  "req-456",
			BodyLength: r.ContentLength,
		}

		if err := writeEvelope(stream, &envelope); err != nil {
			log.Error("Failed to write request envelope", zap.Error(err))
			http.Error(w, "failed to write request envelope", http.StatusInternalServerError)
			return
		}

		if r.ContentLength > 0 {
			if _, err := io.Copy(stream, r.Body); err != nil {
				log.Error("Failed to write request body", zap.Error(err))
				http.Error(w, "failed to write request body", http.StatusInternalServerError)
				return
			}
		}

		//Read response envelope + body from stream
		var respEnvelope gen.ResponseHeader
		if err := readEnvelope(stream, &respEnvelope); err != nil {
			log.Error("Failed to read response envelope", zap.Error(err))
			http.Error(w, "failed to read response envelope", http.StatusInternalServerError)
			return
		}

		// Validate status code from connector
		status := int(respEnvelope.StatusCode)
		if status < 100 || status > 599 {
			log.Warn("invalid status code from connector, mapping to 502", zap.Int32("status", respEnvelope.StatusCode))
			status = http.StatusBadGateway
		}

		for key, value := range respEnvelope.Headers {
			w.Header().Set(key, value)
		}

		// Write status header
		w.WriteHeader(status)

		// Determine whether the client expects/accepts a response body.
		allowBody := true
		if r.Method == http.MethodHead {
			allowBody = false
		}
		if status >= 100 && status < 200 {
			allowBody = false
		}
		if status == http.StatusNoContent || status == http.StatusNotModified {
			allowBody = false
		}

		// If body is allowed, stream it to the client; otherwise drain the stream
		// so the connector can finish writing/close the stream without blocking.
		if allowBody {
			if _, err := io.Copy(w, stream); err != nil {
				log.Error("Failed to write response body", zap.Error(err))
			}
		} else {
			if _, err := io.Copy(io.Discard, stream); err != nil {
				log.Error("Failed to drain response body", zap.Error(err))
			}
		}

		// w.WriteHeader(http.StatusNotImplemented)
		// w.Write([]byte("Ashrix Gateway: Proxy logic pending connector integration"))
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler: mux,
		// TLSConfig will be set by the main loop using GatewayPKI
	}

	return &ProxyServer{
		http: srv,
		log:  log,
	}
}

func (s *ProxyServer) Start() error {
	s.log.Info("Gateway Http Proxy Server starting", zap.String("addr", s.http.Addr))
	// In production, this uses s.http.ListenAndServeTLS("", "")
	// with the Gateway's certificate identity.
	return s.http.ListenAndServe()
}
