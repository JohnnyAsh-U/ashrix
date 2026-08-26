package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	// QUIC — primary
	GatewayQUICAddr string // "gateway.ashrix.io:9445"

	// gRPC/HTTP2 — fallback 1
	GatewayGRPCAddr string // "gateway.ashrix.io:443"

	// WebSocket — fallback 2 (DPI environments)
	GatewayWSURL string // "wss://gateway.ashrix.io/tunnel"

	// Auth
	ConnectorID string

	// TLS — nil in dev (insecure), set in prod
	TLSConfig *tls.Config
}

// Negotiate tries transports in order: QUIC → gRPC → WebSocket.
// Returns the first one that works.
// The rest of the connector never knows which transport won.
func Negotiate(
	ctx context.Context,
	cfg Config,
	log *zap.Logger,
) (Session, error) {

	// ── 1. QUIC — Ferrari ─────────────────────────────────────────
	// Best performance. Connection migration. No HOL blocking.
	// Blocked by: firewalls filtering UDP.
	{
		quicCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		tlsCfg := cfg.TLSConfig
		if tlsCfg == nil {
			tlsCfg = &tls.Config{
				InsecureSkipVerify: false, // dev only
				NextProtos:         []string{"ashrix-tunnel"},
			}
		}

		session, err := DialQUIC(quicCtx, cfg.GatewayQUICAddr, tlsCfg, log)
		if err == nil {
			log.Info("transport negotiated: QUIC",
				zap.String("addr", cfg.GatewayQUICAddr),
			)
			return session, nil
		}
		log.Warn("QUIC unavailable — trying gRPC",
			zap.Error(err),
		)
	}

	// ── 3. WebSocket — Bike with a motor ──────────────────────────
	// Last resort. TCP 443 WebSocket survives most DPI.
	// Blocked by: almost nothing.
	// {
	// 	wsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	// 	defer cancel()

	// 	tlsCfg := cfg.TLSConfig
	// 	if tlsCfg == nil {
	// 		tlsCfg = &tls.Config{
	// 			InsecureSkipVerify: true, // dev only
	// 		}
	// 	}

	// 	session, err := DialWebSocket(
	// 		wsCtx,
	// 		cfg.GatewayWSURL,
	// 		cfg.ConnectorID,
	// 		"kf",
	// 		tlsCfg,
	// 		log,
	// 	)
	// 	if err == nil {
	// 		log.Info("transport negotiated: WebSocket",
	// 			zap.String("url", cfg.GatewayWSURL),
	// 			zap.String("note", "gRPC blocked — possible DPI"),
	// 		)
	// 		return session, nil
	// 	}
	// 	log.Warn("WebSocket unavailable", zap.Error(err))
	// }

	return nil, fmt.Errorf(
		"all transports failed — check network connectivity\n"+
			"  QUIC:      %s\n"+
			"  gRPC:      %s\n"+
			"  WebSocket: %s",
		cfg.GatewayQUICAddr,
		cfg.GatewayGRPCAddr,
		cfg.GatewayWSURL,
	)
}
