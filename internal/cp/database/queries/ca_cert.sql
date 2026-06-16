-- name: GetActiveCACert :one
SELECT id, name, type, cert_pem, serial_number, subject, issued_at, expires_at
FROM ca_certificates
WHERE name = $1 AND type = $2 AND revoked_at IS NULL;

-- name: ListActiveCACerts :many
SELECT id, name, type, cert_pem, serial_number, subject, issued_at, expires_at
FROM ca_certificates
WHERE revoked_at IS NULL
ORDER BY issued_at ASC;

-- name: InsertCACert :one
INSERT INTO ca_certificates (name, type, cert_pem, serial_number, subject, issued_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: DeactivateCACert :one
UPDATE ca_certificates
SET revoked_at = now()
WHERE id = $1
RETURNING id;


-- =================================================================
-- Cert Components
-- =================================================================

-- name: RegisterCompCert :one
INSERT INTO component_certificates (org_id, component_type, component_id, ca_id, cert_pem, serial_number, subject, san, issued_at, expires_at, rotation_of)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: RevokeCompCert :one
UPDATE component_certificates
SET revoked_at = NOW(),
    revoke_reason = $3
WHERE component_id = $1
AND component_type = $2
AND revoked_at IS NULL
RETURNING *;


-- name: GetValidCompCert :one
SELECT id, org_id, component_type, component_id, cert_pem, serial_number, subject,san, issued_at
FROM component_certificates
WHERE component_type = $1
AND component_id = $2
AND revoked_at IS NULL;

