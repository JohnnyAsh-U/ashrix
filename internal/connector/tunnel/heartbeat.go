package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

const (
	heartbeatInterval = 5 * time.Second
	heartbeatTimeout  = 15 * time.Second
)

func (c *TunnelClient) runHeartbeats(ctx context.Context, stream *quic.Stream) error {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	ackCh := make(chan struct{}, 1)
	readErr := make(chan error, 1)
	go func() {
		decoder := json.NewDecoder(stream)
		for {
			var msg proto.ControlMessage
			if err := decoder.Decode(&msg); err != nil {
				readErr <- fmt.Errorf("read heartbeat acknowledgement: %w", err)
			}
			if msg.MsgType == proto.MsgType_HEARTBEAT_ACK {
				select {
				case ackCh <- struct{}{}:
				default:
				}
				c.log.Debug("heartbeat ack received")
			}
		}
	}()

	ackTimer := time.NewTimer(heartbeatTimeout)
	defer ackTimer.Stop()
	encoder := json.NewEncoder(stream)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readErr:
			return err
		case <-ackTimer.C:
			return fmt.Errorf("gateway heartbeat acknowledgement timed out after %s", heartbeatTimeout)
		case <-ackCh:
			if !ackTimer.Stop() {
				select {
				case <-ackTimer.C:
				default:
				}
			}
			ackTimer.Reset(heartbeatTimeout)
		case <-ticker.C:
			hb := proto.ControlMessage{MsgType: proto.MsgType_HEARTBEAT}
			if err := encoder.Encode(&hb); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
 			}
			c.log.Debug("heartbeat sent")
		}
	}
}
