package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"
)

type Config struct {
	// QUIC — primary
	GatewayQUICAddr string // "gateway.ashrix.io:9445"

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
	log *slog.Logger,
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
				"addr", cfg.GatewayQUICAddr,
			)
			return session, nil
		}
		log.Warn("QUIC unavailable — trying gRPC",
			"error", err,
		)
	}

	
	return nil, fmt.Errorf(
		"all transports failed — check network connectivity\n"+
			"  QUIC:      %s\n",
		cfg.GatewayQUICAddr,
	)
}
