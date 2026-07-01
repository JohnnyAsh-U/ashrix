package tunnel

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	// "go.uber.org/zap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"go.uber.org/zap"
)

// ConnectorTunnel manages the full lifecycle of the tunnel.
type ConnectorTunnel struct {
	log       *zap.Logger
	startedAt time.Time

	// Active stream count — reported in heartbeat
	activeStreams atomic.Int64
}

func NewTunnel(log *zap.Logger) *ConnectorTunnel {
	return &ConnectorTunnel{
		log:       log,
		startedAt: time.Now(),
	}
}



// acceptLoop blocks waiting for gateway to open request streams.
// Each stream = one user HTTP request to proxy to localhost:3000.
func (c *ConnectorTunnel) AcceptLoop(
	ctx context.Context,
	session transport.Session,
) error {
	c.log.Debug("accept loop started — waiting for requests")

	for {
		// Check context and session health before blocking
		select {
		case <-ctx.Done():
			return nil
		case <-session.Done():
			return fmt.Errorf("session closed by gateway")
		default:
		}

		// Block until gateway sends a request stream
		stream, err := session.AcceptStream(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept stream: %w", err)
			}
		}

		// Handle in goroutine — accept loop never blocks
		// 100 concurrent users = 100 goroutines = fine
		go c.handleRequestStream(stream)
	}
}
