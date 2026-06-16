-- =================================================================
-- AUTH
-- =================================================================

-- name: CreateAdmin :one
INSERT INTO admins (email, password_hash, role)
VALUES ($1, $2, $3)
RETURNING *;


-- name: GetAdminByID :one
SELECT * FROM admins
WHERE id = $1
  AND revoked_at IS NULL;


-- name: GetAdminByEmail :one
SELECT * FROM admins
WHERE email   = $1
  AND revoked_at IS NULL;

-- name: GetAdminByEmailWithOrg :one
-- Used at password login. Returns record regardless of sso_only
-- so caller can check sso_only and reject if needed.
SELECT * FROM admins
WHERE org_id = $1
  AND email   = $2
  AND revoked_at IS NULL;


-- name: UpdateAdminOTP :one
UPDATE admins
SET otp_secret = $2,
    otp_enabled = $3
WHERE id = $1
RETURNING *;


-- name: ActivateAdmin :one
-- Called at the end of /verify-otp during setup. Creates the org and links it.
UPDATE admins
SET org_id = $2,
    is_active = TRUE
WHERE id = $1
RETURNING *;


-- name: UpdateAdminPassword :exec
UPDATE admins
SET password_hash = $2
WHERE id = $1;


-- name: CreateSetupToken :one
INSERT INTO admin_setup_tokens (admin_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSetupToken :one
SELECT * FROM admin_setup_tokens
WHERE token_hash = $1
    AND used_at IS NULL
    AND expires_at > NOW();

-- name: MarkSetupTokenUsed :exec
UPDATE admin_setup_tokens
SET used_at = NOW()
WHERE id = $1;


-- name: CreateSession :one
INSERT INTO admin_sessions (admin_id, org_id, refresh_token, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;


-- name: GetAdminSession :one
-- Hot path: called on every dashboard request.
-- Returns session only if valid (not expired, not revoked).
SELECT * FROM admin_sessions
WHERE refresh_token = $1
  AND expires_at  > now()
  AND revoked_at  IS NULL;


-- name: RevokeAdminSession :exec
-- Single session revocation (logout).
UPDATE admin_sessions
SET revoked_at = now()
WHERE id         = $1
  AND revoked_at IS NULL;


-- name: RevokeAllAdminSessionsForAdmin :exec
-- Revoke all sessions for an admin (e.g. role changed, account suspended, passchange/account compromise).
UPDATE admin_sessions
SET revoked_at = now()
WHERE admin_id   = $1
  AND revoked_at IS NULL;


-- name: RevokeAllAdminSessionsForOrg :exec
-- Nuclear option — used when org SSO config changes.
UPDATE admin_sessions
SET revoked_at = now()
WHERE org_id     = $1
  AND revoked_at IS NULL;


-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (admin_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;


-- name: GetPasswordResetToken :one
SELECT * FROM password_reset_tokens
WHERE token_hash = $1
    AND used_at IS NULL
    AND expires_at > NOW();


-- name: MarkPasswordResetTokenUsed :exec
UPDATE password_reset_tokens
SET used_at = NOW()
WHERE id = $1;


-- name: DeleteExpiredAdminSessions :exec
-- Housekeeping. Run periodically via background job.
DELETE FROM admin_sessions
WHERE expires_at < now()
   OR revoked_at IS NOT NULL;
