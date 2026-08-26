-- =================================================================
-- APPS
-- =================================================================

-- name: CreateApp :one
INSERT INTO apps (org_id, connector_id, name, subdomain, upstream, protocol, is_public, sock_pass)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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

-- name: ListAppsByConnector :many
SELECT * FROM apps
WHERE connector_id = $1
  AND deleted_at   IS NULL;

-- name: UpdateApp :one
UPDATE apps
SET name         = $3,
    subdomain    = $4,
    upstream     = $5,
    protocol     = $6,
    is_public    = $7,
    connector_id = $8,
    sock_pass = $9
WHERE id         = $1
  AND org_id     = $2
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
