package quic

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/quic-go/quic-go"
	"go.uber.org/zap"
)

type QUICServer struct {
	listener *quic.Listener
	addr     string
	tlsConf  *tls.Config
	log      *zap.Logger
}

func NewQUICServer(cfg *config.Config, tlsConfig *tls.Config, log *zap.Logger) *QUICServer {
	// Enforce mTLS by requiring client certificates
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	return &QUICServer{
		addr:    fmt.Sprintf(":%s", cfg.QUICPort),
		tlsConf: tlsConfig,
		log:     log,
	}
}

func (s *QUICServer) Start(ctx context.Context) error {
	listener, err := quic.ListenAddr(s.addr, s.tlsConf, nil)
	if err != nil {
		return fmt.Errorf("failed to listen on QUIC port %s: %w", s.addr, err)
	}
	s.listener = listener

	s.log.Info("Gateway QUIC Server starting", zap.String("addr", s.addr))

	for {
		conn, err := s.listener.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Clean shutdown via context
			}
			s.log.Error("QUIC accept error", zap.Error(err))
			// Usually accept errors in QUIC are temporary or relate to a specific connection.
			continue
		}

		s.log.Debug("QUIC connection accepted", zap.String("remote_addr", conn.RemoteAddr().String()))

		// TODO: Spawn a goroutine to handle the connection streams
		// go handleQUICConnection(ctx, conn, s.log)
		_ = conn // suppress unused variable error for now
	}
}

func (s *QUICServer) Stop() {
	s.log.Info("Gateway QUIC Server stopping")
	if s.listener != nil {
		s.listener.Close()
	}
}
