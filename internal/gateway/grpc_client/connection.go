package gateway_grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type ConnectionManager struct {
	target    string
	tlsConfig *tls.Config
	dialOpts  []grpc.DialOption

	conn  atomic.Pointer[grpc.ClientConn]
	ready atomic.Bool
	mu    sync.Mutex
}

func NewConnectionManager(target string, tlsConfig *tls.Config, extraOpts ...grpc.DialOption) *ConnectionManager {
	return &ConnectionManager{
		target:    target,
		tlsConfig: tlsConfig,
		dialOpts:  extraOpts,
	}
}

func (cm *ConnectionManager) CurrentConn() *grpc.ClientConn {
	return cm.conn.Load()
}

func (cm *ConnectionManager) Ready() bool {
	return cm.ready.Load()
}

func (cm *ConnectionManager) RefreshConnection(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	creds := credentials.NewTLS(cm.tlsConfig)

	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}, cm.dialOpts...)


	// grpc.NewClient is non-blocking. The connection starts in Idle.
	newConn, err := grpc.NewClient(cm.target, opts...)
	if err != nil {
		return err
	}

	// ── FIX: Trigger the connection attempt ───────────────────────
	// Without this, the state machine stays in Idle forever and
	// WaitForStateChange(ctx, Idle) never returns.
	newConn.Connect()

	// Now block until Ready or context cancellation.
	if err := cm.waitForReady(ctx, newConn); err != nil {
		newConn.Close()
		return err
	}

	fmt.Println("Initializing gRPC Client Connection to CP...")


	oldConn := cm.conn.Swap(newConn)
	cm.ready.Store(true)

	if oldConn != nil {
		go func(c *grpc.ClientConn) {
			time.Sleep(5 * time.Second)
			c.Close()
		}(oldConn)
	}
	return nil
}

// waitForReady blocks until the connection reaches Ready, using the
// canonical gRPC state-machine polling pattern.
func (cm *ConnectionManager) waitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	for state := conn.GetState(); state != connectivity.Ready; state = conn.GetState() {
		if !conn.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
	return nil
}

func (cm *ConnectionManager) Close() error {
	cm.ready.Store(false)
	if c := cm.conn.Swap(nil); c != nil {
		return c.Close()
	}
	return nil
}

func (cm *ConnectionManager) HealthCheckLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c := cm.CurrentConn()
			if c == nil {
				continue
			}
			s := c.GetState()
			fmt.Println(s)
			if s == connectivity.TransientFailure || s == connectivity.Shutdown {
				_ = cm.RefreshConnection(ctx)
			}
		}
	}
}