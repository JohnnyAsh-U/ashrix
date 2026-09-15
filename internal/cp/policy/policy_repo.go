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
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Policy, []Mutation, error)
	GetByID(ctx context.Context, policyID, orgID uuid.UUID) (*Policy, json.RawMessage, error)
	Update(ctx context.Context, policyID uuid.UUID, orgID uuid.UUID, req UpdatePolicyRequest, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) (*Policy, error)
	Delete(ctx context.Context, policyID, orgID uuid.UUID, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) error
	GetMutationsSince(ctx context.Context, orgID uuid.UUID, sinceVersion int64) ([]Mutation, error)
	GetLatestPolicySequence(ctx context.Context, orgID uuid.UUID) (int64, error)
	GetPolicyMutation(ctx context.Context, orgID, policyID uuid.UUID, sequence int64) (*Mutation, error)
}

// Mutation represents a single entry from the policy mutation log.
type Mutation struct {
	Version          int64
	Sequence         int64
	RecordTimestamp  int64
	Signature        []byte
	OrgID            uuid.UUID
	PolicyID         uuid.UUID
	PolicyMutationID uuid.UUID
	Op               string // "UPSERT" or "DELETE"
	Snapshot         map[string]interface{}
	MutatedBy        *uuid.UUID
	MutatedAt        time.Time
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

func partitionSubjects(subjs []Subject) (users, groups, apps []string) {
	for _, s := range subjs {
		switch s.Type {
		case "user":
			users = append(users, s.Value)
		case "group":
			groups = append(groups, s.Value)
		case "app":
			apps = append(apps, s.Value)
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
func buildSnapshotFromRequest(policyID, policyMutationID, orgID uuid.UUID, req CreatePolicyRequest, createdAt time.Time) map[string]interface{} {
	users, groups, subjectApps := partitionSubjects(req.Subjects)
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
			"apps":   subjectApps,
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
	users, groups, apps := partitionSubjects(p.Subjects)
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
			"apps":   apps,
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
	policyMutationID := uuid.New()
	now := time.Now().UTC()
	version := int64(1) // first mutation for this policy
	ts := now.UnixMilli()

	// Compute next sequence
	nextSeq, err := r.queries.GetLatestPolicySequence(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get next sequence: %w", err)
	}

	snapshot := buildSnapshotFromRequest(policyID, policyMutationID, orgID, req, now)
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
		ID:              policyMutationID,
		OrgID:           orgID,
		PolicyID:        policyID,
		Op:              "UPSERT",
		RuleSnapshot:    snapshotBytes,
		MutatedBy:       pgtype.UUID{Bytes: actorID, Valid: true},
		MutatedAt:       time.Now(),
		Sequence:        nextSeq + 1,
		Version:         version,
		Signature:       []byte{}, // signed later by the bundler / CP
		RecordTimestamp: ts,
		IpAddress: func() *netip.Addr {
			ip, _ := netip.ParseAddr(clientIP)
			return &ip
		}(),
		UserAgent: pgtype.Text{String: userAgent, Valid: userAgent != ""},
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

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Policy{
		ID:               policyID,
		PolicyMutationID: policyMutationID,
		OrgID:            orgID,
		Name:             req.Name,
		Description:      req.Description,
		Effect:           req.Effect,
		Priority:         req.Priority,
		Enabled:          true,
		Version:          mut.Version,
		Sequence:         mut.Sequence,
		CreatedBy:        actorID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Subjects:         req.Subjects,
		Resources:        req.Resources,
		Conditions:       req.Conditions,
	}, nil
}

// -----------------------------------------------------------
// LIST POLICIES BY ORG
// -----------------------------------------------------------

func (r *postgresRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Policy, []Mutation, error) {
	rows, err := r.queries.ListPoliciesByOrg(ctx, orgID)
	if err != nil {
		return nil, nil, fmt.Errorf("list policies: %w", err)
	}

	policies := make([]Policy, 0, len(rows))
	mutations := make([]Mutation, 0, len(rows))
	for _, row := range rows {
		policy := Policy{
			ID:               row.ID,
			PolicyMutationID: row.MutationID,
			OrgID:            row.OrgID,
			Sequence:         row.Sequence,
			Name:             row.Name,
			Description:      row.Description.String,
			Effect:           Effect(row.Effect),
			Priority:         row.Priority.Int32,
			Enabled:          row.Enabled,
			Version:          row.Version,
			CreatedBy:        row.CreatedBy,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		}

		subjRows, err := r.queries.GetPolicySubjects(ctx, policy.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get subjects for %s: %w", policy.ID, err)
		}
		for _, s := range subjRows {
			policy.Subjects = append(policy.Subjects, Subject{
				Type: s.SubjectType,
				Value: func() string {
					if s.SubjectType == "app" {
						return s.AppName.String
					} else {
						return s.SubjectValue
					}
				}(),
			})
		}

		resRows, err := r.queries.GetPolicyResources(ctx, policy.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get resources for %s: %w", policy.ID, err)
		}
		for _, res := range resRows {
			policy.Resources = append(policy.Resources, Resource{
				Type: res.ResourceType,
				Value: func() string {
					if res.ResourceType == "app" {
						return res.AppName.String
					} else {
						return res.ResourceValue
					}
				}(),
			})
		}

		condRow, err := r.queries.GetPolicyCondition(ctx, policy.ID)
		if err != nil && err != sql.ErrNoRows {
			return nil, nil, fmt.Errorf("get conditions for %s: %w", policy.ID, err)
		}
		if err == nil {
			if err := json.Unmarshal(condRow.ConditionTree, &policy.Conditions); err != nil {
				return nil, nil, fmt.Errorf("unmarshal conditions for %s: %w", policy.ID, err)
			}
		}

		var snapshot map[string]any
		if err := json.Unmarshal(row.RuleSnapshot, &snapshot); err != nil {
			return nil, nil, fmt.Errorf("unmarshal snapshot v%d: %w", row.Version, err)
		}

		m := Mutation{
			Version:          row.Version,
			Sequence:         row.Sequence,
			RecordTimestamp:  row.RecordTimestamp,
			Signature:        row.Signature,
			OrgID:            row.OrgID,
			PolicyID:         row.ID,
			PolicyMutationID: row.MutationID,
			Op:               row.Op,
			Snapshot:         snapshot,
			MutatedAt:        row.MutatedAt,
		}
		if row.MutatedBy.Valid {
			id := uuid.UUID(row.MutatedBy.Bytes)
			m.MutatedBy = &id
		}
		mutations = append(mutations, m)

		policies = append(policies, policy)
	}

	return policies, mutations, nil
}

// -----------------------------------------------------------
// GET POLICY BY ID
// -----------------------------------------------------------

func (r *postgresRepository) GetByID(ctx context.Context, policyID, orgID uuid.UUID) (*Policy, json.RawMessage, error) {
	row, err := r.queries.GetPolicyByID(ctx, store.GetPolicyByIDParams{
		ID:    policyID,
		OrgID: orgID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("policy not found")
		}
		return nil, nil, fmt.Errorf("get policy: %w", err)
	}

	policy := Policy{
		ID:               row.ID,
		PolicyMutationID: row.MutationID,
		OrgID:            row.OrgID,
		Name:             row.Name,
		Description:      row.Description.String,
		Effect:           Effect(row.Effect),
		Priority:         row.Priority.Int32,
		Enabled:          row.Enabled,
		Sequence:         row.Sequence,
		Version:          row.Version,
		CreatedBy:        row.CreatedBy,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}

	subjRows, err := r.queries.GetPolicySubjects(ctx, policyID)
	if err != nil {
		return nil, nil, err
	}
	for _, s := range subjRows {
		policy.Subjects = append(policy.Subjects, Subject{Type: s.SubjectType, Value: s.SubjectValue})
	}

	resRows, err := r.queries.GetPolicyResources(ctx, policyID)
	if err != nil {
		return nil, nil, err
	}
	for _, res := range resRows {
		policy.Resources = append(policy.Resources, Resource{Type: res.ResourceType, Value: res.ResourceValue})
	}

	condRow, err := r.queries.GetPolicyCondition(ctx, policyID)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
	}
	if err == nil {
		json.Unmarshal(condRow.ConditionTree, &policy.Conditions)
	}

	return &policy, row.RuleSnapshot, nil
}

// -----------------------------------------------------------
// UPDATE POLICY
// -----------------------------------------------------------

func (r *postgresRepository) Update(ctx context.Context, policyID, orgID uuid.UUID, req UpdatePolicyRequest, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) (*Policy, error) {
	oldPolicy, snapShot, err := r.GetByID(ctx, policyID, orgID)
	if err != nil {
		return nil, fmt.Errorf("get existing policy: %w", err)
	}

	// Compute next per-policy version
	nextVersion, err := r.queries.GetNextPolicyVersion(ctx, store.GetNextPolicyVersionParams{
		OrgID:    orgID,
		PolicyID: policyID,
	})
	if err != nil {
		return nil, fmt.Errorf("get next sequence: %w", err)
	}

	// Compute next sequence
	nextSeq, err := r.queries.GetLatestPolicySequence(ctx, orgID)
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
		OldRuleSnapshot: snapShot,
		RuleSnapshot:    snapshotBytes,
		MutatedAt:       time.Now(),
		MutatedBy:       pgtype.UUID{Bytes: actorID, Valid: true},
		Sequence:        nextSeq + 1,
		Version:         int64(nextVersion),
		Signature:       []byte{},
		RecordTimestamp: ts,
		IpAddress: func() *netip.Addr {
			ip, _ := netip.ParseAddr(clientIP)
			return &ip
		}(),
		UserAgent: pgtype.Text{String: userAgent, Valid: userAgent != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("insert mutation: %w", err)
	}

	// 2. Update policy metadata
	_, err = qtx.UpdatePolicy(ctx, store.UpdatePolicyParams{
		ID:          policyID,
		Version:     mut.Version,
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

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	updatedPolicy, _, err := r.GetByID(ctx, policyID, orgID)
	if err != nil {
		return nil, fmt.Errorf("Error:%w", err)
	}
	return updatedPolicy, nil
}

// -----------------------------------------------------------
// DELETE POLICY
// -----------------------------------------------------------

func (r *postgresRepository) Delete(ctx context.Context, policyID, orgID uuid.UUID, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) error {
	policyMutationUUID := uuid.New()
	_, snapshot, err := r.GetByID(ctx, policyID, orgID)
	if err != nil {
		return fmt.Errorf("get existing policy: %w", err)
	}
	// oldStateBytes, _ := json.Marshal(oldPolicy)

	// Compute next sequence for the DELETE mutation
	nextSeq, err := r.queries.GetLatestPolicySequence(ctx, orgID)
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
		ID:              policyMutationUUID,
		OrgID:           orgID,
		PolicyID:        policyID,
		Op:              "DELETE",
		RuleSnapshot:    snapshot,
		MutatedBy:       pgtype.UUID{Bytes: actorID, Valid: true},
		MutatedAt:       time.Now(),
		Sequence:        nextSeq + 1,
		Signature:       []byte{},
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

	return tx.Commit(ctx)
}

// -----------------------------------------------------------
// GET MUTATIONS SINCE (for delta computation / bundling)
// -----------------------------------------------------------

func (r *postgresRepository) GetMutationsSince(ctx context.Context, orgID uuid.UUID, sinceVersion int64) ([]Mutation, error) {
	rows, err := r.queries.GetMutationsSince(ctx, store.GetMutationsSinceParams{
		OrgID:    orgID,
		Sequence: sinceVersion,
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
			Version:          row.Version,
			Sequence:         row.Sequence,
			RecordTimestamp:  row.RecordTimestamp,
			Signature:        row.Signature,
			OrgID:            row.OrgID,
			PolicyID:         row.PolicyID,
			PolicyMutationID: row.ID,
			Op:               row.Op,
			Snapshot:         snapshot,
			MutatedAt:        row.MutatedAt,
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

func (r *postgresRepository) GetLatestPolicySequence(ctx context.Context, orgID uuid.UUID) (int64, error) {
	version, err := r.queries.GetLatestPolicySequence(ctx, orgID)
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
		Version:          row.Version,
		Sequence:         row.Sequence,
		RecordTimestamp:  row.RecordTimestamp,
		Signature:        row.Signature,
		OrgID:            row.OrgID,
		PolicyID:         row.PolicyID,
		PolicyMutationID: row.ID,
		Op:               row.Op,
		Snapshot:         snapshot,
		MutatedAt:        row.MutatedAt,
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
