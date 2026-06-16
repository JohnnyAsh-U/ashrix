-- =================================================================
-- ADMINS
-- =================================================================

-- name: GetAdminByIDPSubject :one
-- Used at OIDC callback. Matches on permanent IdP sub claim.
SELECT * FROM admins
WHERE org_id      = $1
  AND idp_subject = $2
  AND revoked_at  IS NULL;

-- name: ListAdminsByOrg :many
SELECT * FROM admins
WHERE org_id     = $1
  AND revoked_at IS NULL
ORDER BY created_at ASC;

-- name: BindAdminSSO :one
-- Called once during SSO binding confirmation flow.
-- Stores sub claim, links IdP config, marks sso_bound.
UPDATE admins
SET idp_subject   = $2,
    idp_config_id = $3,
    sso_bound     = true
WHERE id          = $1
  AND revoked_at  IS NULL
RETURNING *;

-- name: MigrateAdminToSSOOnly :one
-- Called after owner confirms SSO migration.
-- Clears password_hash — no going back without owner action.
UPDATE admins
SET password_hash = NULL,
    sso_only      = true
WHERE id          = $1
  AND sso_bound   = true
  AND revoked_at  IS NULL
RETURNING *;

-- name: UpdateAdminRole :one
-- Owner only action.
UPDATE admins
SET role = $2
WHERE id         = $1
  AND revoked_at IS NULL
RETURNING *;

-- name: RevokeAdmin :one
UPDATE admins
SET revoked_at = now()
WHERE id         = $1
  AND revoked_at IS NULL
RETURNING *;

-- name: CountOwnersByOrg :one
-- Used before revoking an owner — must always have at least one.
SELECT COUNT(*) FROM admins
WHERE org_id     = $1
  AND role       = 'owner'
  AND revoked_at IS NULL;
