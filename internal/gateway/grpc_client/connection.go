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
	// "google.golang.org/grpc/keepalive"
)

// ConnectionManager owns exactly one *grpc.ClientConn at a time.
// It is hot-swapped atomically so HTTP handlers never block.
type ConnectionManager struct {
	target    string
	tlsConfig *tls.Config
	dialOpts  []grpc.DialOption

	conn  atomic.Pointer[grpc.ClientConn]
	ready atomic.Bool
	mu    sync.Mutex // serializes RefreshConnection to prevent concurrent dials
}

func NewConnectionManager(target string, tlsConfig *tls.Config, extraOpts ...grpc.DialOption) *ConnectionManager {
	return &ConnectionManager{
		target:    target,
		tlsConfig: tlsConfig,
		dialOpts:  extraOpts,
	}
}

// CurrentConn returns the active connection (may be nil).
func (cm *ConnectionManager) CurrentConn() *grpc.ClientConn {
	return cm.conn.Load()
}

// Ready reports whether a healthy connection has been established.
func (cm *ConnectionManager) Ready() bool {
	return cm.ready.Load()
}

// RefreshConnection dials a new conn, waits for it to become ready,
// atomically swaps it, then closes the old conn lazily in the background.
func (cm *ConnectionManager) RefreshConnection(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	fmt.Println("Here")


	creds := credentials.NewTLS(cm.tlsConfig)

	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		// grpc.WithKeepaliveParams(keepalive.ClientParameters{
		// 	Time:                10 * time.Second,
		// 	Timeout:             3 * time.Second,
		// 	PermitWithoutStream: true,
		// }),
	}, cm.dialOpts...)

	// grpc.NewClient is non-blocking (v1.63+).
	newConn, err := grpc.NewClient(cm.target, opts...)
	if err != nil {
		return err
	}



	if err := cm.waitForReady(ctx, newConn); err != nil {
		newConn.Close()
		return err
	}

	oldConn := cm.conn.Swap(newConn)
	cm.ready.Store(true)

	fmt.Println("Here")


	if oldConn != nil {
		// Give in-flight unary RPCs a grace period before force-closing.
		go func(c *grpc.ClientConn) {
			time.Sleep(5 * time.Second)
			c.Close()
		}(oldConn)
	}
	return nil
}

func (cm *ConnectionManager) waitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	for state := conn.GetState(); state != connectivity.Ready; state = conn.GetState() {
		if !conn.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
	return nil
}

// Close tears down the active connection.
func (cm *ConnectionManager) Close() error {
	cm.ready.Store(false)
	if c := cm.conn.Swap(nil); c != nil {
		return c.Close()
	}
	return nil
}

// HealthCheckLoop runs in the background. If the connection sits in
// TransientFailure too long, it forces a refresh.
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
			if s == connectivity.TransientFailure || s == connectivity.Shutdown {
				_ = cm.RefreshConnection(ctx)
			}
		}
	}
}