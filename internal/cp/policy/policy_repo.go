package policy

// ============================================================
// ASHRIX POLICY REPOSITORY — sqlc-generated queries + Go wrapper
// ============================================================
// File: internal/policy/repo.go
//
// This file wraps sqlc-generated queries with domain types and
// transaction orchestration. sqlc generates the raw SQL interface;
// this repo adds business logic (multi-table transactions, JSON
// marshaling, domain type conversion).
//
// Usage:
//   queries := db.New(dbpool)
//   repo := policy.NewRepo(dbpool, queries)
//   policy, err := repo.Create(ctx, orgID, req, actorID)
// ============================================================

import (
	// "bytes"
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
}

// -----------------------------------------------------------
// DOMAIN TYPES
// -----------------------------------------------------------

// type Effect string

// const (
// 	EffectAllow Effect = "ALLOW"
// 	EffectDeny  Effect = "DENY"
// )

// type Subject struct {
// 	Type  string `json:"type"`  // "group" or "user"
// 	Value string `json:"value"` // "engineering", "u-123"
// }

// type Resource struct {
// 	Type  string `json:"type"`  // "app", "path", "method"
// 	Value string `json:"value"` // "jenkins-prod", "/*", "GET"
// }

// type Conditions struct {
// 	MFA     *MFACondition     `json:"mfa,omitempty"`
// 	Device  *DeviceCondition  `json:"device,omitempty"`
// 	Network *NetworkCondition `json:"network,omitempty"`
// 	Time    *TimeCondition    `json:"time,omitempty"`
// }

// type MFACondition struct {
// 	Required bool   `json:"required"`
// 	MinLevel string `json:"min_level,omitempty"`
// }

// type DeviceCondition struct {
// 	Postures []string `json:"postures"`
// }

// type NetworkCondition struct {
// 	AllowedCountries []string `json:"allowed_countries,omitempty"`
// 	BlockedCountries []string `json:"blocked_countries,omitempty"`
// 	AllowedCIDRs     []string `json:"allowed_cidrs,omitempty"`
// 	BlockedCIDRs     []string `json:"blocked_cidrs,omitempty"`
// 	BlockTor         bool     `json:"block_tor,omitempty"`
// }

// type TimeCondition struct {
// 	ScheduleName string `json:"schedule_name"`
// }

// Policy is the domain model for a complete policy with all relations.
type Policy struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Name        string
	Description string
	Effect      Effect
	Priority    int32
	Enabled     bool
	Version     int64
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Subjects   []Subject
	Resources  []Resource
	Conditions Conditions
}

// Mutation represents a single entry from the policy mutation log.
type Mutation struct {
	Version   int64
	OrgID     uuid.UUID
	PolicyID  uuid.UUID
	Op        string // "UPSERT" or "DELETE"
	Snapshot  map[string]interface{}
	MutatedBy *uuid.UUID
	MutatedAt time.Time
}

// CreatePolicyRequest is the input for creating a new policy.
type CreatePolicyRequest struct {
	Name        string
	Description string
	Effect      Effect
	Priority    int32
	Subjects    []Subject
	Resources   []Resource
	Conditions  Conditions
}

// UpdatePolicyRequest is the input for updating an existing policy.
type UpdatePolicyRequest struct {
	Name        *string
	Description *string
	Effect      *Effect
	Priority    *int32
	Enabled     *bool
	Subjects    []Subject   // if non-empty, replaces all subjects
	Resources   []Resource  // if non-empty, replaces all resources
	Conditions  *Conditions // if non-nil, replaces conditions
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
// CREATE POLICY (multi-table transaction)
// -----------------------------------------------------------

func (r *postgresRepository) Create(ctx context.Context, orgID uuid.UUID, req CreatePolicyRequest, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) (*Policy, error) {
	policyID := uuid.New()
	now := time.Now().UTC()

	// Build denormalized snapshot for mutation log
	snapshot := map[string]interface{}{
		"id":          policyID,
		"org_id":      orgID,
		"name":        req.Name,
		"description": req.Description,
		"effect":      req.Effect,
		"priority":    req.Priority,
		"enabled":     true,
		"subjects":    req.Subjects,
		"resources":   req.Resources,
		"conditions":  req.Conditions,
	}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	conditionsBytes, err := json.Marshal(req.Conditions)
	if err != nil {
		return nil, fmt.Errorf("marshal conditions: %w", err)
	}

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)

	// 1. Insert mutation log (generates version)
	version, err := qtx.InsertPolicyMutation(ctx, store.InsertPolicyMutationParams{
		OrgID:        orgID,
		PolicyID:     policyID,
		Op:           "UPSERT",
		RuleSnapshot: snapshotBytes,
		MutatedBy:    pgtype.UUID{Bytes: actorID, Valid: true},
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
		Version:     version,
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
		Version:     version,
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
			ID:         row.ID,
			OrgID:      row.OrgID,
			Name:       row.Name,
			Description: row.Description.String,
			Effect:     Effect(row.Effect),
			Priority:   row.Priority.Int32,
			Enabled:    row.Enabled,
			Version:    row.Version,
			CreatedBy:  row.CreatedBy,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		}

		// Fetch subjects
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

		// Fetch resources
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

		// Fetch conditions
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
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}

	// Fetch subjects
	subjRows, err := r.queries.GetPolicySubjects(ctx, policyID)
	if err != nil {
		return nil, err
	}
	for _, s := range subjRows {
		policy.Subjects = append(policy.Subjects, Subject{Type: s.SubjectType, Value: s.SubjectValue})
	}

	// Fetch resources
	resRows, err := r.queries.GetPolicyResources(ctx, policyID)
	if err != nil {
		return nil, err
	}
	for _, res := range resRows {
		policy.Resources = append(policy.Resources, Resource{Type: res.ResourceType, Value: res.ResourceValue})
	}

	// Fetch conditions
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
// UPDATE POLICY (multi-table transaction)
// -----------------------------------------------------------

func (r *postgresRepository) Update(ctx context.Context, policyID, orgID uuid.UUID, req UpdatePolicyRequest, actorID uuid.UUID, actorEmail string, clientIP string, userAgent string) (*Policy, error) {
	// Get current state for audit log
	oldPolicy, err := r.GetByID(ctx, policyID, orgID)
	if err != nil {
		return nil, fmt.Errorf("get existing policy: %w", err)
	}

	oldStateBytes, _ := json.Marshal(oldPolicy)

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)

	// Build update params
	var namePtr, descPtr, effectPtr *string
	var priorityPtr *int32
	var enabledPtr *bool

	if req.Name != nil {
		namePtr = req.Name
	}
	if req.Description != nil {
		descPtr = req.Description
	}
	if req.Effect != nil {
		eff := string(*req.Effect)
		effectPtr = &eff
	}
	if *req.Priority != 0 || (*req.Priority == 0 && *req.Priority != oldPolicy.Priority) {
		priorityPtr = req.Priority
	}
	if req.Enabled != nil {
		enabledPtr = req.Enabled
	}

	// 1. Insert mutation log
	snapshot := map[string]interface{}{
		"id":          policyID,
		"org_id":      orgID,
		"name":        coalesceStr(req.Name, oldPolicy.Name),
		"description": coalesceStr(req.Description, oldPolicy.Description),
		"effect":      coalesceEffect(req.Effect, oldPolicy.Effect),
		"priority":    coalesceInt32(req.Priority, oldPolicy.Priority),
		"enabled":     coalesceBool(req.Enabled, oldPolicy.Enabled),
		"subjects":    coalesceSubjects(req.Subjects, oldPolicy.Subjects),
		"resources":   coalesceResources(req.Resources, oldPolicy.Resources),
		"conditions":  coalesceConditions(req.Conditions, oldPolicy.Conditions),
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	version, err := qtx.InsertPolicyMutation(ctx, store.InsertPolicyMutationParams{
		OrgID:        orgID,
		PolicyID:     policyID,
		Op:           "UPSERT",
		RuleSnapshot: snapshotBytes,
		MutatedBy:    pgtype.UUID{Bytes: actorID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("insert mutation: %w", err)
	}

	// 2. Update policy metadata
	_, err = qtx.UpdatePolicy(ctx, store.UpdatePolicyParams{
		ID:          policyID,
		Version:     version,
		OrgID:       pgtype.UUID{Bytes: orgID, Valid: true},
		Name:        pgtype.Text{String: *namePtr, Valid: namePtr != nil},
		Description: pgtype.Text{String: *descPtr, Valid: descPtr != nil},
		Effect:      pgtype.Text{String: *effectPtr, Valid: effectPtr != nil},
		Priority:    pgtype.Int4{Int32: *priorityPtr, Valid: priorityPtr != nil},
		Enabled:     pgtype.Bool{Bool: *enabledPtr, Valid: enabledPtr != nil},
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

	// Return updated policy
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

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)

	// 1. Mutation log
	version, err := qtx.InsertPolicyMutation(ctx, store.InsertPolicyMutationParams{
		OrgID:        orgID,
		PolicyID:     policyID,
		Op:           "DELETE",
		RuleSnapshot: oldStateBytes,
		MutatedBy:    pgtype.UUID{Bytes: actorID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert mutation: %w", err)
	}

	// 2. Delete subjects, resources, conditions (CASCADE handles this, but explicit is clearer)
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
		ActorID:   actorID.String(),
		ActorEmail: pgtype.Text{String: actorEmail, Valid: actorEmail != ""},
		OldState:   oldStateBytes,
		NewState:   nil,
		IpAddress: func() *netip.Addr {
			ip, _ := netip.ParseAddr(clientIP)
			return &ip
		}(),
		UserAgent:  pgtype.Text{String: userAgent, Valid: userAgent != ""},
	}); err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}

	_ = version // version is unused after insert, but returned for potential use
	return tx.Commit(ctx)
}

// -----------------------------------------------------------
// GET MUTATIONS SINCE (for delta computation)
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
		var snapshot map[string]interface{}
		if err := json.Unmarshal(row.RuleSnapshot, &snapshot); err != nil {
			return nil, fmt.Errorf("unmarshal snapshot v%d: %w", row.Version, err)
		}

		m := Mutation{
			Version:   row.Version,
			OrgID:     row.OrgID,
			PolicyID:  row.PolicyID,
			Op:        row.Op,
			Snapshot:  snapshot,
			MutatedAt: row.MutatedAt,
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

func coalesceSubjects(new []Subject, old []Subject) []Subject {
	if len(new) > 0 {
		return new
	}
	return old
}

func coalesceResources(new []Resource, old []Resource) []Resource {
	if len(new) > 0 {
		return new
	}
	return old
}

func coalesceConditions(new *Conditions, old Conditions) Conditions {
	if new != nil {
		return *new
	}
	return old
}
