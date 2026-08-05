-- =================================================================
-- REVOCATIONS
-- CP writes. Gateway polls. CP never touches traffic path.
-- =================================================================

-- name: CreateRevocation :one
INSERT INTO revocations (org_id, type, target_id, reason, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListRevocationsSince :many
-- Gateway polls this on every sync cycle.
-- Returns only non-expired revocations created after last_seen_at.
-- Gateway passes its last sync timestamp to get only new entries.
SELECT * FROM revocations
WHERE org_id     = $1
  AND created_at > $2
  AND expires_at > now()
ORDER BY created_at ASC;

-- name: PurgeExpiredRevocations :exec
-- Housekeeping. Run periodically.
DELETE FROM revocations
WHERE expires_at < now();


-- =================================================================
-- CERTIFICATES — CA
-- =================================================================

-- name: CreateCACertificate :one
INSERT INTO ca_certificates (name, type, cert_pem, serial_number, subject, issued_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetActiveCACertByType :one
-- Returns the active (non-revoked) CA cert of a given type.
SELECT * FROM ca_certificates
WHERE type       = $1
  AND revoked_at IS NULL
ORDER BY issued_at DESC
LIMIT 1;

-- name: ListCACertificates :many
SELECT * FROM ca_certificates
ORDER BY issued_at DESC;


-- =================================================================
-- CERTIFICATES — COMPONENTS
-- =================================================================

-- name: CreateComponentCertificate :one
INSERT INTO component_certificates (
    org_id, component_type, component_id, ca_id,
    cert_pem, serial_number, subject, san,
    issued_at, expires_at, rotation_of
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetActiveComponentCert :one
-- Returns the current valid cert for a gateway or connector.
SELECT * FROM component_certificates
WHERE component_type = $1
  AND component_id   = $2
  AND revoked_at     IS NULL
ORDER BY issued_at DESC
LIMIT 1;

-- name: GetActiveComponentCertByType :one
-- Returns all valid certs of a given type.
SELECT * FROM component_certificates
WHERE component_type = $1
  AND revoked_at IS NULL
ORDER BY issued_at DESC
LIMIT 1;

-- name: ListExpiringComponentCerts :many
-- Used by background job to alert before expiry.
-- Returns certs expiring within the next $1 days.
SELECT * FROM component_certificates
WHERE expires_at   < now() + ($1 || ' days')::interval
  AND revoked_at   IS NULL
ORDER BY expires_at ASC;

-- name: RevokeComponentCertificate :one
UPDATE component_certificates
SET revoked_at    = now(),
    revoke_reason = $2
WHERE id          = $1
  AND revoked_at  IS NULL
RETURNING *;


-- -- =================================================================
-- -- CSR REQUESTS
-- -- =================================================================

-- -- name: CreateCSRRequest :one
-- INSERT INTO csr_requests (org_id, component_type, component_id, csr_pem)
-- VALUES ($1, $2, $3, $4)
-- RETURNING *;

-- -- name: GetCSRRequest :one
-- SELECT * FROM csr_requests
-- WHERE id = $1;

-- -- name: MarkCSRSigned :one
-- UPDATE csr_requests
-- SET status         = 'signed',
--     signed_cert_id = $2,
--     processed_at   = now()
-- WHERE id           = $1
--   AND status       = 'pending'
-- RETURNING *;

-- -- name: MarkCSRRejected :one
-- UPDATE csr_requests
-- SET status       = 'rejected',
--     processed_at = now()
-- WHERE id         = $1
--   AND status     = 'pending'
-- RETURNING *;

-- -- name: ListPendingCSRs :many
-- SELECT * FROM csr_requests
-- WHERE status = 'pending'
-- ORDER BY created_at ASC;


-- =================================================================
-- CRL ENTRIES
-- =================================================================

-- name: CreateCRLEntry :one
INSERT INTO crl_entries (cert_id, serial_number, reason)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCRLEntryBySerial :one
-- Gateway calls this to check if a presented cert is revoked.
SELECT * FROM crl_entries
WHERE serial_number = $1;

-- name: ListCRLEntries :many
-- Gateway fetches full CRL on startup and after each sync.
SELECT * FROM crl_entries
ORDER BY revoked_at DESC;


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
