package tunnel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:       100,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: true,
	},
}

// handleRequestStream reads a request from the stream,
// proxies it to the internal app, streams response back.
// Runs in its own goroutine — one per concurrent user request.
func (c *ConnectorTunnel) handleRequestStream(stream transport.Stream) {
	defer stream.Close()

	// ── Read request envelope ─────────────────────────────────────
	var envelope RequestEnvelope
	if err := json.NewDecoder(stream).Decode(&envelope); err != nil {
		c.log.Error("failed to read request envelope",
			zap.Error(err),
		)
		c.writeError(stream, 400, "bad request envelope")
		return
	}

	c.log.Debug("proxying request",
		zap.String("method", envelope.Method),
		zap.String("path", envelope.Path),
		zap.String("user", envelope.UserEmail),
		zap.String("request_id", envelope.RequestID),
	)

	upstreamHost := "localhost:3000"

	// ── Build upstream URL ────────────────────────────────────────
	upstream := fmt.Sprintf("http://%s%s", upstreamHost, envelope.Path)
	if envelope.Query != "" {
		upstream += "?" + envelope.Query
	}

	// ── Body reader ───────────────────────────────────────────────
	var bodyReader io.Reader
	if envelope.BodyLen > 0 {
		bodyReader = io.LimitReader(stream, envelope.BodyLen)
	}

	// ── Build HTTP request ────────────────────────────────────────
	req, err := http.NewRequest(envelope.Method, upstream, bodyReader)
	if err != nil {
		c.log.Error("failed to build request", zap.Error(err))
		c.writeError(stream, 500, "internal error")
		return
	}

	// Copy headers from envelope
	for k, v := range envelope.Headers {
		req.Header.Set(k, v)
	}

	// Inject verified identity headers
	// Internal app trusts these — they come from the connector,
	// not from the user directly
	req.Header.Set("X-Ashrix-User-ID",    envelope.UserID)
	req.Header.Set("X-Ashrix-User-Email", envelope.UserEmail)
	req.Header.Set("X-Ashrix-Request-ID", envelope.RequestID)

	// Strip headers user should not control
	req.Header.Del("X-Forwarded-For")
	req.Host = "localhost"

	// ── Call internal app ─────────────────────────────────────────
	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		c.log.Error("upstream unreachable",
			zap.Error(err),
		)
		c.writeError(stream, 502, "upstream unreachable")
		return
	}
	defer resp.Body.Close()

	// ── Write response envelope ───────────────────────────────────
	respEnvelope := ResponseEnvelope{
		StatusCode: resp.StatusCode,
		Headers:    flattenHeaders(resp.Header),
	}
	if err := json.NewEncoder(stream).Encode(respEnvelope); err != nil {
		c.log.Error("failed to write response envelope", zap.Error(err))
		return
	}

	// ── Stream response body ──────────────────────────────────────
	// Bytes flow: app → connector → stream → gateway → user
	// No buffering — works for any response size
	written, err := io.Copy(stream, resp.Body)

	c.log.Debug("request complete",
		zap.String("method", envelope.Method),
		zap.String("path", envelope.Path),
		zap.Int("status", resp.StatusCode),
		zap.Int64("bytes", written),
		zap.Duration("latency", time.Since(start)),
		zap.Error(err),
	)
}

func (c *ConnectorTunnel) writeError(stream transport.Stream, code int, msg string) {
	resp := ResponseEnvelope{
		StatusCode: code,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}
	data, _ := json.Marshal(resp)
	stream.Write(append(data, '\n'))
	body, _ := json.Marshal(map[string]string{"error": msg})
	stream.Write(body)
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vals := range h {
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	return out
}


