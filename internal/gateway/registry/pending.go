


// internal/gateway/registry/pending.go
package registry

import (
	"sync"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// PendingCommands holds commands intended for connectors that
// are CURRENTLY offline.
//
// Per our reconciliation design: this is NOT a durable delivery
// queue. It is a best-effort convenience so that if a connector
// reconnects within the same gateway process lifetime, it gets
// the command immediately rather than waiting for the next
// heartbeat-driven reconciliation cycle.
//
// If the gateway restarts, this is wiped — and that is fine,
// because reconciliation on reconnect (asking CP for current
// state) is the actual source of correctness, not this queue.
type PendingCommands struct {
	mu       sync.Mutex
	commands map[string][]*pb.GatewayEnvelope // connector_id → queued envelopes
}

const maxPendingPerConnector = 10

func NewPendingCommands() *PendingCommands {
	return &PendingCommands{
		commands: make(map[string][]*pb.GatewayEnvelope),
	}
}

// Enqueue adds a command for an offline connector.
// Bounded — drops oldest if queue is full, since only the
// LATEST state matters (per reconciliation model), not history.
func (p *PendingCommands) Enqueue(connectorID string, env *pb.GatewayEnvelope) {
	p.mu.Lock()
	defer p.mu.Unlock()

	queue := p.commands[connectorID]
	if len(queue) >= maxPendingPerConnector {
		queue = queue[1:]
	}
	p.commands[connectorID] = append(queue, env)
}

// Drain returns and clears all pending commands for a connector.
// Called immediately when a connector's management stream is
// established, BEFORE the accept loop begins — ensures queued
// commands are delivered before any new traffic is processed.
func (p *PendingCommands) Drain(connectorID string) []*pb.GatewayEnvelope {
	p.mu.Lock()
	defer p.mu.Unlock()

	cmds := p.commands[connectorID]
	delete(p.commands, connectorID)
	return cmds
}



