Yes. Given your actual `Connect()` implementation, I would now make the design **durable-first + connect-time reconciliation + active delivery**, while keeping the gateway stateless regarding the event cursor.

There is one important correction to the previous design: **do not have `Connect()` and the dispatcher independently call `stream.Send()`**. Your policy distributor and event reconciler can otherwise race on the same gRPC stream. Put all outbound messages through the `GatewayConn.Send()` method protected by a mutex.

Below is the implementation I would use.

---

# 1. Database

## `gateway_events`

```sql
CREATE TABLE gateway_events (
    gateway_id UUID NOT NULL
        REFERENCES gateways(id)
        ON DELETE CASCADE,

    seq BIGINT NOT NULL,

    event_id UUID NOT NULL DEFAULT gen_random_uuid(),

    command TEXT NOT NULL,

    delivery_mode TEXT NOT NULL
        CHECK (delivery_mode IN ('ACTION', 'SNAPSHOT')),

    payload JSONB NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (gateway_id, seq),

    UNIQUE (event_id)
);

CREATE INDEX idx_gateway_events_gateway_seq
    ON gateway_events(gateway_id, seq);
```

## `gateway_events_acks`

```sql
CREATE TABLE gateway_events_acks (
    gateway_id UUID PRIMARY KEY
        REFERENCES gateways(id)
        ON DELETE CASCADE,

    last_acked_seq BIGINT NOT NULL DEFAULT 0,

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Sequence allocator

I strongly recommend separating sequence allocation from `MAX(seq)`.

```sql
CREATE TABLE gateway_event_sequences (
    gateway_id UUID PRIMARY KEY
        REFERENCES gateways(id)
        ON DELETE CASCADE,

    next_seq BIGINT NOT NULL DEFAULT 1
);
```

When a gateway is provisioned:

```sql
INSERT INTO gateway_event_sequences(gateway_id)
VALUES ($1)
ON CONFLICT DO NOTHING;

INSERT INTO gateway_events_acks(gateway_id)
VALUES ($1)
ON CONFLICT DO NOTHING;
```

---

# 2. SQLC queries

### `gateway_events.sql`

```sql
-- name: AllocateGatewayEventSeq :one
INSERT INTO gateway_event_sequences (
    gateway_id,
    next_seq
)
VALUES ($1, 2)
ON CONFLICT (gateway_id)
DO UPDATE
SET next_seq = gateway_event_sequences.next_seq + 1
RETURNING next_seq - 1 AS seq;
```

```sql
-- name: InsertGatewayEvent :exec
INSERT INTO gateway_events (
    gateway_id,
    seq,
    event_id,
    command,
    delivery_mode,
    payload
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
);
```

```sql
-- name: ListGatewayEventsAfter :many
SELECT
    gateway_id,
    seq,
    event_id,
    command,
    delivery_mode,
    payload,
    created_at
FROM gateway_events
WHERE gateway_id = $1
  AND seq > $2
ORDER BY seq ASC;
```

```sql
-- name: GetLastAckedSeqForGateway :one
SELECT last_acked_seq
FROM gateway_events_acks
WHERE gateway_id = $1;
```

```sql
-- name: AckGatewayEvents :exec
UPDATE gateway_events_acks
SET
    last_acked_seq = GREATEST(
        last_acked_seq,
        $2
    ),
    updated_at = NOW()
WHERE gateway_id = $1;
```

```sql
-- name: GetLatestGatewayEventSeq :one
SELECT COALESCE(MAX(seq), 0)::BIGINT
FROM gateway_events
WHERE gateway_id = $1;
```

Cleanup:

```sql
-- name: DeleteAckedGatewayEvents :exec
DELETE FROM gateway_events ge
USING gateway_events_acks ack
WHERE ge.gateway_id = ack.gateway_id
  AND ge.seq <= ack.last_acked_seq
  AND ge.created_at < NOW() - INTERVAL '30 days';
```

---

# 3. Protobuf

I would change the protocol to make the sequence explicit.

```protobuf
message CPEnvelope {
    int64 seq = 1;
    string event_id = 2;

    google.protobuf.Timestamp sent_at = 3;

    oneof payload {
        Command cmd = 10;
        HelloAck hello_ack = 11;
    }
}
```

Gateway → CP:

```protobuf
message GatewayEnvelope {
    oneof payload {
        Hello hello = 1;
        Heartbeat heartbeat = 2;
        CommandAck cmd_ack = 3;
    }
}
```

ACK:

```protobuf
message CommandAck {
    string gateway_id = 1;

    int64 processed_through_seq = 2;
}
```

Your Hello remains approximately:

```protobuf
message Hello {
    string gateway_id = 1;
    string tenant_id = 2;
    int64 policy_version = 3;
    string binary_version = 4;
}
```

**Do not put `last_acked_seq` in Hello.**

The CP owns that state.

---

# 4. Dispatcher command definitions

```go
package dispatcher

import "github.com/JohnnyAsh-U/ashrix-api/proto/gen"

type CommandType string

const (
	CmdRotateGatewayCert CommandType = "ROTATE_GATEWAY_CERT"
	CmdRevokeGatewayCert CommandType = "REVOKE_GATEWAY_CERT"
	CmdRevokeGateway     CommandType = "REVOKE_GATEWAY"
	CmdDrainGateway      CommandType = "DRAIN_GATEWAY"

	CmdRotateConnectorCert CommandType = "ROTATE_CONNECTOR_CERT"
	CmdRevokeConnectorCert CommandType = "REVOKE_CONNECTOR_CERT"
	CmdRevokeConnector     CommandType = "REVOKE_CONNECTOR"

	CmdRevokeUserSession CommandType = "REVOKE_USER_SESSION"

	CmdCrlSync       CommandType = "CRL_SYNC"
	CmdConnectorSync CommandType = "CONNECTOR_SYNC"
)

type DeliveryMode string

const (
	DeliveryAction   DeliveryMode = "ACTION"
	DeliverySnapshot DeliveryMode = "SNAPSHOT"
)

func DeliveryModeForCommand(cmd CommandType) DeliveryMode {
	switch cmd {
	case CmdCrlSync, CmdConnectorSync:
		return DeliverySnapshot
	default:
		return DeliveryAction
	}
}

type CommandJob struct {
	Type CommandType

	GatewayID string

	ConnectorID string

	SessionID string

	RevokedSerialNumbers []string

	ConnectorInfo []*gen.ConnectorInfo
}
```

---

# 5. Persisted command payload

Create:

```text
internal/cp/platform/cp_grpc/dispatcher/event_payload.go
```

```go
package dispatcher

import (
	"encoding/json"
	"fmt"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

type persistedCommand struct {
	Type CommandType `json:"type"`

	GatewayID string `json:"gateway_id"`

	ConnectorID string `json:"connector_id,omitempty"`

	SessionID string `json:"session_id,omitempty"`

	RevokedSerialNumbers []string `json:"revoked_serial_numbers,omitempty"`

	ConnectorInfo []*gen.ConnectorInfo `json:"connector_info,omitempty"`
}

func MarshalCommand(job CommandJob) ([]byte, error) {
	return json.Marshal(
		persistedCommand{
			Type:                  job.Type,
			GatewayID:             job.GatewayID,
			ConnectorID:           job.ConnectorID,
			SessionID:             job.SessionID,
			RevokedSerialNumbers:  job.RevokedSerialNumbers,
			ConnectorInfo:         job.ConnectorInfo,
		},
	)
}

func UnmarshalCommand(data []byte) (CommandJob, error) {
	var command persistedCommand

	if err := json.Unmarshal(data, &command); err != nil {
		return CommandJob{}, fmt.Errorf(
			"unmarshal command payload: %w",
			err,
		)
	}

	return CommandJob{
		Type:                  command.Type,
		GatewayID:             command.GatewayID,
		ConnectorID:           command.ConnectorID,
		SessionID:             command.SessionID,
		RevokedSerialNumbers:  command.RevokedSerialNumbers,
		ConnectorInfo:         command.ConnectorInfo,
	}, nil
}
```

---

# 6. Gateway event model

```go
package dispatcher

import "time"

type GatewayEvent struct {
	GatewayID string

	Seq int64

	EventID string

	Command CommandType

	DeliveryMode DeliveryMode

	Payload []byte

	CreatedAt time.Time
}
```

---

# 7. Convert persisted event → protobuf command

```go
package dispatcher

import (
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

func GatewayEventToCmd(
	event store.GatewayEvent,
) (*gen.Command, error) {

	job := CommandJob{
		Type:         CommandType(event.Command),
		GatewayID:    event.GatewayID.String(),
	}

	if len(event.Payload) > 0 {
		var err error

		job, err = UnmarshalCommand(
			event.Payload,
		)

		if err != nil {
			return nil, err
		}
	}

	return BuildCommand(job)
}

func BuildCommand(
	job CommandJob,
) (*gen.Command, error) {

	switch job.Type {

	case CmdRotateGatewayCert:
		return &gen.Command{
			Payload: &gen.Command_RotateGatewayCert{
				RotateGatewayCert: &gen.RotateGatewayCertCmd{},
			},
		}, nil

	case CmdRevokeGatewayCert:
		return &gen.Command{
			Payload: &gen.Command_RevokeGatewayCert{
				RevokeGatewayCert: &gen.RevokeGatewayCertCmd{},
			},
		}, nil

	case CmdRevokeGateway:
		return &gen.Command{
			Payload: &gen.Command_RevokeGateway{
				RevokeGateway: &gen.RevokeGatewayCmd{},
			},
		}, nil

	case CmdDrainGateway:
		return &gen.Command{
			Payload: &gen.Command_DrainGateway{
				DrainGateway: &gen.DrainGatewayCmd{},
			},
		}, nil

	case CmdRotateConnectorCert:
		return &gen.Command{
			Payload: &gen.Command_RotateConnectorCert{
				RotateConnectorCert: &gen.RotateConnectorCertCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		}, nil

	case CmdRevokeConnectorCert:
		return &gen.Command{
			Payload: &gen.Command_RevokeConnectorCert{
				RevokeConnectorCert: &gen.RevokeConnectorCertCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		}, nil

	case CmdRevokeConnector:
		return &gen.Command{
			Payload: &gen.Command_RevokeConnector{
				RevokeConnector: &gen.RevokeConnectorCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		}, nil

	case CmdRevokeUserSession:
		return &gen.Command{
			Payload: &gen.Command_RevokeSession{
				RevokeSession: &gen.RevokeSessionCmd{
					SessionId: job.SessionID,
				},
			},
		}, nil

	case CmdCrlSync:
		return &gen.Command{
			Payload: &gen.Command_CrlSync{
				CrlSync: &gen.CrlSyncCmd{
					RevokedSerialNumbers:
						job.RevokedSerialNumbers,
				},
			},
		}, nil

	case CmdConnectorSync:
		return &gen.Command{
			Payload: &gen.Command_ConnectorSync{
				ConnectorSync: &gen.ConnectorSyncCmd{
					Connectors: job.ConnectorInfo,
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf(
			"unsupported command type: %s",
			job.Type,
		)
	}
}
```

Adapt the generated sqlc `GatewayEvent` field names if yours differ.

---

# 8. Event creation service

This replaces the current "put job in bounded queue and hope it gets delivered" approach.

```go
package dispatcher

import (
	"context"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type EventService struct {
	queries *store.Queries
}

func NewEventService(
	queries *store.Queries,
) *EventService {
	return &EventService{
		queries: queries,
	}
}

func (s *EventService) CreateEvent(
	ctx context.Context,
	job CommandJob,
) (GatewayEvent, error) {

	gatewayID, err := uuid.Parse(job.GatewayID)
	if err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"invalid gateway ID: %w",
			err,
		)
	}

	payload, err := MarshalCommand(job)
	if err != nil {
		return GatewayEvent{}, err
	}

	tx, err := s.queries.BeginTx(ctx)
	if err != nil {
		return GatewayEvent{}, err
	}

	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)

	seq, err := q.AllocateGatewayEventSeq(
		ctx,
		gatewayID,
	)
	if err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"allocate gateway event sequence: %w",
			err,
		)
	}

	eventID := uuid.New()

	mode := DeliveryModeForCommand(job.Type)

	err = q.InsertGatewayEvent(
		ctx,
		store.InsertGatewayEventParams{
			GatewayID: gatewayID,

			Seq: seq,

			EventID: pgtype.UUID{
				Bytes: eventID,
				Valid: true,
			},

			Command: string(job.Type),

			DeliveryMode: string(mode),

			Payload: payload,
		},
	)

	if err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"insert gateway event: %w",
			err,
		)
	}

	err = q.EnsureGatewayEventsAck(
		ctx,
		gatewayID,
	)

	if err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"ensure gateway ACK: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"commit gateway event: %w",
			err,
		)
	}

	return GatewayEvent{
		GatewayID:    job.GatewayID,
		Seq:          seq,
		EventID:      eventID.String(),
		Command:      job.Type,
		DeliveryMode: mode,
		Payload:      payload,
	}, nil
}
```

---

# 9. Important: create the event in the same transaction as the state mutation

This is one of the most important pieces.

For example, revoking a certificate should **not** be:

```text
UPDATE certificate

then

INSERT gateway_event
```

in two unrelated transactions.

Instead:

```text
BEGIN

UPDATE certificate
    revoked_at = NOW()

INSERT CRL entry

allocate gateway seq

INSERT gateway_events

COMMIT
```

Otherwise you can get:

```text
certificate revoked
       ↓
process crashes
       ↓
no gateway event
       ↓
gateway never learns
```

For Ashrix, that is unacceptable.

Your repository methods that change security state should therefore own the transaction.

---

# 10. Snapshot compaction

```go
package dispatcher

import "github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"

func CompactGatewayEvents(
	events []store.GatewayEvent,
) []store.GatewayEvent {

	if len(events) <= 1 {
		return events
	}

	latest := make(map[string]int)

	for i, event := range events {

		if event.DeliveryMode !=
			string(DeliverySnapshot) {

			continue
		}

		latest[event.Command] = i
	}

	result := make(
		[]store.GatewayEvent,
		0,
		len(events),
	)

	for i, event := range events {

		if event.DeliveryMode ==
			string(DeliverySnapshot) {

			latestIndex, ok :=
				latest[event.Command]

			if ok && latestIndex != i {
				continue
			}
		}

		result = append(result, event)
	}

	return result
}
```

---

# 11. Update your `GatewayConn`

This is important because your `Connect()` and dispatcher must never concurrently write to the gRPC stream.

```go
package registry

import (
	"context"
	"sync"
	"time"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

type GatewayConn struct {
	GatewayID string

	TenantID string

	Stream gen.ControlPlaneService_ConnectServer

	ConnectedAt time.Time

	LastSeen time.Time

	CurrentPolicyVersion uint64

	Ctx context.Context

	Cancel context.CancelFunc

	sendMu sync.Mutex

	ackMu sync.Mutex

	ackWaiters map[int64]chan error
}
```

Initialize:

```go
func NewGatewayConn(
	ctx context.Context,
	cancel context.CancelFunc,
	stream gen.ControlPlaneService_ConnectServer,
	gatewayID string,
	tenantID string,
	policyVersion uint64,
) *GatewayConn {

	return &GatewayConn{
		GatewayID: gatewayID,
		TenantID:  tenantID,
		Stream:    stream,

		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),

		CurrentPolicyVersion: policyVersion,

		Ctx:    ctx,
		Cancel: cancel,

		ackWaiters: make(map[int64]chan error),
	}
}
```

---

# 12. Serialized Send

```go
func (c *GatewayConn) Send(
	envelope *gen.CPEnvelope,
) error {

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	return c.Stream.Send(envelope)
}
```

This means:

```text
Connect reconciliation
        │
        ├── Send()
        │
        │ locked
        ▼

Policy distributor
        │
        ├── Send()
        │
        │ waits
        ▼
```

No concurrent stream corruption.

---

# 13. ACK manager

```go
func (c *GatewayConn) RegisterAck(
	seq int64,
) <-chan error {

	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	ch := make(chan error, 1)

	c.ackWaiters[seq] = ch

	return ch
}
```

Resolve:

```go
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
```

---

# 14. ACK cumulative resolution

Because ACK is:

```text
processed_through_seq = 742
```

you should resolve **all waiters <= 742**.

```go
func (c *GatewayConn) ResolveThrough(
	seq int64,
	err error,
) {

	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	for waitingSeq, ch :=
		range c.ackWaiters {

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
```

---

# 15. `SendEventAndWaitAck`

```go
func SendEventAndWaitAck(
	ctx context.Context,
	conn *registry.GatewayConn,
	event store.GatewayEvent,
) error {

	cmd, err :=
		dispatcher.GatewayEventToCmd(event)

	if err != nil {
		return err
	}

	ackCh :=
		conn.RegisterAck(event.Seq)

	envelope := &proto.CPEnvelope{
		Seq: event.Seq,

		EventId: event.EventID.String(),

		SentAt: timestamppb.Now(),

		Payload: &proto.CPEnvelope_Cmd{
			Cmd: cmd,
		},
	}

	if err := conn.Send(envelope); err != nil {

		conn.ResolveAck(
			event.Seq,
			err,
		)

		return fmt.Errorf(
			"send event %d: %w",
			event.Seq,
			err,
		)
	}

	select {

	case err := <-ackCh:
		return err

	case <-ctx.Done():
		return ctx.Err()
	}
}
```

---

# 16. Reconciliation service

Create:

```text
internal/cp/platform/cp_grpc/reconciler.go
```

```go
package cp_grpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/dispatcher"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"

	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *cpServer) reconcileGateway(
	ctx context.Context,
	conn *registry.GatewayConn,
) error {

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return fmt.Errorf(
			"invalid gateway ID: %w",
			err,
		)
	}

	// --------------------------------------------
	// 1. Get authoritative cursor from CP
	// --------------------------------------------

	lastAck, err :=
		s.gatewayRepo.GetLastAckedSeqForGateway(
			ctx,
			gatewayID,
		)

	if err != nil {

		s.log.Error(
			"failed to get gateway event ACK",
			slog.String(
				"gateway_id",
				conn.GatewayID,
			),
			slog.String(
				"error",
				err.Error(),
			),
		)

		return err
	}

	// --------------------------------------------
	// 2. Get every event after cursor
	// --------------------------------------------

	events, err :=
		s.gatewayRepo.ListGatewayEventsAfter(
			ctx,
			store.ListGatewayEventsAfterParams{
				GatewayID: gatewayID,
				Seq:       lastAck,
			},
		)

	if err != nil {
		return fmt.Errorf(
			"load gateway events: %w",
			err,
		)
	}

	if len(events) == 0 {
		return nil
	}

	// --------------------------------------------
	// 3. Compact complete snapshots
	// --------------------------------------------

	events =
		dispatcher.CompactGatewayEvents(
			events,
		)

	// --------------------------------------------
	// 4. Send in original sequence order
	// --------------------------------------------

	for _, event := range events {

		cmd, err :=
			dispatcher.GatewayEventToCmd(
				event,
			)

		if err != nil {
			return fmt.Errorf(
				"convert event %d: %w",
				event.Seq,
				err,
			)
		}

		envelope := &proto.CPEnvelope{
			Seq: event.Seq,

			EventId: event.EventID.String(),

			SentAt: timestamppb.Now(),

			Payload: &proto.CPEnvelope_Cmd{
				Cmd: cmd,
			},
		}

		if err := conn.Send(envelope); err != nil {
			return fmt.Errorf(
				"send event %d: %w",
				event.Seq,
				err,
			)
		}
	}

	return nil
}
```

Notice something important:

**I don't wait for each ACK here.**

Why?

Because the gateway can receive the entire reconciled batch and ACK cumulatively.

---

# 17. Gateway ACK

Gateway receives:

```text
100
101
102
103
```

and successfully executes all four.

It sends:

```text
processed_through_seq = 103
```

CP updates:

```text
last_acked_seq = 103
```

Done.

---

# 18. But snapshot compaction creates a subtle ACK issue

Suppose CP has:

```text
100 ACTION
101 CRL_SYNC
102 ACTION
103 CRL_SYNC
```

and compacts:

```text
100 ACTION
102 ACTION
103 CRL_SYNC
```

The gateway executes:

```text
100
102
103
```

and ACKs:

```text
103
```

That's valid because:

```text
101
```

was deliberately superseded by:

```text
103
```

The ACK means:

> "I have reconciled my state through the CP's event stream through sequence 103."

Not:

> "I executed every physical row in the database."

That's the correct semantic.

---

# 19. Gateway-side command processing

Your gateway should process sequentially.

```go
func (g *Gateway) handleCPEnvelope(
	ctx context.Context,
	env *proto.CPEnvelope,
) error {

	cmd := env.GetCmd()

	if cmd == nil {
		return fmt.Errorf(
			"CP envelope %d has no command",
			env.Seq,
		)
	}

	if err := g.executeCommand(
		ctx,
		cmd,
	); err != nil {

		g.logger.Error(
			"command execution failed",
			"seq", env.Seq,
			"event_id", env.EventId,
			"error", err,
		)

		return err
	}

	return nil
}
```

---

# 20. Execute command

```go
func (g *Gateway) executeCommand(
	ctx context.Context,
	cmd *proto.Command,
) error {

	switch p := cmd.Payload.(type) {

	case *proto.Command_CrlSync:

		return g.applyCRLSnapshot(
			ctx,
			p.CrlSync,
		)

	case *proto.Command_ConnectorSync:

		return g.applyConnectorSnapshot(
			ctx,
			p.ConnectorSync,
		)

	case *proto.Command_RevokeConnector:

		return g.revokeConnector(
			ctx,
			p.RevokeConnector,
		)

	case *proto.Command_RevokeConnectorCert:

		return g.revokeConnectorCert(
			ctx,
			p.RevokeConnectorCert,
		)

	case *proto.Command_RotateConnectorCert:

		return g.rotateConnectorCert(
			ctx,
			p.RotateConnectorCert,
		)

	case *proto.Command_RevokeSession:

		return g.revokeSession(
			ctx,
			p.RevokeSession,
		)

	case *proto.Command_DrainGateway:

		return g.drain(
			ctx,
			p.DrainGateway,
		)

	case *proto.Command_RevokeGateway:

		return g.revokeGateway(
			ctx,
			p.RevokeGateway,
		)

	default:

		return fmt.Errorf(
			"unsupported command: %T",
			p,
		)
	}
}
```

---

# 21. Gateway sends ACK only after successful execution

```go
func (g *Gateway) processCommand(
	ctx context.Context,
	stream proto.ControlPlaneService_ConnectClient,
	env *proto.CPEnvelope,
) error {

	if err := g.handleCPEnvelope(
		ctx,
		env,
	); err != nil {
		return err
	}

	ack := &proto.GatewayEnvelope{
		Payload: &proto.GatewayEnvelope_CmdAck{
			CmdAck: &proto.CommandAck{
				GatewayId: g.id,

				ProcessedThroughSeq: env.Seq,
			},
		},
	}

	return stream.Send(ack)
}
```

---

# 22. CP receive loop

Now your current giant `for` loop becomes:

```go
func (s *cpServer) receiveLoop(
	ctx context.Context,
	stream proto.ControlPlaneService_ConnectServer,
	conn *registry.GatewayConn,
) error {

	for {

		msg, err := stream.Recv()

		if err == io.EOF {
			s.log.Info(
				"gateway disconnected",
				slog.String(
					"gateway_id",
					conn.GatewayID,
				),
			)

			return nil
		}

		if err != nil {
			return fmt.Errorf(
				"gateway stream receive: %w",
				err,
			)
		}

		conn.LastSeen = time.Now()

		switch p := msg.Payload.(type) {

		case *proto.GatewayEnvelope_Hello:

			if err := s.handleHello(
				ctx,
				conn,
				p.Hello,
			); err != nil {
				return err
			}

		case *proto.GatewayEnvelope_Heartbeat:

			if err := s.handleHeartbeat(
				ctx,
				conn,
				p.Heartbeat,
			); err != nil {
				s.log.Warn(
					"heartbeat handling failed",
					"error", err,
				)
			}

		case *proto.GatewayEnvelope_CmdAck:

			if err := s.handleCommandAck(
				ctx,
				conn,
				p.CmdAck,
			); err != nil {

				s.log.Error(
					"command ACK handling failed",
					"error", err,
				)

				return err
			}

		default:

			s.log.Warn(
				"unknown gateway message",
				"gateway_id",
				conn.GatewayID,
			)
		}
	}
}
```

---

# 23. ACK handler

This replaces your current `strconv.ParseInt(p.CmdAck.CmdId...)`.

```go
func (s *cpServer) handleCommandAck(
	ctx context.Context,
	conn *registry.GatewayConn,
	ack *proto.CommandAck,
) error {

	if ack.GatewayId != conn.GatewayID {
		return status.Error(
			codes.PermissionDenied,
			"gateway ACK identity mismatch",
		)
	}

	seq := ack.ProcessedThroughSeq

	if seq < 0 {
		return status.Error(
			codes.InvalidArgument,
			"invalid ACK sequence",
		)
	}

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return err
	}

	// --------------------------------------------
	// Verify ACK cannot jump beyond known events
	// --------------------------------------------

	latest, err :=
		s.gatewayRepo.GetLatestGatewayEventSeq(
			ctx,
			gatewayID,
		)

	if err != nil {
		return err
	}

	if seq > latest {
		return status.Errorf(
			codes.InvalidArgument,
			"ACK %d exceeds latest event %d",
			seq,
			latest,
		)
	}

	// --------------------------------------------
	// Monotonic database update
	// --------------------------------------------

	if err :=
		s.gatewayRepo.AckGatewayEvents(
			ctx,
			store.AckGatewayEventsParams{
				GatewayID:     gatewayID,
				LastAckedSeq: seq,
			},
		); err != nil {

		return fmt.Errorf(
			"persist gateway ACK: %w",
			err,
		)
	}

	// Wake any active delivery waiter.
	conn.ResolveThrough(seq, nil)

	return nil
}
```

---

# 24. Hello handler

Your existing Hello handling can remain:

```go
func (s *cpServer) handleHello(
	ctx context.Context,
	conn *registry.GatewayConn,
	hello *proto.Hello,
) error {

	if hello.GatewayId != conn.GatewayID {
		return status.Error(
			codes.Unauthenticated,
			"gateway identity mismatch",
		)
	}

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return err
	}

	_, err =
		s.gatewayRepo.UpdateGatewayBinaryVersion(
			ctx,
			store.UpdateGatewayBinaryVersionParams{
				ID: gatewayID,

				Version: pgtype.Text{
					Valid:  true,
					String: hello.BinaryVersion,
				},
			},
		)

	if err != nil {
		s.log.Warn(
			"failed to update gateway binary version",
			"gateway_id",
			conn.GatewayID,
			"error",
			err,
		)
	}

	return conn.Send(
		&proto.CPEnvelope{
			Payload: &proto.CPEnvelope_HelloAck{
				HelloAck: &proto.HelloAck{
					ServerVersion: "1.0.0",
					ServerTime:    timestamppb.Now(),
				},
			},
		},
	)
}
```

---

# 25. Heartbeat

```go
func (s *cpServer) handleHeartbeat(
	ctx context.Context,
	conn *registry.GatewayConn,
	heartbeat *proto.Heartbeat,
) error {

	if heartbeat.GatewayId != conn.GatewayID {
		return status.Error(
			codes.PermissionDenied,
			"heartbeat gateway mismatch",
		)
	}

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return err
	}

	_, err =
		s.gatewayRepo.UpdateGatewayHeartBeat(
			ctx,
			gatewayID,
		)

	return err
}
```

---

# 26. Final `Connect()`

This is what your current function should converge toward:

```go
func (s *cpServer) Connect(
	stream proto.ControlPlaneService_ConnectServer,
) error {

	// ==================================================
	// 1. Authenticate mTLS identity
	// ==================================================

	gw, err :=
		s.authenticateGateway(stream)

	if err != nil {
		return err
	}

	// ==================================================
	// 2. Receive Hello
	// ==================================================

	hello, err :=
		s.receiveHello(stream)

	if err != nil {
		return err
	}

	// ==================================================
	// 3. Verify Hello identity against certificate
	// ==================================================

	gatewayID := gw.ID.String()

	if hello.GatewayId != gatewayID {

		return status.Error(
			codes.Unauthenticated,
			"gateway identity mismatch",
		)
	}

	// ==================================================
	// 4. Register connection
	// ==================================================

	ctx, cancel :=
		context.WithCancel(
			stream.Context(),
		)

	defer cancel()

	conn :=
		registry.NewGatewayConn(
			ctx,
			cancel,
			stream,
			gatewayID,
			hello.TenantId,
			uint64(hello.PolicyVersion),
		)

	if _, exists :=
		s.registry.GetConnection(gatewayID); exists {

		return status.Error(
			codes.AlreadyExists,
			"gateway already connected",
		)
	}

	s.registry.Register(conn)

	defer s.registry.Unregister(
		gatewayID,
	)

	s.log.Info(
		"gateway connected",
		slog.String(
			"gateway_id",
			gatewayID,
		),
		slog.String(
			"tenant_id",
			hello.TenantId,
		),
		slog.Uint64(
			"policy_version",
			uint64(hello.PolicyVersion),
		),
	)

	// ==================================================
	// 5. Reconcile durable gateway events
	// ==================================================

	if err := s.reconcileGateway(
		ctx,
		conn,
	); err != nil {

		s.log.Error(
			"gateway reconciliation failed",
			slog.String(
				"gateway_id",
				gatewayID,
			),
			slog.String(
				"error",
				err.Error(),
			),
		)

		return status.Error(
			codes.Internal,
			"gateway reconciliation failed",
		)
	}

	// ==================================================
	// 6. Policy reconciliation
	// ==================================================

	tenantUUID, err :=
		uuid.Parse(hello.TenantId)

	if err != nil {
		return status.Error(
			codes.InvalidArgument,
			"invalid tenant ID",
		)
	}

	if uint64(hello.PolicyVersion) <
		s.distributor.LatestVersion(
			tenantUUID,
		) {

		go func() {

			if err :=
				s.distributor.PushToGateway(
					ctx,
					conn,
					tenantUUID,
				); err != nil {

				s.log.Error(
					"policy reconciliation failed",
					slog.String(
						"gateway_id",
						gatewayID,
					),
					slog.String(
						"error",
						err.Error(),
					),
				)
			}

		}()
	}

	// ==================================================
	// 7. Normal stream
	// ==================================================

	return s.receiveLoop(
		ctx,
		stream,
		conn,
	)
}
```

---

# 27. Active command delivery

Now replace your current bounded `CommandJob` queue with a **wake-up mechanism**.

The important distinction:

```text
BAD:

Command
 ↓
memory queue
 ↓
queue full
 ↓
DROP


GOOD:

Command
 ↓
PostgreSQL event
 ↓
Wake connected gateway
 ↓
Gateway reads DB
```

So the database is written first.

---

# 28. Active gateway worker

```go
type GatewayDispatcher struct {
	store    *store.Queries
	registry *registry.GatewayRegistry

	wakeMu sync.Mutex

	wakeups map[string]chan struct{}

	logger *slog.Logger
}
```

Constructor:

```go
func NewGatewayDispatcher(
	q *store.Queries,
	reg *registry.GatewayRegistry,
	logger *slog.Logger,
) *GatewayDispatcher {

	return &GatewayDispatcher{
		store:    q,
		registry: reg,
		logger:   logger,

		wakeups: make(
			map[string]chan struct{},
		),
	}
}
```

---

# 29. Wake gateway

```go
func (d *GatewayDispatcher) Wakeup(
	gatewayID string,
) {

	d.wakeMu.Lock()

	ch, exists :=
		d.wakeups[gatewayID]

	if !exists {

		ch = make(chan struct{}, 1)

		d.wakeups[gatewayID] = ch

		go d.worker(
			gatewayID,
			ch,
		)
	}

	d.wakeMu.Unlock()

	select {
	case ch <- struct{}{}:
	default:
	}
}
```

The important part:

```text
10 commands
   ↓
10 DB rows
   ↓
10 Wakeup()
   ↓
1 wake signal
   ↓
worker reads DB
```

No command is lost.

---

# 30. Worker

```go
func (d *GatewayDispatcher) worker(
	gatewayID string,
	wakeup <-chan struct{},
) {

	for range wakeup {

		conn, exists :=
			d.registry.GetConnection(
				gatewayID,
			)

		if !exists {
			continue
		}

		ctx, cancel :=
			context.WithTimeout(
				context.Background(),
				30*time.Second,
			)

		err :=
			d.deliverPending(
				ctx,
				conn,
			)

		cancel()

		if err != nil {

			d.logger.Error(
				"gateway event delivery failed",
				slog.String(
					"gateway_id",
					gatewayID,
				),
				slog.String(
					"error",
					err.Error(),
				),
			)
		}
	}
}
```

---

# 31. Deliver pending

```go
func (d *GatewayDispatcher) deliverPending(
	ctx context.Context,
	conn *registry.GatewayConn,
) error {

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return err
	}

	lastAck, err :=
		d.store.GetLastAckedSeqForGateway(
			ctx,
			gatewayID,
		)

	if err != nil {
		return err
	}

	events, err :=
		d.store.ListGatewayEventsAfter(
			ctx,
			store.ListGatewayEventsAfterParams{
				GatewayID: gatewayID,
				Seq:       lastAck,
			},
		)

	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	events =
		dispatcher.CompactGatewayEvents(
			events,
		)

	for _, event := range events {

		cmd, err :=
			dispatcher.GatewayEventToCmd(
				event,
			)

		if err != nil {
			return err
		}

		envelope :=
			&proto.CPEnvelope{
				Seq: event.Seq,

				EventId:
					event.EventID.String(),

				SentAt:
					timestamppb.Now(),

				Payload:
					&proto.CPEnvelope_Cmd{
						Cmd: cmd,
					},
			}

		if err := conn.Send(
			envelope,
		); err != nil {
			return err
		}
	}

	return nil
}
```

---

# 32. Creating a command

Now your application code does:

```go
job := dispatcher.CommandJob{
	Type:       dispatcher.CmdRevokeConnector,
	GatewayID:  gatewayID,
	ConnectorID: connectorID,
}

event, err :=
	eventService.CreateEvent(
		ctx,
		job,
	)

if err != nil {
	return err
}

gatewayDispatcher.Wakeup(
	gatewayID,
)
```

The critical ordering is:

```text
CreateEvent()
    ↓
COMMIT
    ↓
Wakeup()
```

**Never wake first.**

---

# 33. CRL revocation flow

This is particularly important for your question about CRL.

Suppose:

```go
func RevokeCertificate(...) error
```

The transaction should conceptually be:

```sql
BEGIN;

UPDATE component_certificates
SET revoked_at = NOW()
WHERE id = $1;

INSERT INTO crl_entries (...);

-- allocate gateway seq
-- insert CRL_SYNC snapshot event

COMMIT;
```

Then:

```go
gatewayDispatcher.Wakeup(gatewayID)
```

If the gateway is connected:

```text
DB
 ↓
wake
 ↓
dispatcher
 ↓
CRL_SYNC
 ↓
gateway
```

If it isn't connected:

```text
DB
 ↓
nothing
```

Later:

```text
gateway reconnects
 ↓
Connect()
 ↓
last_acked_seq
 ↓
events > cursor
 ↓
latest CRL_SYNC
```

So **you do not need to separately query the current CRL during `Connect()`** if `CRL_SYNC` events are guaranteed to exist for every relevant state change.

---

# 34. However, I would add one safety mechanism

Even with event replay, I would have a periodic **state reconciliation fallback**.

Not:

```text
every reconnect → always send entire CRL
```

but:

```text
every reconnect:
    replay event log

periodically:
    verify state version
```

For example:

```protobuf
message Hello {
    string gateway_id = 1;
    string tenant_id = 2;
    int64 policy_version = 3;

    uint64 crl_version = 4;
    uint64 connector_version = 5;

    string binary_version = 6;
}
```

Then CP can detect:

```text
CP CRL version:       87
Gateway CRL version:  86
```

and generate/send a current snapshot.

This is not the event cursor.

They're different concepts:

```text
last_acked_seq
    =
delivery state


crl_version
    =
application state
```

You don't need to persist `last_acked_seq` in bbolt.

---

# 35. Gateway bbolt

Your gateway can therefore store:

```text
bbolt
├── crl
│   ├── version
│   └── snapshot
│
├── connectors
│   ├── version
│   └── snapshot
│
├── policy
│   ├── version
│   └── bundle
│
└── gateway
    └── configuration
```

Not:

```text
last_acked_seq
```

That remains CP-owned.

---

# 36. One critical issue with `REVOKE_GATEWAY`

Your current code does:

```go
if job.Type == CmdRevokeGateway {
	conn.Cancel()
}
```

immediately after sending.

Remove that.

It should be:

```text
CP
 │
 │ REVOKE_GATEWAY seq=500
 ▼
Gateway
 │
 │ revoke itself
 │
 │ ACK 500
 ▼
CP
 │
 │ persist ACK
 │
 ▼
connection closes
```

If you disconnect immediately after `Send()`, you don't know whether the gateway actually executed the command.

---

# 37. Another important issue in your existing code

You currently have:

```go
if err == io.EOF {
    log.Fatal("Gateway Disconnected Cleanly")
}
```

**Never use `log.Fatal()` inside a gRPC connection handler.**

`log.Fatal()` terminates the entire CP process.

Use:

```go
if err == io.EOF {
	s.log.Info(
		"gateway disconnected cleanly",
		slog.String(
			"gateway_id",
			conn.GatewayID,
		),
	)

	return nil
}
```

Likewise, avoid the scattered:

```go
fmt.Println()
log.Println()
```

and use your `slog.Logger`.

---

# 38. Final architecture

Your resulting system is now:

```text
                    CONTROL PLANE
                         │
                         │
                 ┌───────▼────────┐
                 │   PostgreSQL   │
                 │                │
                 │ gateway_events │
                 │ event ACKs     │
                 │ CRL            │
                 │ connectors     │
                 │ certificates   │
                 └───────┬────────┘
                         │
             ┌───────────┴────────────┐
             │                        │
       CONNECT/RECONNECT          ACTIVE CHANGE
             │                        │
             ▼                        ▼
      reconcileGateway()          CreateEvent()
             │                        │
             │                    COMMIT
             │                        │
             │                      Wakeup
             │                        │
             └───────────┬────────────┘
                         ▼
                  GatewayDispatcher
                         │
                         ▼
                    GatewayConn
                         │
                         │ serialized Send()
                         ▼
                    gRPC stream
                         │
                         ▼
                       GATEWAY
                         │
                  execute command
                         │
                    update bbolt
                         │
                         ▼
                  ACK(seq = N)
                         │
                         ▼
                    PostgreSQL
                         │
                  last_acked_seq=N
```

And the fundamental guarantees are:

### If CP crashes

Events survive in PostgreSQL.

### If the dispatcher crashes

Events survive in PostgreSQL.

### If the gRPC connection dies

Events survive in PostgreSQL.

### If the gateway crashes after executing but before ACK

The command may execute again.

Therefore **every action handler must be idempotent**.

### If the gateway misses 50 CRL snapshots

Only the latest CRL snapshot needs to be replayed.

### If the gateway misses 50 session revocations

All 50 action events are replayed.

### If the gateway is connected when a certificate is revoked

The new `CRL_SYNC` event is persisted and the dispatcher wakes the connection immediately.

### If the gateway is disconnected

The same event remains in `gateway_events` and is picked up on the next `Connect()`.

---

## The one thing I'd change from your current implementation immediately

Your current flow is essentially:

```text
CommandJob
   ↓
bounded memory queue
   ↓
Send()
```

Change it to:

```text
CommandJob
   ↓
PostgreSQL transaction
   ↓
gateway_events
   ↓
Wakeup()
   ↓
Send()
```

That is the architectural boundary you were missing.

For Ashrix, **the dispatcher should never be the owner of command durability**. PostgreSQL is. The dispatcher is merely the mechanism that makes durable commands reach a currently connected gateway quickly. The `Connect()` reconciliation is the recovery mechanism. And the gateway's bbolt is the state store, **not the event cursor**.
