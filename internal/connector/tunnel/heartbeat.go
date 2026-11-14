package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

func (c *TunnelClient) runHeartbeats(ctx context.Context, stream *quic.Stream) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Reader for incoming management messages (Heartbeat Acks)
	go func() {
		decoder := json.NewDecoder(stream)
		for {
			var msg proto.ControlMessage
			if err := decoder.Decode(&msg); err != nil {
				return
			}
			if msg.MsgType == proto.MsgType_HEARTBEAT_ACK {
				c.log.Debug("heartbeat ack received")
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Context Canceled")
		case <-ticker.C:
			hb := proto.ControlMessage{MsgType: proto.MsgType_HEARTBEAT}
			if err := json.NewEncoder(stream).Encode(&hb); err != nil {
				c.log.Error("failed to send heartbeat", "err", err)
				return fmt.Errorf("Context Canceled")
			}
			c.log.Debug("heartbeat sent")
		}
	}
}