-- =================================================================
-- GATEWAYS
-- token_hash: SHA-256 of enrollment token. Never plaintext.
-- =================================================================

-- name: CreateGateway :one
WITH new_gateway AS (
    INSERT INTO gateways (
        org_id, 
        name, 
        deployment_type, 
        token_hash, 
        public_url, 
        ip_address,
        log_to_cp
    )
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING *
),
init_sequence AS (
    INSERT INTO gateway_event_sequences (gateway_id)
    SELECT id FROM new_gateway
    ON CONFLICT DO NOTHING
),
init_acks AS (
    INSERT INTO gateway_events_acks (gateway_id)
    SELECT id FROM new_gateway
    ON CONFLICT DO NOTHING
)
SELECT * FROM new_gateway;





-- name: ListGatewaysByOrg :many
SELECT * FROM gateways
WHERE org_id     = $1
ORDER BY created_at ASC;

-- name: ListActiveGatewaysByOrg :many
SELECT * FROM gateways
WHERE org_id = $1
AND is_active = true
ORDER BY created_at ASC;


-- name: ReCreateGateway :one
UPDATE gateways 
SET created_at = NOW(), 
    status = 'pending',
    last_heartbeat = NULL,
    revoked_at = NULL,
    version = NULL,
    is_active = true,
    log_to_cp = $6,
    ip_address = $5,
    public_url = $4,
    token_hash = $2,
    name = $3
WHERE id = $1
RETURNING *;



-- name: EnrollGateway :one
UPDATE gateways 
SET enrolled_at = NOW(), 
    status = 'healthy',
    last_heartbeat = NOW()
WHERE token_hash = $1
  AND is_active = true
  AND revoked_at IS NULL
RETURNING *;





-- name: GetActiveGatewayByID :one
SELECT * FROM gateways
WHERE id         = $1
  AND is_active = true
  AND revoked_at IS NULL;


-- name: GetGatewayByID :one
SELECT * FROM gateways
WHERE id         = $1;



-- name: GetGatewayByIDAndOrg :one
SELECT * FROM gateways
WHERE id         = $1
  AND org_id     = $2
  AND is_active = true
  AND revoked_at IS NULL;

-- name: GetGatewayByTokenHash :one
-- Called on every Gateway → CP request (heartbeat, policy sync).
-- Keep this fast — it is on the service path.
SELECT * FROM gateways
WHERE token_hash = $1
  AND status = 'pending'
  AND is_active = true
  AND revoked_at IS NULL;



-- name: UpdateGatewayHeartbeat :one
-- Called every 30s by Gateway. Updates last_heartbeat and version.
UPDATE gateways
SET last_heartbeat = now()
WHERE id           = $1
  AND is_active = true
  AND revoked_at   IS NULL
RETURNING *;


-- name: UpdateGatewayBinaryVersion :one
UPDATE gateways
SET version = $2
WHERE id = $1
  AND is_active = true
  AND revoked_at IS NULL
RETURNING *;



-- name: RevokeGateway :one
-- Immediately disconnects gateway. Token becomes invalid.
UPDATE gateways
SET revoked_at = now(),
    status     = 'offline',
    is_active = false
WHERE id         = $1
  AND org_id     = $2
  AND is_active = true
  AND revoked_at IS NULL
RETURNING *;


-- name: UpdateGatewayStatus :one
UPDATE gateways
SET status = $2
WHERE id = $1
  AND is_active = true
  AND revoked_at IS NULL
RETURNING *;


-- =================================================================
-- CONNECTORS
-- token_hash: SHA-256 of connector enrollment token.
-- =================================================================

-- name: CreateConnector :one
INSERT INTO connectors (org_id, gateway_id, name, token_hash, open_sock)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;


-- name: ReCreateConnector :one
UPDATE connectors 
SET created_at = NOW(), 
    status = 'pending',
    last_seen = NULL,
    revoked_at = NULL,
    is_active = true,
    active_streams = 0,
    open_sock = $5,
    token_hash = $2,
    name = $3,
    gateway_id = $4
WHERE id = $1
RETURNING *;


-- name: EnrollConnector :one
UPDATE connectors 
SET enrolled_at = NOW(), 
    status = 'connected',
    is_active = true,
    last_seen = NOW()
WHERE token_hash = $1
  AND revoked_at IS NULL
RETURNING *;



-- name: GetConnectorByID :one
SELECT * FROM connectors
WHERE id         = $1;

-- name: GetActiveConnectorByID :one
SELECT * FROM connectors
WHERE id         = $1
  AND is_active = true
  AND revoked_at IS NULL;



-- name: GetConnectorByIDAndOrg :one
SELECT * FROM connectors
WHERE id         = $1
  AND org_id     = $2;


-- name: ListActiveConnectorsByGateway :many
-- Called on gateway → connector auth.
SELECT c.*
FROM connectors c
WHERE c.gateway_id = $1
  AND c.is_active = true
  AND c.revoked_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM component_certificates cc
      WHERE cc.component_id = c.id
        AND cc.component_type = 'connector'
        AND cc.revoked_at IS NULL
        AND cc.expires_at > NOW()
  );

-- name: GetConnectorByTokenHash :one
-- Called on connector → gateway auth.
SELECT * FROM connectors
WHERE token_hash = $1
  AND status = 'pending'
  AND is_active = true
  AND revoked_at IS NULL;

-- name: ListConnectorsByOrg :many
SELECT * FROM connectors
WHERE org_id     = $1
ORDER BY created_at ASC;

-- name: ListConnectorsWithGatewayNameByOrg :many
SELECT c.*, g.name AS gateway_name
FROM connectors c
JOIN gateways g ON c.gateway_id = g.id
WHERE c.org_id = $1
ORDER BY c.created_at ASC;

-- name: ListConnectorsByGateway :many
SELECT * FROM connectors
WHERE gateway_id = $1
  AND is_active = true
  AND revoked_at IS NULL
ORDER BY created_at ASC;

-- name: UpdateConnectorStatus :one
UPDATE connectors
SET last_seen = now(),
    active_streams = $3,
    status    = $2
WHERE id      = $1
  AND is_active = true
  AND revoked_at IS NULL
RETURNING *;

-- name: RevokeConnector :one
UPDATE connectors
SET revoked_at = now(),
    status     = 'disconnected',
    is_active = false,
    active_streams = 0
WHERE id         = $1
  AND gateway_id     = $2
  AND is_active = true
  AND revoked_at IS NULL
RETURNING *;

-- name: RevokeConnectorsByGateway :exec
-- Called when a gateway is revoked — cascade revoke all its connectors.
UPDATE connectors
SET revoked_at = now(),
    status     = 'disconnected',
    active_streams = 0,
    is_active = false
WHERE gateway_id = $1
  AND revoked_at IS NULL;




