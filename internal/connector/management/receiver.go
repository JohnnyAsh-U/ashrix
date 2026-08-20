package management

import (
	"context"
	"fmt"
	"syscall"
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)

//The management receiver
func (c *ManagementConn) RunReceiver(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in RunReceiver", zap.Any("panic", r))
			err = fmt.Errorf("RunReceiver panic: %v", r)
		}
	}()
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
			c.log.Info("received command",
				zap.String("cmd", p.Cmd.Cmd),
				zap.String("payload", p.Cmd.Payload),
			)
			switch p.Cmd.Cmd {
			case "ROTATE_CONNECTOR_CERT":
				go func() {
					c.log.Warn("starting connector cert rotation")
					if err := c.PKI.Renew(ctx, true); err != nil {
						c.log.Error("connector cert rotation failed", zap.Error(err))
					} else {
						c.log.Info("connector cert rotation succeeded, exiting to reconnect with new cert")
						_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
					}
				}()
			case "REVOKE_CONNECTOR_CERT", "REVOKE_CONNECTOR":
				go func() {
					c.log.Error("connector certificate or component revoked - clearing credentials and shutting down")
					_ = c.AppStorage.ClearCredential()
					time.Sleep(500 * time.Millisecond)
					_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
				}()
			}
		default:
			c.log.Warn("received unknown message type",
				zap.String("type", fmt.Sprintf("%T", p)),
			)

		}
	}
	
}