package management

import (
	"context"
	"fmt"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)

//The management receiver
func (c *ManagementConn) RunReceiver(ctx context.Context) error {
	// Receive loop — this is the main goroutine
	return c.receiver(ctx)
}

// receiver receives messages from the management stream and handles them.
func (c *ManagementConn) receiver(ctx context.Context) error {
	for {
		msg, err := c.Stream.Recv()
		if err != nil {
			return fmt.Errorf("management stream recv: %w", err)
		}

		switch p := msg.Payload.(type) {
		case *proto.GatewayConnectorEnvelope_Cmd:
			c.log.Debug("received command",
				zap.String("cmd", p.Cmd.Cmd),
				zap.String("payload", p.Cmd.Payload),
			)
		default:
			c.log.Warn("received unknown message type",
				zap.String("type", fmt.Sprintf("%T", p)),
			)

		}
	}
	
}