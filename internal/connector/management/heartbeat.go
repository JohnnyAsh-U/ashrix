package management

import (
	"context"
	"fmt"
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// runHeartbeat sends heartbeats every 30s on the management stream.
func (c *ManagementConn) RunHeartbeat(ctx context.Context, seconds time.Duration) (err error) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in RunHeartbeat", "panic", r)
			err = fmt.Errorf("RunHeartbeat panic: %v", r)
		}
	}()

	ticker := time.NewTicker(seconds)
	defer ticker.Stop()

	var seq int64

	for {
		select {
		case <-ticker.C:
			seq++
			err := c.Stream.Send(&proto.ConnectorGatewayEnvelope{
				ConnectorId: c.ConnectorID,
				SentAt:      timestamppb.Now(),
				Payload: &proto.ConnectorGatewayEnvelope_Heartbeat{
					Heartbeat: &proto.ConnectorHeartbeat{
						Seq:             seq,
						TunnelState:     "connected",
						TunnelTransport: "grpc",
						UptimeSeconds:   c.uptimeSeconds(),
						ActiveStreams: c.ActiveStreams.Load(),
					},
				},
			})
			if err != nil {
				return fmt.Errorf("heartbeat send: %w", err)
			}

			c.log.Debug("heartbeat sent",
				"seq", seq,
			)

		case <-ctx.Done():
			return nil
		}
	}
}

func (c *ManagementConn) uptimeSeconds() int64 {
	return int64(time.Since(c.StartedAt).Seconds())
}