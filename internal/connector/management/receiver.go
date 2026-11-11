package management

import (
	"context"
	"fmt"
)

// The management receiver
func (c *ManagementConn) RunReceiver(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in RunReceiver", "panic", r)
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

		c.log.Warn("ignoring unsupported gateway management message", "type", fmt.Sprintf("%T", msg.Payload))
	}

}
