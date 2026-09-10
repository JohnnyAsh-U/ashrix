-- name: GetUserActiveSession :many
SELECT sessions.* FROM user_sessions as sessions
-- JOIN gateways ON sessions.gateway_id = gateways.id
WHERE org_id     = sqlc.arg('org_id')::uuid
  AND (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('gateway_id')::uuid IS NULL OR gateway_id = sqlc.narg('gateway_id')::uuid)
  AND (sqlc.narg('user_email')::text IS NULL OR user_email ILIKE '%' || sqlc.narg('user_email')::text || '%')
  AND expires_at > NOW()
  AND revoked_at IS NULL
ORDER BY issued_at DESC;


-- name: ListRevokedUserSessionByOrg :many
SELECT * FROM user_sessions
WHERE org_id = $1
  AND expires_at > NOW()
  AND revoked_at IS NOT NULL
ORDER BY issued_at DESC;


-- name: CreateUserSessionForGateway :one
INSERT INTO user_sessions (org_id, user_id, gateway_id, user_email, expires_at, issued_at)
VALUES ($1, $2, $3, $4, $5, NOW())
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

-- name: CountActiveUserSessionsByGateway :one
SELECT COUNT(*)::bigint AS active_count FROM user_sessions
WHERE gateway_id = $1
  AND (expires_at IS NULL OR expires_at > NOW())
  AND revoked_at IS NULL;
