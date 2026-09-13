package registry

import (
	"context"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"sync"
	"time"
)

// GatewayConn represents an active gRPC connection to a gateway.
type GatewayConn struct {
	GatewayID string
	TenantID  string // For self-hosted: the single tenant. For hosted: "" (looked up per-request)
	// GatewayType GatewayType
	Stream      pb.ControlPlaneService_ConnectServer // The bidirectional gRPC stream
	ConnectedAt time.Time
	LastSeen    time.Time

	// Current state known from the gateway
	CurrentPolicyVersion uint64
	// CurrentTrustVersion  uint64

	// Control
	Ctx    context.Context
	Cancel context.CancelFunc

	sendMu sync.Mutex

	ackMu sync.Mutex

	ackWaiters map[int64]chan error
}

func NewGatewayConn(
	ctx context.Context,
	cancel context.CancelFunc,
	stream pb.ControlPlaneService_ConnectServer,
	gatewayID string,
	tenantID string,
	// policyVersion uint64,
) *GatewayConn {

	return &GatewayConn{
		GatewayID: gatewayID,
		TenantID:  tenantID,
		Stream:    stream,

		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),

		CurrentPolicyVersion: 0,

		Ctx:    ctx,
		Cancel: cancel,

		ackWaiters: make(map[int64]chan error),
	}
}

// Send sends a CPMessage to this gateway over its active stream.
func (c *GatewayConn) Send(
	envelope *pb.CPEnvelope,
) error {

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	return c.Stream.Send(envelope)
}

func (c *GatewayConn) RegisterAck(
	seq int64,
) <-chan error {

	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	ch := make(chan error, 1)

	c.ackWaiters[seq] = ch

	return ch
}

func (c *GatewayConn) ResolveAck(
	seq int64,
	err error,
) {

	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	ch, exists :=
		c.ackWaiters[seq]

	if !exists {
		return
	}

	delete(
		c.ackWaiters,
		seq,
	)

	ch <- err
	close(ch)
}

func (c *GatewayConn) ResolveThrough(
	seq int64,
	err error,
) {

	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	for waitingSeq, ch := range c.ackWaiters {

		if waitingSeq > seq {
			continue
		}

		delete(
			c.ackWaiters,
			waitingSeq,
		)

		ch <- err
		close(ch)
	}
}
