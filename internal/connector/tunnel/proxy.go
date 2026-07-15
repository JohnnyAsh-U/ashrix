package tunnel

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
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
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in handleRequestStream", zap.Any("panic", r))
			c.writeError(stream, 500, "internal server error due to panic")
		}
	}()

	// ── Read request envelope (length-prefixed JSON) ──────────────
	var envelope gen.RequestHeader
	if err := readEnvelope(stream, &envelope); err != nil {
		c.log.Error("failed to read request envelope",
			zap.Error(err),
		)
		c.writeError(stream, 400, "bad request envelope")
		return
	}

	//===============Getting the app from the applist using requestheader app id================//
	var requestApp *gen.ConnectorApps
	for i := range c.apps {
		if c.apps[i].Id == envelope.AppId {
			requestApp = c.apps[i]
			break
		}
	}

	if requestApp == nil {
		c.log.Error("App Not Found")
		c.writeError(stream, 502, "App Not Found")
		return
	}

	c.log.Debug("proxying request",
		zap.String("method", envelope.Method),
		zap.String("path", envelope.Path),
		zap.String("user", envelope.UserEmail),
		zap.String("appName", requestApp.Name),
	)


	// ── Build upstream URL ────────────────────────────────────────
	upstream := fmt.Sprintf("%s%s", requestApp.Upstream, envelope.Path)
	if envelope.Query != "" {
		upstream += "?" + envelope.Query
	}

	fmt.Println(upstream)

	// ── Body reader ───────────────────────────────────────────────
	var bodyReader io.Reader
	switch {
	case envelope.BodyLength < 0:
		c.log.Error("invalid body length", zap.Int64("length", envelope.BodyLength))
		c.writeError(stream, 400, "invalid body length")
		return
	case envelope.BodyLength == 0:
		bodyReader = nil
	default:
		bodyReader = io.LimitReader(stream, envelope.BodyLength)
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
	req.Header.Set("X-Ashrix-User-ID", envelope.UserId)
	req.Header.Set("X-Ashrix-User-Email", envelope.UserEmail)
	// req.Header.Set("X-Ashrix-Request-ID", envelope.RequestID)

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
	// respEnvelope := gen.ResponseHeader{
	// 	StatusCode: int32(resp.StatusCode),
	// 	Headers:    flattenHeaders(resp.Header),
	// }
	// if err := json.NewEncoder(stream).Encode(&respEnvelope); err != nil {
	// 	c.log.Error("failed to write response envelope", zap.Error(err))
	// 	return
	// }
	respEnvelope := gen.ResponseHeader{
		StatusCode: int32(resp.StatusCode),
		Headers:    flattenHeaders(resp.Header),
	}
	if err := writeEnvelope(stream, &respEnvelope); err != nil {
		c.log.Error("failed to write response envelope",
			zap.Error(err),
		)
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
	resp := gen.ResponseHeader{
		StatusCode: int32(code),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}
	// write length-prefixed response envelope
	if err := writeEnvelope(stream, &resp); err != nil {
		c.log.Error("failed to write error envelope", zap.Error(err))
		return
	}
	body, _ := json.Marshal(map[string]string{"error": msg})
	if _, err := stream.Write(body); err != nil {
		c.log.Error("failed to write error body", zap.Error(err))
	}
}

const maxEnvelopeSize = 64 * 1024

func readEnvelope(stream transport.Stream, envelope interface{}) error {
	// Read 4-byte big endian length prefix
	lengthPrefix := make([]byte, 4)
	if _, err := io.ReadFull(stream, lengthPrefix); err != nil {
		return fmt.Errorf("Read Envelope Length Prefix: %w", err)
	}
	length := binary.BigEndian.Uint32(lengthPrefix)
	if length > maxEnvelopeSize {
		return fmt.Errorf("Envelope too large: %d bytes (max: %d)", length, maxEnvelopeSize)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(stream, data); err != nil {
		return fmt.Errorf("Read Envelope Data: %w", err)
	}
	if err := json.Unmarshal(data, envelope); err != nil {
		return fmt.Errorf("Unmarshal Envelope: %w", err)
	}
	return nil
}

func writeEnvelope(stream transport.Stream, envelope interface{}) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("Marshal Envelope: %w", err)
	}
	if len(data) > maxEnvelopeSize {
		return fmt.Errorf("Envelope too large: %d bytes (max: %d)", len(data), maxEnvelopeSize)
	}
	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, uint32(len(data)))
	if _, err := stream.Write(lengthPrefix); err != nil {
		return fmt.Errorf("Write Envelope Length Prefix: %w", err)
	}
	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("Write Envelope Data: %w", err)
	}
	return nil
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
