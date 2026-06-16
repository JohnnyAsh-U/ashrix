package org

import (
	"fmt"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OrgHandler struct {
	service *Service
}

func NewOrgHandler(service *Service) *OrgHandler {
	return &OrgHandler{service: service}
}

// @Summary Create a new organization
// @Description Create a new organization with the provided name and slug.
// @Tags Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param org body CreateOrgRequest true "Organization details"
// @Success 201 {object} OrgResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs [post]
func (h *OrgHandler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	var req CreateOrgRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	svc, err := h.service.CreateOrg(r.Context(), req.Name, req.Slug)
	if err != nil {
		dto.SendError(w, err)
		return
	}
	dto.SendSuccess(w, http.StatusCreated, svc)
}

// @Summary Get organization by ID
// @Description Retrieve organization details by its unique ID.
// @Tags Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Organization ID"
// @Success 200 {object} OrgResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{id} [get]
func (h *OrgHandler) GetOrgByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	parsedID, parseErr := uuid.Parse(id)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid UUID format"))
		return
	}

	svc, err := h.service.GetOrgByID(r.Context(), parsedID)
	if err != nil {
		dto.SendError(w, err)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svc)
}

// @Summary Get organization by slug
// @Description Retrieve organization details by its unique slug.
// @Tags Organizations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param slug path string true "Organization Slug"
// @Success 200 {object} OrgResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/slug/{slug} [get]
func (h *OrgHandler) GetOrgBySlug(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetOrgBySlug")
	slug := chi.URLParam(r, "slug")
	svc, err := h.service.GetOrgBySlug(r.Context(), slug)
	if err != nil {
		dto.SendError(w, err)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svc)
}

// @Summary Update organization name
// @Description Update the name of an existing organization.
// @Tags Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Organization ID"
// @Param name body UpdateOrgRequest true "New organization name"
// @Success 200 {object} OrgResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{id}/name [put]
func (h *OrgHandler) UpdateOrgName(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	parsedID, parseErr := uuid.Parse(id)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid UUID format"))
		return
	}

	var req UpdateOrgRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}
	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}
	svc, err := h.service.UpdateOrgName(r.Context(), parsedID, req.Name)
	if err != nil {
		dto.SendError(w, err)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svc)
}

// @Summary Set organization custom domain
// @Description Set a custom domain for the organization.
// @Tags Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Organization ID"
// @Param customDomain body UpdateOrgCustomDomainRequest true "Custom domain details"
// @Success 200 {object} OrgResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{id}/custom-domain [put]
func (h *OrgHandler) SetOrgCustomDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	parsedID, parseErr := uuid.Parse(id)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid UUID format"))
		return
	}

	var req UpdateOrgCustomDomainRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}
	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}
	svc, err := h.service.SetOrgCustomDomain(r.Context(), parsedID, req.CustomDomain)
	if err != nil {
		dto.SendError(w, err)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svc)
}

// @Summary Verify organization custom domain
// @Description Verify the organization's custom domain using a domain token.
// @Tags Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Organization ID"
// @Param domainToken body DomainVerifyRequest true "Domain verification token"
// @Success 200 {object} OrgResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{id}/verify-domain [put]
func (h *OrgHandler) VerifyOrgCustomDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	parsedID, parseErr := uuid.Parse(id)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid UUID format"))
		return
	}

	var req DomainVerifyRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}
	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}
	svc, err := h.service.VerifyOrgCustomDomain(r.Context(), parsedID, req.DomainToken)
	if err != nil {
		dto.SendError(w, err)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svc)
}

// @Summary Delete organization
// @Description Delete an organization by its unique ID.
// @Tags Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Organization ID"
// @Success 200 {object} OrgResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /orgs/{id} [delete]
func (h *OrgHandler) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	parsedID, parseErr := uuid.Parse(id)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid UUID format"))
		return
	}

	svc, err := h.service.DeleteOrg(r.Context(), parsedID)
	if err != nil {
		dto.SendError(w, err)
		return
	}
	dto.SendSuccess(w, http.StatusOK, svc)
}

// Routes registers the organization-related routes to the provided router group.
func (h *OrgHandler) Routes(rg chi.Router) {
	rg.Post("/", h.CreateOrg)
	rg.Get("/slug/{slug}", h.GetOrgBySlug)
	rg.Get("/{id}", h.GetOrgByID)
	rg.Put("/{id}/name", h.UpdateOrgName)
	rg.Put("/{id}/custom-domain", h.SetOrgCustomDomain)
	rg.Put("/{id}/verify-domain", h.VerifyOrgCustomDomain)
	rg.Delete("/{id}", h.DeleteOrg)
}
