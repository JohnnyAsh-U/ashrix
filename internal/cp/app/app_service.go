package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/events"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo          Repository
	eventRepo     events.Repository
	connectorRepo connector.Repository
	dispatcher    *events.GatewayDispatcher
}

func NewService(repo Repository, eventRepo events.Repository, connectorRepo connector.Repository, dispatcher *events.GatewayDispatcher) *Service {
	return &Service{repo: repo, eventRepo: eventRepo, connectorRepo: connectorRepo, dispatcher: dispatcher}
}

func (s *Service) CreateApp(ctx context.Context, orgID uuid.UUID, req CreateAppRequest) (AppResponse, *dto.AppError) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.SockPass), 12)

	checkHealth := true
	if req.CheckHealth != nil {
		checkHealth = *req.CheckHealth
	}
	checkInterval := int32(60)
	if req.CheckInterval != nil && *req.CheckInterval > 0 {
		checkInterval = *req.CheckInterval
	}
	healthEndpoint := pgtype.Text{String: req.HealthEndpoint, Valid: req.HealthEndpoint != ""}

	params := store.CreateAppParams{
		OrgID:          orgID,
		Name:           req.Name,
		Subdomain:      req.Subdomain,
		Upstream:       req.Upstream,
		Protocol:       req.Protocol,
		IsPublic:       *req.IsPublic,
		SockPass:       string(hash),
		CheckHealth:    checkHealth,
		CheckInterval:  checkInterval,
		HealthEndpoint: healthEndpoint,
	}

	if req.ConnectorID != nil {
		params.ConnectorID = pgtype.UUID{Bytes: *req.ConnectorID, Valid: true}
	}

	app, err := s.repo.Create(ctx, params)
	if err != nil {
		return AppResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to create app", err.Error())
	}
	s.DispatchReloadConnectorCmd(ctx, app.ConnectorID.String())
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

	checkHealth := app.CheckHealth
	if req.CheckHealth != nil {
		checkHealth = *req.CheckHealth
	}
	checkInterval := app.CheckInterval
	if req.CheckInterval != nil && *req.CheckInterval > 0 {
		checkInterval = *req.CheckInterval
	}
	healthEndpoint := app.HealthEndpoint
	if req.HealthEndpoint != "" {
		healthEndpoint = pgtype.Text{String: req.HealthEndpoint, Valid: true}
	}

	params := store.UpdateAppParams{
		ID:             id,
		OrgID:          orgID,
		Name:           req.Name,
		Subdomain:      req.Subdomain,
		Upstream:       req.Upstream,
		Protocol:       req.Protocol,
		IsPublic:       *req.IsPublic,
		SockPass:       string(sockPass),
		CheckHealth:    checkHealth,
		CheckInterval:  checkInterval,
		HealthEndpoint: healthEndpoint,
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
	s.DispatchReloadConnectorCmd(ctx, app.ConnectorID.String())
	return mapToAppResponse(updatedApp), nil
}

func (s *Service) ListAppsByOrg(ctx context.Context, orgID uuid.UUID) ([]AppResponse, *dto.AppError) {
	apps, err := s.repo.ListAppsWithDetailsByOrg(ctx, orgID)
	if err != nil {
		return nil, dto.NewAppError(500, dto.CodeInternal, "Failed to list apps", err.Error())
	}

	var res []AppResponse
	for _, app := range apps {
		res = append(res, s.mapToAppDetailResponse(ctx, app))
	}
	if res == nil {
		res = []AppResponse{}
	}
	return res, nil
}

func (s *Service) mapToAppDetailResponse(ctx context.Context, app store.ListAppsWithDetailsByOrgRow) AppResponse {
	resp := AppResponse{
		ID:             app.ID.String(),
		OrgID:          app.OrgID.String(),
		Name:           app.Name,
		Subdomain:      app.Subdomain,
		Upstream:       app.Upstream,
		Protocol:       app.Protocol,
		IsPublic:       app.IsPublic,
		SockPass:       app.SockPass,
		CheckHealth:    app.CheckHealth,
		CheckInterval:  app.CheckInterval,
		HealthEndpoint: app.HealthEndpoint.String,
		HealthStatus:   app.HealthStatus,
		CreatedAt:      app.CreatedAt,
	}
	if app.ConnectorID.Valid {
		connIDStr := uuid.UUID(app.ConnectorID.Bytes).String()
		resp.ConnectorID = &connIDStr
	}
	if app.ConnectorName.Valid {
		resp.ConnectorName = app.ConnectorName.String
	}
	if app.GatewayName.Valid {
		resp.GatewayName = app.GatewayName.String
	}
	if app.LastSeen.Valid {
		resp.LastSeen = &app.LastSeen.Time
	}

	trafficCount, _ := s.repo.CountAppTrafficToday(ctx, pgtype.UUID{Bytes: app.ID, Valid: true})
	resp.NumberOfTrafficToday = trafficCount

	policies, _ := s.repo.ListPoliciesByAppResource(ctx, store.ListPoliciesByAppResourceParams{
		OrgID:           app.OrgID,
		ResourceValue:   app.ID.String(),
		ResourceValue_2: app.Subdomain,
	})
	policySummaries := make([]AppPolicySummary, 0, len(policies))
	for _, p := range policies {
		policySummaries = append(policySummaries, AppPolicySummary{
			ID:       p.ID.String(),
			Name:     p.Name,
			Effect:   p.Effect,
			Priority: p.Priority.Int32,
		})
	}
	resp.Policies = policySummaries

	usersCount, _ := s.repo.CountPolicyUserSubjectsByApp(ctx, store.CountPolicyUserSubjectsByAppParams{
		OrgID:           app.OrgID,
		ResourceValue:   app.ID.String(),
		ResourceValue_2: app.Subdomain,
	})
	groupsCount, _ := s.repo.CountPolicyGroupSubjectsByApp(ctx, store.CountPolicyGroupSubjectsByAppParams{
		OrgID:           app.OrgID,
		ResourceValue:   app.ID.String(),
		ResourceValue_2: app.Subdomain,
	})
	resp.NumberOfUsers = usersCount
	resp.NumberOfGroups = groupsCount

	return resp
}

func (s *Service) DeleteApp(ctx context.Context, id, orgID uuid.UUID) (AppResponse, *dto.AppError) {
	app, err := s.repo.Delete(ctx, store.DeleteAppParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppResponse{}, dto.NewNotFoundError("App not found")
		}
		return AppResponse{}, dto.NewAppError(500, dto.CodeInternal, "Failed to delete app", err.Error())
	}

	s.DispatchReloadConnectorCmd(ctx, app.ConnectorID.String())
	return mapToAppResponse(app), nil
}

func (s *Service) DispatchReloadConnectorCmd(ctx context.Context, connectorID string) {
	connectorUUID, err := uuid.Parse(connectorID)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(connectorUUID)
	connector, err := s.connectorRepo.GetConnectorByID(ctx, connectorUUID)
	if err != nil {
		fmt.Println(err.Error())
	}
	payload := events.CommandJob{
		Type:        events.CmdReloadConnector,
		GatewayID:   connector.GatewayID.String(),
		ConnectorID: connectorID,
	}
	_, err = s.eventRepo.CreateEvent(ctx, payload)
	if err != nil {
		fmt.Println(err.Error())
	}
	s.dispatcher.Wakeup(connector.GatewayID.String())
}

func mapToAppResponse(app store.App) AppResponse {
	resp := AppResponse{
		ID:             app.ID.String(),
		OrgID:          app.OrgID.String(),
		Name:           app.Name,
		Subdomain:      app.Subdomain,
		Upstream:       app.Upstream,
		Protocol:       app.Protocol,
		IsPublic:       app.IsPublic,
		SockPass:       app.SockPass,
		CheckHealth:    app.CheckHealth,
		CheckInterval:  app.CheckInterval,
		HealthEndpoint: app.HealthEndpoint.String,
		HealthStatus:   app.HealthStatus,
		Policies:       []AppPolicySummary{},
		CreatedAt:      app.CreatedAt,
	}
	if app.ConnectorID.Valid {
		connIDStr := uuid.UUID(app.ConnectorID.Bytes).String()
		resp.ConnectorID = &connIDStr
	}
	if app.LastSeen.Valid {
		resp.LastSeen = &app.LastSeen.Time
	}
	return resp
}
