package logs

import (
	"net/http"
	"strconv"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type LogsHandler struct {
	service *Service
}

func NewLogsHandler(service *Service) *LogsHandler {
	return &LogsHandler{service: service}
}

// @Summary List Access Logs with Filters
// @Description Retrieve access logs filtered by user_email, app, gateway, result.
// @Tags Access Logs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_email query string false "Filter by user email"
// @Param app query string false "Filter by app ID"
// @Param gateway query string false "Filter by gateway ID"
// @Param result query string false "Filter by result (allowed, denied)"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Page size (default 50)"
// @Success 200 {array} AccessLogResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /logs/access [get]
func (h *LogsHandler) ListAccessLogs(w http.ResponseWriter, r *http.Request) {
	orgIDStr := middleware.OrgIDFromCtx(r.Context())
	orgID, parseErr := uuid.Parse(orgIDStr)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	q := r.URL.Query()
	userEmail := q.Get("user_email")
	appID := q.Get("app")
	gatewayID := q.Get("gateway")
	result := q.Get("result")

	var page int32 = 1
	if pStr := q.Get("page"); pStr != "" {
		if p, err := strconv.ParseInt(pStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	var limit int32 = 50
	if lStr := q.Get("limit"); lStr != "" {
		if l, err := strconv.ParseInt(lStr, 10, 32); err == nil && l > 0 {
			limit = int32(l)
		}
	}

	req := FilterAccessLogsRequest{
		UserEmail: userEmail,
		AppID:     appID,
		GatewayID: gatewayID,
		Result:    result,
		Page:      page,
		Limit:     limit,
	}

	logs, appErr := h.service.ListAccessLogs(r.Context(), orgID, req)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}

	dto.SendSuccess(w, http.StatusOK, logs)
}

func (h *LogsHandler) WithAuthRoutes(rg chi.Router) {
	rg.Get("/access", h.ListAccessLogs)
}
