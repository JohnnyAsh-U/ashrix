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


-- name: GetLastAckedSeqForGateway :one
SELECT last_acked_seq
FROM gateway_events_acks
WHERE gateway_id = $1;


-- name: AckGatewayEvents :exec
UPDATE gateway_events_acks
SET
    last_acked_seq = GREATEST(
        last_acked_seq,
        $2
    ),
    updated_at = NOW()
WHERE gateway_id = $1;


-- name: GetLatestGatewayEventSeq :one
SELECT COALESCE(MAX(seq), 0)::BIGINT
FROM gateway_events
WHERE gateway_id = $1;


-- name: DeleteAckedGatewayEvents :exec
DELETE FROM gateway_events ge
USING gateway_events_acks ack
WHERE ge.gateway_id = ack.gateway_id
  AND ge.seq <= ack.last_acked_seq
  AND ge.created_at < NOW() - INTERVAL '30 days';