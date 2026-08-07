-- name: InsertPolicyMutation :one
INSERT INTO policy_mutations (
    org_id, policy_id, op, rule_snapshot, mutated_by, sequence, signature, record_timestamp
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING version, sequence;


-- name: UpdateMutationSignature :exec
UPDATE policy_mutations
SET signature = $1
WHERE org_id = $2 AND policy_id = $3 AND sequence = $4;


-- name: GetPolicyMutation :one
SELECT 
    version,
    org_id,
    policy_id,
    op,
    sequence,
    record_timestamp,
    signature,
    rule_snapshot,
    mutated_by,
    mutated_at
FROM policy_mutations
WHERE org_id = $1 AND policy_id = $2 AND sequence = $3; 


-- name: GetMutationsSince :many
SELECT 
    version,
    org_id,
    policy_id,
    op,
    sequence,
    record_timestamp,
    signature,
    rule_snapshot,
    mutated_by,
    mutated_at
FROM policy_mutations
WHERE org_id = $1 AND version > $2
ORDER BY version ASC;


-- name: GetLatestPolicyVersion :one
SELECT COALESCE(MAX(version), 0)::bigint AS version
FROM policy_mutations
WHERE org_id = $1;

-- name: GetNextPolicySequence :one
SELECT COALESCE(MAX(sequence), 0) + 1 AS next_sequence
FROM policy_mutations
WHERE org_id = $1 AND policy_id = $2;



-- =============================================================================
-- Normalized Policy CRUD
-- =============================================================================


-- name: InsertPolicy :one
INSERT INTO policies (
    id, org_id, name, description, effect, priority, enabled, version, sequence, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;


-- name: UpdatePolicy :one
UPDATE policies SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    effect = COALESCE(sqlc.narg('effect'), effect),
    priority = COALESCE(sqlc.narg('priority'), priority),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    version = $2,
    sequence = $3,
    updated_at = NOW()
WHERE id = $1 AND org_id = sqlc.narg('org_id')
RETURNING *;


-- name: DeletePolicy :exec
DELETE FROM policies WHERE id = $1 AND org_id = $2;

-- name: GetPolicyByID :one
SELECT 
    p.id, p.org_id, p.name, p.description, p.effect, p.priority, 
    p.enabled, p.version, p.sequence, p.created_by, p.created_at, p.updated_at
FROM policies p
WHERE p.id = $1 AND p.org_id = $2;


-- name: ListPoliciesByOrg :many
SELECT 
    p.id, p.org_id, p.name, p.description, p.effect, p.priority, 
    p.enabled, p.version, p.sequence, p.created_by, p.created_at, p.updated_at
FROM policies p
WHERE p.org_id = $1
ORDER BY p.effect DESC, p.priority DESC, p.id;




-- name: GetPolicyWithDetails :one
SELECT 
    p.id, p.org_id, p.name, p.description, p.effect, p.priority, 
    p.enabled, p.version, p.sequence, p.created_by, p.created_at, p.updated_at,
    COALESCE(jsonb_agg(DISTINCT jsonb_build_object('type', ps.subject_type, 'value', ps.subject_value)) FILTER (WHERE ps.policy_id IS NOT NULL), '[]') AS subjects,
    COALESCE(jsonb_agg(DISTINCT jsonb_build_object('type', pr.resource_type, 'value', pr.resource_value)) FILTER (WHERE pr.policy_id IS NOT NULL), '[]') AS resources,
    pc.condition_tree as conditions
FROM policies p
LEFT JOIN policy_subjects ps ON ps.policy_id = p.id
LEFT JOIN policy_resources pr ON pr.policy_id = p.id
LEFT JOIN policy_conditions pc ON pc.policy_id = p.id
WHERE p.id = $1 AND p.org_id = $2
GROUP BY p.id, pc.condition_tree;



-- =============================================================================
-- Subjects, Resources, Conditions
-- =============================================================================

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



-- =============================================================================
-- Snapshot Builder (for mutation log)
-- =============================================================================


-- name: BuildPolicyRuleSnapshot :one
SELECT 
    jsonb_build_object(
        'policy_id', p.id::text,
        'tenant_id', p.org_id::text,
        'name', p.name,
        'description', p.description,
        'effect', p.effect,
        'priority', p.priority,
        'subject', jsonb_build_object(
            'users', COALESCE(array_agg(DISTINCT ps.subject_value) FILTER (WHERE ps.subject_type = 'user'), ARRAY[]::text[]),
            'groups', COALESCE(array_agg(DISTINCT ps.subject_value) FILTER (WHERE ps.subject_type = 'group'), ARRAY[]::text[])
        ),
        'resource', jsonb_build_object(
            'app_ids', COALESCE(array_agg(DISTINCT pr.resource_value) FILTER (WHERE pr.resource_type = 'app'), ARRAY[]::text[]),
            'paths', COALESCE(array_agg(DISTINCT pr.resource_value) FILTER (WHERE pr.resource_type = 'path'), ARRAY[]::text[]),
            'methods', COALESCE(array_agg(DISTINCT pr.resource_value) FILTER (WHERE pr.resource_type = 'method'), ARRAY[]::text[])
        ),
        'conditions', pc.condition_tree,
        'enabled', p.enabled,
        'version', p.version,
        'created_at', p.created_at
    )::jsonb as rule_snapshot
FROM policies p
LEFT JOIN policy_subjects ps ON ps.policy_id = p.id
LEFT JOIN policy_resources pr ON pr.policy_id = p.id
LEFT JOIN policy_conditions pc ON pc.policy_id = p.id
WHERE p.id = $1 AND p.org_id = $2
GROUP BY p.id, pc.condition_tree;

-- =============================================================================
-- Audit
-- =============================================================================

-- name: InsertPolicyAuditLog :exec
INSERT INTO policy_audit_log (
    org_id, policy_id, action, actor_id, actor_email, old_state, new_state, ip_address, user_agent
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);