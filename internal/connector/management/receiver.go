package management

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)

var ErrCertRotated = errors.New("certificate rotated, triggering session reconnect")

// The management receiver
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
	// Create a channel to catch rotation signal from background worker
	rotateCh := make(chan error, 1)

	for {
		// Non-blocking check to see if a rotation signaled an exit
		select {
		case err := <-rotateCh:
			return err
		default:
		}

		msg, err := c.Stream.Recv()
		if err != nil {
			return fmt.Errorf("management stream recv: %w", err)
		}

		switch p := msg.Payload.(type) {
		case *proto.GatewayConnectorEnvelope_Cmd:
			c.log.Info("received command",
				zap.String("cmd", p.Cmd.Cmd.String()),
				zap.String("payload", p.Cmd.Payload),
			)
			switch p.Cmd.Cmd {
			case pb.CommandType_CMD_ROTATE_CONNECTOR_CERT:
				go c.handleCertRotation(ctx, rotateCh)
			case pb.CommandType_CMD_REVOKE_CONNECTOR_CERT, pb.CommandType_CMD_REVOKE_CONNECTOR:
				go func() {
					c.log.Error("connector certificate or component revoked - clearing credentials and shutting down")
					_ = c.AppStorage.ClearCredential()
					time.Sleep(500 * time.Millisecond)
					_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
				}()
			case pb.CommandType_CMD_RELOAD_CONNECTOR:
				return fmt.Errorf("Received Reload Command")
			}
		default:
			c.log.Warn("received unknown message type",
				zap.String("type", fmt.Sprintf("%T", p)),
			)

		}
	}

}

func (c *ManagementConn) handleCertRotation(ctx context.Context, rotateCh chan<- error) {
	c.rotatingMu.Lock()
	if c.isRotating {
		c.rotatingMu.Unlock()
		c.log.Warn("connector cert rotation already in progress, skipping duplicate command")
		return
	}
	c.isRotating = true
	c.rotatingMu.Unlock()

	defer func() {
		c.rotatingMu.Lock()
		c.isRotating = false
		c.rotatingMu.Unlock()
	}()

	c.log.Warn("starting connector cert rotation")

	// Perform renewal & hot-swap in memory
	if err := c.PKI.Renew(ctx, true); err != nil {
		c.log.Error("connector cert rotation failed", zap.Error(err))
		return
	}

	c.log.Info("connector cert rotation succeeded — triggering soft connection reset")

	// Signal receiver to exit gracefully with ErrCertRotated
	select {
	case rotateCh <- ErrCertRotated:
	default:
	}
}
