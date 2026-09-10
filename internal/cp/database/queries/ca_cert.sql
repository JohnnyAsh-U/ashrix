-- name: GetActiveCACert :one
SELECT id, name, type, cert_pem, serial_number, subject, issued_at, expires_at
FROM ca_certificates
WHERE name = $1 AND type = $2 AND revoked_at IS NULL;

-- name: ListActiveCACerts :many
SELECT id, name, type, cert_pem, serial_number, subject, issued_at, expires_at
FROM ca_certificates
WHERE name = $1 AND type = $2 AND revoked_at IS NULL;

-- name: InsertCACert :one
INSERT INTO ca_certificates (name, type, cert_pem, serial_number, subject, issued_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: DeactivateCACert :one
UPDATE ca_certificates
SET revoked_at = now()
WHERE id = $1
RETURNING id;


-- name: GetActiveCACertByType :one
-- Returns the active (non-revoked) CA cert of a given type.
SELECT * FROM ca_certificates
WHERE type       = $1
  AND revoked_at IS NULL
ORDER BY issued_at DESC
LIMIT 1;



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



-- =================================================================
-- CRL ENTRIES
-- =================================================================

-- name: CreateCRLEntry :one
INSERT INTO crl_entries (cert_id, serial_number, org_id, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetCRLEntryBySerial :one
-- Gateway calls this to check if a presented cert is revoked.
SELECT * FROM crl_entries
WHERE serial_number = $1;

-- name: ListCRLEntriesByOrg :many
-- Gateway fetches full CRL on startup and after each sync.
SELECT * FROM crl_entries
WHERE org_id = $1
ORDER BY revoked_at DESC;


