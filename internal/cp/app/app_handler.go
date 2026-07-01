package app

import (
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AppHandler struct {
	service *Service
}

func NewAppHandler(service *Service) *AppHandler {
	return &AppHandler{service: service}
}

// @Summary Create a new app
// @Description Create a new app in the specified organization.
// @Tags Apps
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param orgId path string true "Organization ID"
// @Param app body CreateAppRequest true "App details"
// @Success 201 {object} AppResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{orgId}/apps [post]
func (h *AppHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	var req CreateAppRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	svcResp, svcErr := h.service.CreateApp(r.Context(), orgID, req)
	if svcErr != nil {
		dto.SendError(w, svcErr)
		return
	}
	dto.SendSuccess(w, http.StatusCreated, svcResp)
}

// @Summary Update an app
// @Description Update the details of an existing app.
// @Tags Apps
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param orgId path string true "Organization ID"
// @Param id path string true "App ID"
// @Param app body UpdateAppRequest true "Updated app details"
// @Success 200 {object} AppResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{orgId}/apps/{id} [put]
func (h *AppHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid App ID format"))
		return
	}

	var req UpdateAppRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	svcResp, svcErr := h.service.UpdateApp(r.Context(), id, orgID, req)
	if svcErr != nil {
		dto.SendError(w, svcErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svcResp)
}

// @Summary List apps by organization
// @Description Retrieve a list of apps for a given organization.
// @Tags Apps
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param orgId path string true "Organization ID"
// @Success 200 {array} AppResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{orgId}/apps [get]
func (h *AppHandler) ListAppsByOrg(w http.ResponseWriter, r *http.Request) {
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	svcResp, svcErr := h.service.ListAppsByOrg(r.Context(), orgID)
	if svcErr != nil {
		dto.SendError(w, svcErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svcResp)
}

// @Summary Delete an app
// @Description Soft delete an app by its unique ID and organization ID.
// @Tags Apps
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param orgId path string true "Organization ID"
// @Param id path string true "App ID"
// @Success 200 {object} AppResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{orgId}/apps/{id} [delete]
func (h *AppHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid App ID format"))
		return
	}

	svcResp, svcErr := h.service.DeleteApp(r.Context(), id, orgID)
	if svcErr != nil {
		dto.SendError(w, svcErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svcResp)
}

// Routes registers the app-related routes to the provided router group.
func (h *AppHandler) Routes(rg chi.Router) {
	rg.Post("/", h.CreateApp)
	rg.Get("/", h.ListAppsByOrg)
	rg.Put("/{id}", h.UpdateApp)
	rg.Delete("/{id}", h.DeleteApp)
}
