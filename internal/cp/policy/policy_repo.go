package policy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store" // sqlc-generated package
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// -----------------------------------------------------------
// REPOSITORY INTERFACE
// -----------------------------------------------------------

type Repository interface {
	Create(ctx context.Context, orgID uuid.UUID, req CreatePolicyRequest, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) (*Policy, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Policy, error)
	GetByID(ctx context.Context, policyID, orgID uuid.UUID) (*Policy, error)
	Update(ctx context.Context, policyID uuid.UUID, orgID uuid.UUID, req UpdatePolicyRequest, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) (*Policy, error)
	Delete(ctx context.Context, policyID, orgID uuid.UUID, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) error
	GetMutationsSince(ctx context.Context, orgID uuid.UUID, sinceVersion int64) ([]Mutation, error)
	GetLatestVersion(ctx context.Context, orgID uuid.UUID) (int64, error)
	GetPolicyMutation(ctx context.Context, orgID, policyID uuid.UUID, sequence int64) (*Mutation, error)
}


// Mutation represents a single entry from the policy mutation log.
type Mutation struct {
	Version         int64
	Sequence        int64
	RecordTimestamp int64
	Signature       []byte
	OrgID           uuid.UUID
	PolicyID        uuid.UUID
	Op              string // "UPSERT" or "DELETE"
	Snapshot        map[string]interface{}
	MutatedBy       *uuid.UUID
	MutatedAt time.Time
}


// -----------------------------------------------------------
// REPOSITORY STRUCT
// -----------------------------------------------------------

type postgresRepository struct {
	db      *pgxpool.Pool
	queries *store.Queries
}

func NewRepository(dbConn *pgxpool.Pool, queries *store.Queries) Repository {
	return &postgresRepository{db: dbConn, queries: queries}
}


// -----------------------------------------------------------
// SNAPSHOT HELPERS (proto-aligned JSONB)
// -----------------------------------------------------------

func partitionSubjects(subjs []Subject) (users, groups []string) {
	for _, s := range subjs {
		switch s.Type {
		case "user":
			users = append(users, s.Value)
		case "group":
			groups = append(groups, s.Value)
		}
	}
	return
}

func partitionResources(res []Resource) (appIDs, paths, methods []string) {
	for _, r := range res {
		switch r.Type {
		case "app":
			appIDs = append(appIDs, r.Value)
		case "path":
			paths = append(paths, r.Value)
		case "method":
			methods = append(methods, r.Value)
		}
	}
	return
}

// buildSnapshotFromRequest builds a proto-aligned JSONB snapshot during Create.
func buildSnapshotFromRequest(policyID, orgID uuid.UUID, req CreatePolicyRequest, createdAt time.Time) map[string]interface{} {
	users, groups := partitionSubjects(req.Subjects)
	apps, paths, methods := partitionResources(req.Resources)

	return map[string]any{
		"policy_id":   policyID.String(),
		"tenant_id":   orgID.String(),
		"name":        req.Name,
		"description": req.Description,
		"effect":      string(req.Effect),
		"priority":    req.Priority,
		"subject": map[string]any{
			"users":  users,
			"groups": groups,
		},
		"resource": map[string]any{
			"app_ids": apps,
			"paths":   paths,
			"methods": methods,
		},
		"conditions": req.Conditions,
		"enabled":    true,
		"created_at": createdAt.Format(time.RFC3339Nano),
	}
}

// buildSnapshotFromPolicy builds a proto-aligned JSONB snapshot during Update.
func buildSnapshotFromPolicy(p Policy) map[string]interface{} {
	users, groups := partitionSubjects(p.Subjects)
	apps, paths, methods := partitionResources(p.Resources)

	return map[string]any{
		"policy_id":   p.ID.String(),
		"tenant_id":   p.OrgID.String(),
		"name":        p.Name,
		"description": p.Description,
		"effect":      string(p.Effect),
		"priority":    p.Priority,
		"subject": map[string]any{
			"users":  users,
			"groups": groups,
		},
		"resource": map[string]any{
			"app_ids": apps,
			"paths":   paths,
			"methods": methods,
		},
		"conditions": p.Conditions,
		"enabled":    p.Enabled,
		"created_at": p.CreatedAt.Format(time.RFC3339Nano),
	}
}


// -----------------------------------------------------------
// CREATE POLICY
// -----------------------------------------------------------

func (r *postgresRepository) Create(ctx context.Context, orgID uuid.UUID, req CreatePolicyRequest, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) (*Policy, error) {
	policyID := uuid.New()
	now := time.Now().UTC()
	seq := int64(1) // first mutation for this policy
	ts := now.UnixMilli()

	snapshot := buildSnapshotFromRequest(policyID, orgID, req, now)
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	conditionsBytes, err := json.Marshal(req.Conditions)
	if err != nil {
		return nil, fmt.Errorf("marshal conditions: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)

	// 1. Insert mutation log (generates global version)
	mut, err := qtx.InsertPolicyMutation(ctx, store.InsertPolicyMutationParams{
		OrgID:           orgID,
		PolicyID:        policyID,
		Op:              "UPSERT",
		RuleSnapshot:    snapshotBytes,
		MutatedBy:       pgtype.UUID{Bytes: actorID, Valid: true},
		Sequence:        seq,
		Signature:       nil, // signed later by the bundler / CP
		RecordTimestamp: ts,
	})
	if err != nil {
		return nil, fmt.Errorf("insert mutation: %w", err)
	}

	// 2. Insert policy
	_, err = qtx.InsertPolicy(ctx, store.InsertPolicyParams{
		ID:          policyID,
		OrgID:       orgID,
		Name:        req.Name,
		Description: pgtype.Text{String: req.Description, Valid: true},
		Effect:      string(req.Effect),
		Priority:    pgtype.Int4{Int32: req.Priority, Valid: true},
		Enabled:     true,
		Version:     mut.Version,
		Sequence:    mut.Sequence,
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("insert policy: %w", err)
	}

	// 3. Insert subjects
	for _, subj := range req.Subjects {
		if err := qtx.InsertPolicySubject(ctx, store.InsertPolicySubjectParams{
			PolicyID:     policyID,
			SubjectType:  subj.Type,
			SubjectValue: subj.Value,
		}); err != nil {
			return nil, fmt.Errorf("insert subject: %w", err)
		}
	}

	// 4. Insert resources
	for _, res := range req.Resources {
		if err := qtx.InsertPolicyResource(ctx, store.InsertPolicyResourceParams{
			PolicyID:      policyID,
			ResourceType:  res.Type,
			ResourceValue: res.Value,
		}); err != nil {
			return nil, fmt.Errorf("insert resource: %w", err)
		}
	}

	// 5. Insert conditions
	if err := qtx.InsertPolicyCondition(ctx, store.InsertPolicyConditionParams{
		PolicyID:      policyID,
		ConditionTree: conditionsBytes,
	}); err != nil {
		return nil, fmt.Errorf("insert conditions: %w", err)
	}

	// 6. Audit log
	if err := qtx.InsertPolicyAuditLog(ctx, store.InsertPolicyAuditLogParams{
		OrgID:      orgID,
		PolicyID:   pgtype.Text{String: policyID.String(), Valid: true},
		Action:     "CREATE",
		ActorID:    actorID.String(),
		ActorEmail: pgtype.Text{String: actorEmail, Valid: true},
		OldState:   nil,
		NewState:   snapshotBytes,
		IpAddress: func() *netip.Addr {
			ip := netip.MustParseAddr(clientIP)
			return &ip
		}(),
		UserAgent: pgtype.Text{String: userAgent, Valid: true},
	}); err != nil {
		return nil, fmt.Errorf("insert audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Policy{
		ID:          policyID,
		OrgID:       orgID,
		Name:        req.Name,
		Description: req.Description,
		Effect:      req.Effect,
		Priority:    req.Priority,
		Enabled:     true,
		Version:     mut.Version,
		Sequence:    mut.Sequence,
		CreatedBy:   actorID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Subjects:    req.Subjects,
		Resources:   req.Resources,
		Conditions:  req.Conditions,
	}, nil
}

// -----------------------------------------------------------
// LIST POLICIES BY ORG
// -----------------------------------------------------------

func (r *postgresRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Policy, error) {
	rows, err := r.queries.ListPoliciesByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}

	policies := make([]Policy, 0, len(rows))
	for _, row := range rows {
		policy := Policy{
			ID:          row.ID,
			OrgID:       row.OrgID,
			Name:        row.Name,
			Description: row.Description.String,
			Effect:      Effect(row.Effect),
			Priority:    row.Priority.Int32,
			Enabled:     row.Enabled,
			Version:     row.Version,
			Sequence:    row.Sequence,
			CreatedBy:   row.CreatedBy,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}

		subjRows, err := r.queries.GetPolicySubjects(ctx, policy.ID)
		if err != nil {
			return nil, fmt.Errorf("get subjects for %s: %w", policy.ID, err)
		}
		for _, s := range subjRows {
			policy.Subjects = append(policy.Subjects, Subject{
				Type:  s.SubjectType,
				Value: s.SubjectValue,
			})
		}

		resRows, err := r.queries.GetPolicyResources(ctx, policy.ID)
		if err != nil {
			return nil, fmt.Errorf("get resources for %s: %w", policy.ID, err)
		}
		for _, res := range resRows {
			policy.Resources = append(policy.Resources, Resource{
				Type:  res.ResourceType,
				Value: res.ResourceValue,
			})
		}

		condRow, err := r.queries.GetPolicyCondition(ctx, policy.ID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("get conditions for %s: %w", policy.ID, err)
		}
		if err == nil {
			if err := json.Unmarshal(condRow.ConditionTree, &policy.Conditions); err != nil {
				return nil, fmt.Errorf("unmarshal conditions for %s: %w", policy.ID, err)
			}
		}

		policies = append(policies, policy)
	}

	return policies, nil
}

// -----------------------------------------------------------
// GET POLICY BY ID
// -----------------------------------------------------------

func (r *postgresRepository) GetByID(ctx context.Context, policyID, orgID uuid.UUID) (*Policy, error) {
	row, err := r.queries.GetPolicyByID(ctx, store.GetPolicyByIDParams{
		ID:    policyID,
		OrgID: orgID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("policy not found")
		}
		return nil, fmt.Errorf("get policy: %w", err)
	}

	policy := Policy{
		ID:          row.ID,
		OrgID:       row.OrgID,
		Name:        row.Name,
		Description: row.Description.String,
		Effect:      Effect(row.Effect),
		Priority:    row.Priority.Int32,
		Enabled:     row.Enabled,
		Version:     row.Version,
		Sequence:    row.Sequence,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}

	subjRows, err := r.queries.GetPolicySubjects(ctx, policyID)
	if err != nil {
		return nil, err
	}
	for _, s := range subjRows {
		policy.Subjects = append(policy.Subjects, Subject{Type: s.SubjectType, Value: s.SubjectValue})
	}

	resRows, err := r.queries.GetPolicyResources(ctx, policyID)
	if err != nil {
		return nil, err
	}
	for _, res := range resRows {
		policy.Resources = append(policy.Resources, Resource{Type: res.ResourceType, Value: res.ResourceValue})
	}

	condRow, err := r.queries.GetPolicyCondition(ctx, policyID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		json.Unmarshal(condRow.ConditionTree, &policy.Conditions)
	}

	return &policy, nil
}

// -----------------------------------------------------------
// UPDATE POLICY
// -----------------------------------------------------------

func (r *postgresRepository) Update(ctx context.Context, policyID, orgID uuid.UUID, req UpdatePolicyRequest, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) (*Policy, error) {
	oldPolicy, err := r.GetByID(ctx, policyID, orgID)
	if err != nil {
		return nil, fmt.Errorf("get existing policy: %w", err)
	}

	oldStateBytes, _ := json.Marshal(oldPolicy)

	// Compute next per-policy sequence
	nextSeq, err := r.queries.GetNextPolicySequence(ctx, store.GetNextPolicySequenceParams{
		OrgID:    orgID,
		PolicyID: policyID,
	})
	if err != nil {
		return nil, fmt.Errorf("get next sequence: %w", err)
	}
	ts := time.Now().UTC().UnixMilli()

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)

	// Build merged policy for snapshot
	merged := *oldPolicy
	if req.Name != nil {
		merged.Name = *req.Name
	}
	if req.Description != nil {
		merged.Description = *req.Description
	}
	if req.Effect != nil {
		merged.Effect = *req.Effect
	}
	if req.Priority != nil {
		merged.Priority = *req.Priority
	}
	if req.Enabled != nil {
		merged.Enabled = *req.Enabled
	}
	if len(req.Subjects) > 0 {
		merged.Subjects = req.Subjects
	}
	if len(req.Resources) > 0 {
		merged.Resources = req.Resources
	}
	if req.Conditions != nil {
		merged.Conditions = *req.Conditions
	}

	snapshot := buildSnapshotFromPolicy(merged)
	snapshotBytes, _ := json.Marshal(snapshot)

	// 1. Insert mutation log
	mut, err := qtx.InsertPolicyMutation(ctx, store.InsertPolicyMutationParams{
		OrgID:           orgID,
		PolicyID:        policyID,
		Op:              "UPSERT",
		RuleSnapshot:    snapshotBytes,
		MutatedBy:       pgtype.UUID{Bytes: actorID, Valid: true},
		Sequence:        int64(nextSeq),
		Signature:       nil,
		RecordTimestamp: ts,
	})
	if err != nil {
		return nil, fmt.Errorf("insert mutation: %w", err)
	}

	// 2. Update policy metadata
	_, err = qtx.UpdatePolicy(ctx, store.UpdatePolicyParams{
		ID:          policyID,
		Version:     mut.Version,
		Sequence:    mut.Sequence,
		OrgID:       pgtype.UUID{Bytes: orgID, Valid: true},
		Name:        pgtype.Text{String: coalesceStr(req.Name, oldPolicy.Name), Valid: req.Name != nil},
		Description: pgtype.Text{String: coalesceStr(req.Description, oldPolicy.Description), Valid: req.Description != nil},
		Effect:      pgtype.Text{String: string(coalesceEffect(req.Effect, oldPolicy.Effect)), Valid: req.Effect != nil},
		Priority:    pgtype.Int4{Int32: coalesceInt32(req.Priority, oldPolicy.Priority), Valid: req.Priority != nil},
		Enabled:     pgtype.Bool{Bool: coalesceBool(req.Enabled, oldPolicy.Enabled), Valid: req.Enabled != nil},
	})
	if err != nil {
		return nil, fmt.Errorf("update policy: %w", err)
	}

	// 3. Replace subjects if provided
	if len(req.Subjects) > 0 {
		if err := qtx.DeletePolicySubjects(ctx, policyID); err != nil {
			return nil, fmt.Errorf("delete old subjects: %w", err)
		}
		for _, subj := range req.Subjects {
			if err := qtx.InsertPolicySubject(ctx, store.InsertPolicySubjectParams{
				PolicyID:     policyID,
				SubjectType:  subj.Type,
				SubjectValue: subj.Value,
			}); err != nil {
				return nil, fmt.Errorf("insert subject: %w", err)
			}
		}
	}

	// 4. Replace resources if provided
	if len(req.Resources) > 0 {
		if err := qtx.DeletePolicyResources(ctx, policyID); err != nil {
			return nil, fmt.Errorf("delete old resources: %w", err)
		}
		for _, res := range req.Resources {
			if err := qtx.InsertPolicyResource(ctx, store.InsertPolicyResourceParams{
				PolicyID:      policyID,
				ResourceType:  res.Type,
				ResourceValue: res.Value,
			}); err != nil {
				return nil, fmt.Errorf("insert resource: %w", err)
			}
		}
	}

	// 5. Replace conditions if provided
	if req.Conditions != nil {
		condBytes, _ := json.Marshal(*req.Conditions)
		if err := qtx.InsertPolicyCondition(ctx, store.InsertPolicyConditionParams{
			PolicyID:      policyID,
			ConditionTree: condBytes,
		}); err != nil {
			return nil, fmt.Errorf("update conditions: %w", err)
		}
	}

	// 6. Audit log
	newStateBytes, _ := json.Marshal(snapshot)
	if err := qtx.InsertPolicyAuditLog(ctx, store.InsertPolicyAuditLogParams{
		OrgID:      orgID,
		PolicyID:   pgtype.Text{String: policyID.String(), Valid: true},
		Action:     "UPDATE",
		ActorID:    actorID.String(),
		ActorEmail: pgtype.Text{String: actorEmail, Valid: actorEmail != ""},
		OldState:   oldStateBytes,
		NewState:   newStateBytes,
		IpAddress: func() *netip.Addr {
			ip, _ := netip.ParseAddr(clientIP)
			return &ip
		}(),
		UserAgent: pgtype.Text{String: userAgent, Valid: userAgent != ""},
	}); err != nil {
		return nil, fmt.Errorf("insert audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.GetByID(ctx, policyID, orgID)
}

// -----------------------------------------------------------
// DELETE POLICY
// -----------------------------------------------------------

func (r *postgresRepository) Delete(ctx context.Context, policyID, orgID uuid.UUID, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) error {
	oldPolicy, err := r.GetByID(ctx, policyID, orgID)
	if err != nil {
		return fmt.Errorf("get existing policy: %w", err)
	}
	oldStateBytes, _ := json.Marshal(oldPolicy)

	// Compute next sequence for the DELETE mutation
	nextSeq, err := r.queries.GetNextPolicySequence(ctx, store.GetNextPolicySequenceParams{
		OrgID:    orgID,
		PolicyID: policyID,
	})
	if err != nil {
		return fmt.Errorf("get next sequence: %w", err)
	}
	ts := time.Now().UTC().UnixMilli()

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)

	// 1. Mutation log (DELETE op)
	_, err = qtx.InsertPolicyMutation(ctx, store.InsertPolicyMutationParams{
		OrgID:           orgID,
		PolicyID:        policyID,
		Op:              "DELETE",
		RuleSnapshot:    oldStateBytes,
		MutatedBy:       pgtype.UUID{Bytes: actorID, Valid: true},
		Sequence:        int64(nextSeq),
		Signature:       nil,
		RecordTimestamp: ts,
	})
	if err != nil {
		return fmt.Errorf("insert mutation: %w", err)
	}

	// 2. Delete normalized rows
	_ = qtx.DeletePolicySubjects(ctx, policyID)
	_ = qtx.DeletePolicyResources(ctx, policyID)
	_ = qtx.DeletePolicyCondition(ctx, policyID)

	// 3. Delete policy
	if err := qtx.DeletePolicy(ctx, store.DeletePolicyParams{
		ID:    policyID,
		OrgID: orgID,
	}); err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}

	// 4. Audit log
	if err := qtx.InsertPolicyAuditLog(ctx, store.InsertPolicyAuditLogParams{
		OrgID:      orgID,
		PolicyID:   pgtype.Text{String: policyID.String(), Valid: true},
		Action:     "DELETE",
		ActorID:    actorID.String(),
		ActorEmail: pgtype.Text{String: actorEmail, Valid: actorEmail != ""},
		OldState:   oldStateBytes,
		NewState:   nil,
		IpAddress: func() *netip.Addr {
			ip, _ := netip.ParseAddr(clientIP)
			return &ip
		}(),
		UserAgent: pgtype.Text{String: userAgent, Valid: userAgent != ""},
	}); err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}

	return tx.Commit(ctx)
}

// -----------------------------------------------------------
// GET MUTATIONS SINCE (for delta computation / bundling)
// -----------------------------------------------------------

func (r *postgresRepository) GetMutationsSince(ctx context.Context, orgID uuid.UUID, sinceVersion int64) ([]Mutation, error) {
	rows, err := r.queries.GetMutationsSince(ctx, store.GetMutationsSinceParams{
		OrgID:   orgID,
		Version: sinceVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("get mutations: %w", err)
	}

	mutations := make([]Mutation, 0, len(rows))
	for _, row := range rows {
		var snapshot map[string]any
		if err := json.Unmarshal(row.RuleSnapshot, &snapshot); err != nil {
			return nil, fmt.Errorf("unmarshal snapshot v%d: %w", row.Version, err)
		}

		m := Mutation{
			Version:         row.Version,
			Sequence:        row.Sequence,
			RecordTimestamp: row.RecordTimestamp,
			Signature:       row.Signature,
			OrgID:           row.OrgID,
			PolicyID:        row.PolicyID,
			Op:              row.Op,
			Snapshot:        snapshot,
			MutatedAt:       row.MutatedAt,
		}
		if row.MutatedBy.Valid {
			id := uuid.UUID(row.MutatedBy.Bytes)
			m.MutatedBy = &id
		}
		mutations = append(mutations, m)
	}

	return mutations, nil
}

// -----------------------------------------------------------
// GET LATEST VERSION
// -----------------------------------------------------------

func (r *postgresRepository) GetLatestVersion(ctx context.Context, orgID uuid.UUID) (int64, error) {
	version, err := r.queries.GetLatestPolicyVersion(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("get latest version: %w", err)
	}
	return version, nil
}


// -----------------------------------------------------------
// GET POLICY MUTATION (for signature caching / bundle building)
// -----------------------------------------------------------

func (r *postgresRepository) GetPolicyMutation(ctx context.Context, orgID, policyID uuid.UUID, sequence int64) (*Mutation, error) {
	row, err := r.queries.GetPolicyMutation(ctx, store.GetPolicyMutationParams{
		OrgID:    orgID,
		PolicyID: policyID,
		Sequence: sequence,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("mutation not found")
		}
		return nil, fmt.Errorf("get mutation: %w", err)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal(row.RuleSnapshot, &snapshot); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}

	m := &Mutation{
		Version:         row.Version,
		Sequence:        row.Sequence,
		RecordTimestamp: row.RecordTimestamp,
		Signature:       row.Signature,
		OrgID:           row.OrgID,
		PolicyID:        row.PolicyID,
		Op:              row.Op,
		Snapshot:        snapshot,
		MutatedAt:       row.MutatedAt,
	}
	if row.MutatedBy.Valid {
		id := uuid.UUID(row.MutatedBy.Bytes)
		m.MutatedBy = &id
	}
	return m, nil
}

// -----------------------------------------------------------
// HELPERS
// -----------------------------------------------------------

func coalesceStr(ptr *string, fallback string) string {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

func coalesceEffect(ptr *Effect, fallback Effect) Effect {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

func coalesceInt32(ptr *int32, fallback int32) int32 {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

func coalesceBool(ptr *bool, fallback bool) bool {
	if ptr != nil {
		return *ptr
	}
	return fallback
}
