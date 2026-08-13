-- name: GetUserActiveSession :many
SELECT * FROM user_sessions
WHERE org_id     = $1
  AND user_id    = $2
  AND expires_at > NOW()
  AND revoked_at IS NULL
ORDER BY issued_at DESC;


-- name: CreateUserSessionForGateway :one
INSERT INTO user_sessions (org_id, user_id, gateway_id, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RevokeActiveUserSession :one
UPDATE user_sessions
SET revoked_at = now()
WHERE id    = $1
  AND expires_at > NOW()
  AND revoked_at IS NULL
RETURNING *;

-- name: RevokeAllUserSessions :many
UPDATE user_sessions
SET revoked_at = now()
WHERE user_id    = $1
  AND org_id     = $2
  AND expires_at > NOW()
  AND revoked_at IS NULL
RETURNING *;

-- name: GetUserSessionByID :one
SELECT * FROM user_sessions
WHERE id = $1;
