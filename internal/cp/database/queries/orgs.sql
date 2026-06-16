-- =================================================================
-- ORGS
-- =================================================================

-- name: CreateOrg :one
INSERT INTO orgs (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetOrgByID :one
SELECT * FROM orgs
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetOrgBySlug :one
SELECT * FROM orgs
WHERE slug = $1
  AND deleted_at IS NULL;

-- name: GetOrgByCustomDomain :one
-- Used by Gateway to resolve incoming hostname to org (v2)
SELECT * FROM orgs
WHERE custom_domain = $1
  AND domain_verified = true
  AND deleted_at IS NULL;

-- name: UpdateOrgName :one
UPDATE orgs
SET name = $2
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: SetOrgCustomDomain :one
-- Sets custom domain and generates a verification token.
-- domain_verified stays false until DNS challenge passes.
UPDATE orgs
SET custom_domain               = $2,
    domain_verification_token   = $3,
    domain_verified             = false
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: VerifyOrgCustomDomain :one
-- Called after CP confirms the DNS TXT record is present.
UPDATE orgs
SET domain_verified = true
WHERE id = $1
  AND domain_verification_token = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteOrg :one
-- Soft delete only. Hard delete is a manual ops action.
UPDATE orgs
SET deleted_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;