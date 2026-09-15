package policy

import (
	"context"
	"log/slog"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/google/uuid"
)

// =============================================================================
// SERVICE
// =============================================================================

type Service interface {
	Create(ctx context.Context, req CreatePolicyRequest) (*Policy, error)
	GetByID(ctx context.Context, policyID uuid.UUID) (*Policy, error)
	List(ctx context.Context, query ListPoliciesQuery) ([]Policy, int64, error)
	Update(ctx context.Context, policyID uuid.UUID, req UpdatePolicyRequest) (*Policy, error)
	Delete(ctx context.Context, policyID uuid.UUID) error
}

type policyService struct {
	repo Repository
	dist *PolicyDistributor
}

func NewService(repo Repository, dist *PolicyDistributor) Service {
	return &policyService{repo: repo, dist: dist}
}

func (s *policyService) Create(ctx context.Context, req CreatePolicyRequest) (*Policy, error) {
	orgID := middleware.OrgIDFromCtx(ctx)
	actorID := middleware.AdminIDFromCtx(ctx)
	actorEmail := middleware.EmailFromCtx(ctx)
	ua := middleware.UserAgent(ctx)
	ip := middleware.ClientIP(ctx)
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return nil, err
	}

	policy, err := s.repo.Create(ctx, orgUUID, req, actorUUID, actorEmail, ip, ua)
	if err != nil {
		return nil, err
	}

	// Distribute asynchronously so the HTTP response isn't blocked.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.dist.Distribute(ctx, orgUUID); err != nil {
			slog.Error("policy distribution failed", "org_id", orgID, "error", err)
		}
	}()

	return policy, nil
}

func (s *policyService) GetByID(ctx context.Context, policyID uuid.UUID) (*Policy, error) {
	orgID := middleware.OrgIDFromCtx(ctx)
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}
	policy, _, err := s.repo.GetByID(ctx, policyID, orgUUID)
	return policy, err
}

func (s *policyService) List(ctx context.Context, query ListPoliciesQuery) ([]Policy, int64, error) {
	orgID := middleware.OrgIDFromCtx(ctx)
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, 0, err
	}

	all, _, err := s.repo.ListByOrg(ctx, orgUUID)
	if err != nil {
		return nil, 0, err
	}

	// In-memory pagination (move to SQL LIMIT/OFFSET when orgs grow large).
	total := int64(len(all))
	start := (query.Page - 1) * query.Limit
	end := start + query.Limit
	if start > len(all) {
		return nil, total, nil
	}
	if end > len(all) {
		end = len(all)
	}

	return all[start:end], total, nil
}

func (s *policyService) Update(ctx context.Context, policyID uuid.UUID, req UpdatePolicyRequest) (*Policy, error) {
	orgID := middleware.OrgIDFromCtx(ctx)
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	actorID := middleware.AdminIDFromCtx(ctx)
	actorEmail := middleware.EmailFromCtx(ctx)
	clientIP := middleware.ClientIP(ctx)
	userAgent := middleware.UserAgent(ctx)

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return nil, err
	}

	policy, err := s.repo.Update(ctx, policyID, orgUUID, req, actorUUID, actorEmail, clientIP, userAgent)
	if err != nil {
		return nil, err
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.dist.Distribute(ctx, orgUUID); err != nil {
			slog.Error("policy distribution failed", "org_id", orgID, "error", err)
		}
	}()

	return policy, nil
}

func (s *policyService) Delete(ctx context.Context, policyID uuid.UUID) error {
	orgID := middleware.OrgIDFromCtx(ctx)
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return err
	}

	actorID := middleware.AdminIDFromCtx(ctx)
	actorEmail := middleware.EmailFromCtx(ctx)
	clientIP := middleware.ClientIP(ctx)
	userAgent := middleware.UserAgent(ctx)

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, policyID, orgUUID, actorUUID, actorEmail, clientIP, userAgent); err != nil {
		return err
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.dist.Distribute(ctx, orgUUID); err != nil {
			slog.Error("policy distribution failed", "org_id", orgID, "error", err)
		}
	}()

	return nil
}
