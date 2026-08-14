-- =================================================================
-- AUDIT LOGS
-- Append-only. Never update or delete.
-- =================================================================

-- name: CreateAuditLog :one
INSERT INTO audit_logs (org_id, actor_id, action, target_type, target_id, details, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAuditLogsByOrg :many
SELECT * FROM audit_logs
WHERE org_id     = $1
  AND created_at >= $2
  AND created_at <= $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: ListAuditLogsByActor :many
SELECT * FROM audit_logs
WHERE org_id     = $1
  AND actor_id   = $2
  AND created_at >= $3
  AND created_at <= $4
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: ListAuditLogsByTarget :many
SELECT * FROM audit_logs
WHERE org_id      = $1
  AND target_type = $2
  AND target_id   = $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;


-- =================================================================
-- ACCESS LOGS
-- Written async by Gateway in batches. CP never in traffic path.
-- =================================================================

-- name: CreateAccessLog :one
INSERT INTO access_logs (
    org_id, app_id, idp_config_id, user_email,
    method, path, status, latency_ms,
    ip, result, deny_reason
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: BulkCreateAccessLogs :copyfrom
-- Uses pgx COPY protocol for high-throughput batch inserts.
-- Gateway drains its local buffer to CP every 10s.
INSERT INTO access_logs (
    org_id, app_id, idp_config_id, user_email,
    method, path, status, latency_ms,
    ip, result, deny_reason
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListAccessLogsByOrg :many
SELECT * FROM access_logs
WHERE org_id     = $1
  AND created_at >= $2
  AND created_at <= $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: ListAccessLogsByApp :many
SELECT * FROM access_logs
WHERE app_id     = $1
  AND org_id     = $2
  AND created_at >= $3
  AND created_at <= $4
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: ListDeniedAccessLogs :many
-- Dashboard denied requests view.
SELECT * FROM access_logs
WHERE org_id     = $1
  AND result     = 'denied'
  AND created_at >= $2
  AND created_at <= $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountAccessLogsByResult :one
-- Dashboard summary counts.
SELECT
    COUNT(*) FILTER (WHERE result = 'allowed') AS allowed_count,
    COUNT(*) FILTER (WHERE result = 'denied')  AS denied_count
FROM access_logs
WHERE org_id     = $1
  AND created_at >= $2
  AND created_at <= $3;

-- name: PurgeOldAccessLogs :exec
-- Retention policy. Default 90 days. Run via background job.
DELETE FROM access_logs
WHERE created_at < now() - ($1 || ' days')::interval;
