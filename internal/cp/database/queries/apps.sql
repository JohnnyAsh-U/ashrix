-- =================================================================
-- APPS
-- =================================================================

-- name: CreateApp :one
INSERT INTO apps (org_id, connector_id, name, subdomain, upstream, protocol, is_public, enable_security_headers, sock_pass, check_health, check_interval, health_endpoint)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetAppByID :one
SELECT * FROM apps
WHERE id         = $1
  AND deleted_at IS NULL;

-- name: GetAppByIDAndOrg :one
SELECT * FROM apps
WHERE id         = $1
  AND org_id     = $2
  AND deleted_at IS NULL;

-- name: GetAppBySubdomain :one
-- Called by Gateway to resolve subdomain to app during routing.
SELECT * FROM apps
WHERE org_id     = $1
  AND subdomain  = $2
  AND deleted_at IS NULL;

-- name: ListAppsByOrg :many
SELECT * FROM apps
WHERE org_id     = $1
  AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: ListAppsWithDetailsByOrg :many
SELECT a.*,
       c.name AS connector_name,
       g.name AS gateway_name
FROM apps a
LEFT JOIN connectors c ON a.connector_id = c.id
LEFT JOIN gateways g ON c.gateway_id = g.id
WHERE a.org_id     = $1
  AND a.deleted_at IS NULL
ORDER BY a.created_at ASC;

-- name: ListAppsByConnector :many
SELECT * FROM apps
WHERE connector_id = $1
  AND deleted_at   IS NULL;

-- name: ListAppsByGateway :many
SELECT a.*
FROM apps a
JOIN connectors c ON a.connector_id = c.id
WHERE (c.gateway_id = $1 OR c.secondary_gateway_id = $1)
  AND a.deleted_at IS NULL;

-- name: UpdateApp :one
UPDATE apps
SET name                    = $3,
    subdomain               = $4,
    upstream                = $5,
    protocol                = $6,
    is_public               = $7,
    enable_security_headers = $8,
    connector_id            = $9,
    sock_pass               = $10,
    check_health            = $11,
    check_interval          = $12,
    health_endpoint         = $13
WHERE id         = $1
  AND org_id     = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateAppHealthStatus :one
UPDATE apps
SET health_status = $2,
    last_seen     = $3
WHERE id         = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteApp :one
-- Soft delete. Policies referencing this app remain for audit history.
UPDATE apps
SET deleted_at = now()
WHERE id         = $1
  AND org_id     = $2
  AND deleted_at IS NULL
RETURNING *;


-- name: MarkOfflineStaleApp :execrows
UPDATE apps
SET health_status = 'unhealthy'
WHERE check_health = true
  AND health_status = 'healthy'
  AND last_seen < NOW() - (sqlc.arg(threshold_minutes)::text || ' minutes')::interval
  AND last_seen IS NOT NULL;

