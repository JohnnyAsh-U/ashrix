-- =================================================================
-- POLICIES
-- Evaluated in priority order (lowest first).
-- First matching policy wins — OR logic between policies.
-- =================================================================

-- name: CreatePolicy :one
INSERT INTO policies (app_id, org_id, name, priority)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPolicyByID :one
SELECT * FROM policies
WHERE id         = $1
  AND deleted_at IS NULL;

-- name: GetPolicyByIDAndOrg :one
SELECT * FROM policies
WHERE id         = $1
  AND org_id     = $2
  AND deleted_at IS NULL;

-- name: ListPoliciesByApp :many
-- Returns active policies in priority order.
-- Used by Gateway policy sync to build local eval bundle.
SELECT * FROM policies
WHERE app_id     = $1
  AND is_active  = true
  AND deleted_at IS NULL
ORDER BY priority ASC;

-- name: ListAllPoliciesByOrg :many
-- Used by dashboard to display all policies across all apps.
SELECT * FROM policies
WHERE org_id     = $1
  AND deleted_at IS NULL
ORDER BY app_id, priority ASC;

-- name: UpdatePolicy :one
UPDATE policies
SET name       = $3,
    priority   = $4,
    is_active  = $5,
    updated_at = now()
WHERE id         = $1
  AND org_id     = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: DeletePolicy :one
-- Soft delete. Rules are cascade-orphaned but retained for audit.
UPDATE policies
SET deleted_at = now(),
    updated_at = now()
WHERE id         = $1
  AND org_id     = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: CheckPriorityConflict :one
-- Call before CreatePolicy or UpdatePolicy to prevent duplicate priority.
SELECT EXISTS (
    SELECT 1 FROM policies
    WHERE app_id   = $1
      AND priority = $2
      AND id      != $3       -- exclude self on update (pass gen_random_uuid() for create)
      AND deleted_at IS NULL
) AS conflict;


-- =================================================================
-- POLICY RULES
-- Flat rules. AND logic implicit within a policy.
-- All rules in a policy must match for the policy to apply.
-- =================================================================

-- name: CreatePolicyRule :one
INSERT INTO policy_rules (policy_id, effect, rule_type, value)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPolicyRuleByID :one
SELECT * FROM policy_rules
WHERE id = $1;

-- name: ListRulesByPolicy :many
-- Core of the policy sync bundle. Called per policy.
SELECT * FROM policy_rules
WHERE policy_id = $1
ORDER BY created_at ASC;

-- name: ListRulesByPolicies :many
-- Batch fetch for policy sync — all rules for multiple policies at once.
-- Avoids N+1 when syncing a full app policy bundle.
SELECT * FROM policy_rules
WHERE policy_id = ANY($1::uuid[])
ORDER BY policy_id, created_at ASC;

-- name: DeletePolicyRule :one
DELETE FROM policy_rules
WHERE id        = $1
  AND policy_id = $2
RETURNING *;

-- name: DeleteAllRulesForPolicy :exec
-- Called before rebuilding a policy's rules on full update.
DELETE FROM policy_rules
WHERE policy_id = $1;
