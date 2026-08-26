package app

import (
	"context"
	"database/sql"
	"errors"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateApp(ctx context.Context, orgID uuid.UUID, req CreateAppRequest) (AppResponse, *dto.AppError) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.SockPass), 12)

	params := store.CreateAppParams{
		OrgID:     orgID,
		Name:      req.Name,
		Subdomain: req.Subdomain,
		Upstream:  req.Upstream,
		Protocol:  req.Protocol,
		IsPublic:  *req.IsPublic,
		SockPass:  string(hash),
	}

	if req.ConnectorID != nil {
		params.ConnectorID = pgtype.UUID{Bytes: *req.ConnectorID, Valid: true}
	}

	app, err := s.repo.Create(ctx, params)
	if err != nil {
		return AppResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create app", err.Error())
	}
	return mapToAppResponse(app), nil
}

func (s *Service) GetApp(ctx context.Context, id, orgID uuid.UUID) (AppResponse, *dto.AppError) {
	app, err := s.repo.GetByIDAndOrg(ctx, store.GetAppByIDAndOrgParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppResponse{}, dto.NewNotFoundError("App not found")
		}
		return AppResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to get app", err.Error())
	}
	return mapToAppResponse(app), nil
}

func (s *Service) UpdateApp(ctx context.Context, id, orgID uuid.UUID, req UpdateAppRequest) (AppResponse, *dto.AppError) {
	app, err := s.repo.GetByID(ctx, id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppResponse{}, dto.NewNotFoundError("App not found")
		}
		return AppResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to get app", err.Error())
	}

	var sockPass []byte
	sockPass = []byte(app.SockPass)

	if req.SockPass != "" {
		sockPass, _ = bcrypt.GenerateFromPassword([]byte(req.SockPass), 12)
	}

	params := store.UpdateAppParams{
		ID:        id,
		OrgID:     orgID,
		Name:      req.Name,
		Subdomain: req.Subdomain,
		Upstream:  req.Upstream,
		Protocol:  req.Protocol,
		IsPublic:  *req.IsPublic,
		SockPass:  string(sockPass),
	}

	if req.ConnectorID != nil {
		params.ConnectorID = pgtype.UUID{Bytes: *req.ConnectorID, Valid: true}
	}

	updatedApp, err := s.repo.Update(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppResponse{}, dto.NewNotFoundError("App not found")
		}
		return AppResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to update app", err.Error())
	}
	return mapToAppResponse(updatedApp), nil
}

func (s *Service) ListAppsByOrg(ctx context.Context, orgID uuid.UUID) ([]AppResponse, *dto.AppError) {
	apps, err := s.repo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, dto.NewAppError(500, dto.CodeInternal, "Failed to list apps", err.Error())
	}

	var res []AppResponse
	for _, app := range apps {
		res = append(res, mapToAppResponse(app))
	}
	if res == nil {
		res = []AppResponse{}
	}
	return res, nil
}

func (s *Service) DeleteApp(ctx context.Context, id, orgID uuid.UUID) (AppResponse, *dto.AppError) {
	app, err := s.repo.Delete(ctx, store.DeleteAppParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppResponse{}, dto.NewNotFoundError("App not found")
		}
		return AppResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to delete app", err.Error())
	}
	return mapToAppResponse(app), nil
}

func mapToAppResponse(app store.App) AppResponse {
	resp := AppResponse{
		ID:        app.ID.String(),
		OrgID:     app.OrgID.String(),
		Name:      app.Name,
		Subdomain: app.Subdomain,
		Upstream:  app.Upstream,
		Protocol:  app.Protocol,
		IsPublic:  app.IsPublic,
		SockPass:  app.SockPass,
		CreatedAt: app.CreatedAt,
	}
	if app.ConnectorID.Valid {
		connIDStr := uuid.UUID(app.ConnectorID.Bytes).String()
		resp.ConnectorID = &connIDStr
	}
	return resp
}
