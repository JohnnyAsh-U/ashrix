-- =================================================================
-- IDP CONFIGS
-- client_secret must be AES-256-GCM encrypted before insert.
-- Never store or return plaintext secrets.
-- =================================================================

-- name: CreateIDPConfig :one
INSERT INTO idp_configs (
  org_id, 
  name, 
  provider_type, 
  client_id, 
  client_secret, 
  issuer_url, 
  scopes, 
  email_claim,
  name_claim,
  group_claim,
  extra_config
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetIDPConfigByID :one
SELECT * FROM idp_configs
WHERE id         = $1
  AND deleted_at IS NULL;

-- name: GetIDPConfigByIDAndOrg :one
-- Always scope to org — never allow cross-org access.
SELECT * FROM idp_configs
WHERE id         = $1
  AND org_id     = $2
  AND deleted_at IS NULL;

-- name: ListIDPConfigsByOrg :many
SELECT * FROM idp_configs
WHERE org_id     = $1
  AND deleted_at IS NULL
ORDER BY created_at ASC;


-- name: MarkIDPConfigVerified :one
-- Called after owner completes SSO binding confirmation flow.
UPDATE idp_configs
SET is_verified = true
WHERE id        = $1
  AND org_id    = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateIDPConfig :one
-- Updating secrets rotates the encrypted value.
UPDATE idp_configs
SET name          = $3,
    client_id     = $4,
    client_secret = $5,
    issuer_url    = $6,
    is_active     = $7
WHERE id          = $1
  AND org_id      = $2
  AND deleted_at  IS NULL
RETURNING *;

-- name: DeleteIDPConfig :one
-- Soft delete. Cannot delete if it is the only verified active IdP
-- — enforce this check in the service layer before calling.
UPDATE idp_configs
SET deleted_at = now()
WHERE id        = $1
  AND org_id    = $2
  AND deleted_at IS NULL
RETURNING *;



-- name: AddAppIdpMapping :one
INSERT INTO app_idp_mappings (app_id, idp_id, is_required) 
VALUES ($1, $2, $3) 
RETURNING *;

-- name: DeleteAppIdpMapping :one
DELETE FROM app_idp_mappings
WHERE app_id = $1 AND idp_id = $2
RETURNING *;

-- name: ListAppIdPs :many
SELECT idp_configs.* 
FROM idp_configs 
JOIN app_idp_mappings ON idp_configs.id = app_idp_mappings.idp_id 
WHERE app_idp_mappings.app_id = $1 AND idp_configs.is_active = true;

