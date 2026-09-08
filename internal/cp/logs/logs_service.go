package logs

import (
	"context"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type FilterAccessLogsRequest struct {
	UserEmail string `json:"user_email"`
	AppID     string `json:"app"`
	GatewayID string `json:"gateway"`
	Result    string `json:"result"`
	Page      int32  `json:"page"`
	Limit     int32  `json:"limit"`
}

type AccessLogResponse struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	GatewayID  string     `json:"gateway_id,omitempty"`
	AppID      string     `json:"app_id,omitempty"`
	PolicyID   string     `json:"policy_id,omitempty"`
	UserID     string     `json:"user_id,omitempty"`
	UserEmail  string     `json:"user_email,omitempty"`
	Method     string     `json:"method,omitempty"`
	Path       string     `json:"path,omitempty"`
	Status     int32      `json:"status,omitempty"`
	LatencyMs  int32      `json:"latency_ms,omitempty"`
	Action     string     `json:"action,omitempty"`
	IP         string     `json:"ip,omitempty"`
	Result     string     `json:"result"`
	DenyReason string     `json:"deny_reason,omitempty"`
	BytesIn    int64      `json:"bytes_in"`
	BytesOut   int64      `json:"bytes_out"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListAccessLogs(ctx context.Context, orgID uuid.UUID, req FilterAccessLogsRequest) ([]AccessLogResponse, *dto.AppError) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit < 1 || limit > 500 {
		limit = 50
	}
	offset := (page - 1) * limit

	params := store.ListAccessLogsFilteredParams{
		OrgID:  orgID,
		Limit:  limit,
		Offset: offset,
	}

	if req.UserEmail != "" {
		params.UserEmail = pgtype.Text{String: req.UserEmail, Valid: true}
	}
	if req.AppID != "" {
		if appUUID, err := uuid.Parse(req.AppID); err == nil {
			params.AppID = pgtype.UUID{Bytes: appUUID, Valid: true}
		}
	}
	if req.GatewayID != "" {
		if gwUUID, err := uuid.Parse(req.GatewayID); err == nil {
			params.GatewayID = pgtype.UUID{Bytes: gwUUID, Valid: true}
		}
	}
	if req.Result != "" {
		params.Result = pgtype.Text{String: req.Result, Valid: true}
	}

	logs, err := s.repo.ListAccessLogsFiltered(ctx, params)
	if err != nil {
		return nil, dto.NewAppError(500, dto.CodeInternal, "Failed to fetch access logs", err.Error())
	}

	res := make([]AccessLogResponse, 0, len(logs))
	for _, l := range logs {
		item := AccessLogResponse{
			ID:         l.ID.String(),
			OrgID:      l.OrgID.String(),
			UserID:     l.UserID.String,
			UserEmail:  l.UserEmail.String,
			Method:     l.Method.String,
			Path:       l.Path.String,
			Status:     l.Status.Int32,
			LatencyMs:  l.LatencyMs.Int32,
			Action:     l.Action.String,
			IP:         l.Ip.String,
			Result:     l.Result,
			DenyReason: l.DenyReason.String,
			BytesIn:    l.BytesIn,
			BytesOut:   l.BytesOut,
			CreatedAt:  l.CreatedAt,
		}
		if l.GatewayID.Valid {
			item.GatewayID = uuid.UUID(l.GatewayID.Bytes).String()
		}
		if l.AppID.Valid {
			item.AppID = uuid.UUID(l.AppID.Bytes).String()
		}
		if l.PolicyID.Valid {
			item.PolicyID = uuid.UUID(l.PolicyID.Bytes).String()
		}
		res = append(res, item)
	}

	return res, nil
}
