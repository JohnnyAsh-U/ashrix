package org

import (
	"context"
	"database/sql"
	"errors"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateOrg(ctx context.Context, name, slug string) (OrgResponse, *dto.AppError) {
	org, err := s.repo.Create(ctx, store.CreateOrgParams{Name: name, Slug: slug})
	if err != nil {
		return OrgResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create organization", err.Error())
	}
	return mapToOrgResponse(org), nil
}

func (s *Service) GetOrgByID(ctx context.Context, id uuid.UUID) (OrgResponse, *dto.AppError) {
	org, err := s.repo.GetByID(ctx, id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrgResponse{}, dto.NewNotFoundError("Organization not found")
		}
		return OrgResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to get organization", err.Error())
	}
	return mapToOrgResponse(org), nil
}

func (s *Service) GetOrgBySlug(ctx context.Context, slug string) (OrgResponse, *dto.AppError) {
	org, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrgResponse{}, dto.NewNotFoundError("Organization not found")
		}
		return OrgResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to get organization", err.Error())
	}
	return mapToOrgResponse(org), nil
}

func (s *Service) UpdateOrgName(ctx context.Context, id uuid.UUID, name string) (OrgResponse, *dto.AppError) {
	org, err := s.repo.UpdateName(ctx, store.UpdateOrgNameParams{ID: id, Name: name})
	if err != nil {
		return OrgResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to update organization name", err.Error())
	}
	return mapToOrgResponse(org), nil
}

func (s *Service) SetOrgCustomDomain(ctx context.Context, id uuid.UUID, customDomain string) (OrgResponse, *dto.AppError) {
	// Generate a domain verification token, a random 6-character string, and store it in the database along with the custom domain.
	domainToken := utils.GenerateRandomString(6)
	org, err := s.repo.SetCustomDomain(ctx, store.SetOrgCustomDomainParams{ID: id, CustomDomain: pgtype.Text{String: customDomain, Valid: true}, DomainVerificationToken: pgtype.Text{String: domainToken, Valid: true}})
	if err != nil {
		return OrgResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to set organization custom domain", err.Error())
	}
	return mapToOrgResponse(org), nil
}

func (s *Service) VerifyOrgCustomDomain(ctx context.Context, id uuid.UUID, domainToken string) (OrgResponse, *dto.AppError) {
	org, err := s.repo.VerifyCustomDomain(ctx, store.VerifyOrgCustomDomainParams{ID: id, DomainVerificationToken: pgtype.Text{String: domainToken, Valid: true}})
	if err != nil {
		return OrgResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to verify organization custom domain", err.Error())
	}
	return mapToOrgResponse(org), nil
}

func (s *Service) DeleteOrg(ctx context.Context, id uuid.UUID) (OrgResponse, *dto.AppError) {
	org, err := s.repo.Delete(ctx, id)
	if err != nil {
		return OrgResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to delete organization", err.Error())
	}
	return mapToOrgResponse(org), nil
}

func mapToOrgResponse(org store.Org) OrgResponse {
	return OrgResponse{
		ID:        org.ID.String(),
		Name:      org.Name,
		Slug:      org.Slug,
		CustomDomain: org.CustomDomain.String,
		CreatedAt: org.CreatedAt,
	}
}