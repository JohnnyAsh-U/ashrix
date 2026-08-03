-- name: InsertPolicyMutation :one
INSERT INTO policy_mutations (org_id, policy_id, op, rule_snapshot, mutated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING version;

-- name: InsertPolicy :one
INSERT INTO policies (
    id, org_id, name, description, effect, priority, enabled, version, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: InsertPolicySubject :exec
INSERT INTO policy_subjects (policy_id, subject_type, subject_value)
VALUES ($1, $2, $3)
ON CONFLICT (policy_id, subject_type, subject_value) DO NOTHING;

-- name: InsertPolicyResource :exec
INSERT INTO policy_resources (policy_id, resource_type, resource_value)
VALUES ($1, $2, $3)
ON CONFLICT (policy_id, resource_type, resource_value) DO NOTHING;

-- name: InsertPolicyCondition :exec
INSERT INTO policy_conditions (policy_id, condition_tree)
VALUES ($1, $2)
ON CONFLICT (policy_id) DO UPDATE SET condition_tree = EXCLUDED.condition_tree, updated_at = NOW();

-- name: InsertPolicyAuditLog :exec
INSERT INTO policy_audit_log (
    org_id, policy_id, action, actor_id, actor_email, old_state, new_state, ip_address, user_agent
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetPolicyByID :one
SELECT 
    p.id, p.org_id, p.name, p.description, p.effect, p.priority, 
    p.enabled, p.version, p.created_by, p.created_at, p.updated_at
FROM policies p
WHERE p.id = $1 AND p.org_id = $2;

-- name: ListPoliciesByOrg :many
SELECT 
    p.id, p.org_id, p.name, p.description, p.effect, p.priority, 
    p.enabled, p.version, p.created_by, p.created_at, p.updated_at
FROM policies p
WHERE p.org_id = $1
ORDER BY p.effect DESC, p.priority DESC, p.id;

-- name: GetPolicySubjects :many
SELECT policy_id, subject_type, subject_value
FROM policy_subjects
WHERE policy_id = $1;

-- name: GetPolicyResources :many
SELECT policy_id, resource_type, resource_value
FROM policy_resources
WHERE policy_id = $1;

-- name: GetPolicyCondition :one
SELECT policy_id, condition_tree, updated_at
FROM policy_conditions
WHERE policy_id = $1;

-- name: DeletePolicySubjects :exec
DELETE FROM policy_subjects WHERE policy_id = $1;

-- name: DeletePolicyResources :exec
DELETE FROM policy_resources WHERE policy_id = $1;

-- name: DeletePolicyCondition :exec
DELETE FROM policy_conditions WHERE policy_id = $1;

-- name: DeletePolicy :exec
DELETE FROM policies WHERE id = $1 AND org_id = $2;

-- name: UpdatePolicy :one
UPDATE policies SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    effect = COALESCE(sqlc.narg('effect'), effect),
    priority = COALESCE(sqlc.narg('priority'), priority),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    version = $2,
    updated_at = NOW()
WHERE id = $1 AND org_id = sqlc.narg('org_id')
RETURNING *;

-- name: GetMutationsSince :many
SELECT version, org_id, policy_id, op, rule_snapshot, mutated_by, mutated_at
FROM policy_mutations
WHERE org_id = $1 AND version > $2
ORDER BY version ASC;

-- name: GetLatestPolicyVersion :one
SELECT COALESCE(MAX(version), 0)::bigint AS version
FROM policy_mutations
WHERE org_id = $1;