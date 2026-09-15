package policy

import (
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)


type PolicyHandler struct {
	service Service
}

func NewPolicyHandler(service Service) *PolicyHandler {
	return &PolicyHandler{service: service}
}

func mapToDomainSubjects(dtos []Subject) []Subject {
	out := make([]Subject, len(dtos))
	for i, s := range dtos {
		out[i] = Subject{Type: s.Type, Value: s.Value}
	}
	return out
}

func mapToDomainResources(dtos []Resource) []Resource {
	out := make([]Resource, len(dtos))
	for i, r := range dtos {
		out[i] = Resource{Type: r.Type, Value: r.Value}
	}
	return out
}

func mapToDomainConditions(dto Conditions) Conditions {
	c := Conditions{}
	if dto.MFA != nil {
		c.MFA = &MFACondition{Required: dto.MFA.Required, MinLevel: dto.MFA.MinLevel}
	}
	if dto.Device != nil {
		c.Device = &DeviceCondition{Postures: dto.Device.Postures}
	}
	if dto.Network != nil {
		c.Network = &NetworkCondition{
			AllowedCountries: dto.Network.AllowedCountries,
			BlockedCountries: dto.Network.BlockedCountries,
			AllowedCIDRs:     dto.Network.AllowedCIDRs,
			BlockedCIDRs:     dto.Network.BlockedCIDRs,
			BlockTor:         dto.Network.BlockTor,
		}
	}
	if dto.Time != nil {
		c.Time = &TimeCondition{ScheduleName: dto.Time.ScheduleName}
	}
	return c
}

func mapToResponse(p *Policy) Policy {
	subjects := make([]Subject, len(p.Subjects))
	for i, s := range p.Subjects {
		subjects[i] = Subject{Type: s.Type, Value: s.Value}
	}
	resources := make([]Resource, len(p.Resources))
	for i, r := range p.Resources {
		resources[i] = Resource{Type: r.Type, Value: r.Value}
	}

	conds := Conditions{}
	if p.Conditions.MFA != nil {
		conds.MFA = &MFACondition{Required: p.Conditions.MFA.Required, MinLevel: p.Conditions.MFA.MinLevel}
	}
	if p.Conditions.Device != nil {
		conds.Device = &DeviceCondition{Postures: p.Conditions.Device.Postures}
	}
	if p.Conditions.Network != nil {
		conds.Network = &NetworkCondition{
			AllowedCountries: p.Conditions.Network.AllowedCountries,
			BlockedCountries: p.Conditions.Network.BlockedCountries,
			AllowedCIDRs:     p.Conditions.Network.AllowedCIDRs,
			BlockedCIDRs:     p.Conditions.Network.BlockedCIDRs,
			BlockTor:         p.Conditions.Network.BlockTor,
		}
	}
	if p.Conditions.Time != nil {
		conds.Time = &TimeCondition{ScheduleName: p.Conditions.Time.ScheduleName}
	}

	return Policy{
		ID:          p.ID,
		OrgID:       p.OrgID,
		Name:        p.Name,
		Description: p.Description,
		Effect:      p.Effect,
		Priority:    p.Priority,
		Enabled:     p.Enabled,
		Version:     p.Version,
		Sequence:    p.Sequence,
		CreatedBy:   p.CreatedBy,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		Subjects:    subjects,
		Resources:   resources,
		Conditions:  conds,
	}
}

// CreatePolicy godoc
// @Summary Create a new policy
// @Description Create a new access policy for the organization.
// @Tags Policy
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param policy body CreatePolicyRequest true "Policy details"
// @Success 201 {object} Policy
// @Failure 400 {object} dto.AppError
// @Failure 401 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /policies [post]
func (h *PolicyHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req CreatePolicyRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		return
	}

	policy, err := h.service.Create(r.Context(), req)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError(err))
		return
	}

	dto.SendSuccess(w, http.StatusCreated, policy)
}

// GetPolicy godoc
// @Summary Get a policy by ID
// @Description Retrieve a single policy. Must belong to the caller's org.
// @Tags Policy
// @Produce json
// @Security BearerAuth
// @Param id path string true "Policy ID" format(uuid)
// @Success 200 {object} Policy
// @Failure 400 {object} dto.AppError
// @Failure 401 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Router /policies/{id} [get]
func (h *PolicyHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	parsedID, err := uuid.Parse(id)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("invalid_uuid"))
		return
	}

	policy, err := h.service.GetByID(r.Context(), parsedID)
	if err != nil {
		if err.Error() == "policy not found" {
			dto.SendError(w, dto.NewNotFoundError("not_found"))
			return
		}
		dto.SendError(w, dto.NewBadRequestError(err))
		return
	}

	dto.SendSuccess(w, http.StatusOK, policy)
}

// ListPolicies godoc
// @Summary List policies
// @Description List all policies for the caller's organization with pagination.
// @Tags Policy
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param sort_by query string false "Sort field" default(priority)
// @Param order query string false "Sort order" Enums(asc, desc) default(desc)
// @Success 200 {object} ListPoliciesResponse
// @Failure 401 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /policies [get]
func (h *PolicyHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	var query ListPoliciesQuery
	// if err := dto.DecodeJSON(w, r, &query); err != nil {
	// 	dto.SendError(w, dto.NewBadRequestError("invalid_query"))
	// 	return
	// }
	if query.Page == 0 {
		query.Page = 1
	}
	if query.Limit == 0 {
		query.Limit = 20
	}

	policies, total, err := h.service.List(r.Context(), query)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError(err))
		return
	}

	items := make([]Policy, len(policies))
	for i, p := range policies {
		items[i] = mapToResponse(&p)
	}

	dto.SendSuccess(w, http.StatusOK, ListPoliciesResponse{
		Items: items,
		Total: total,
		Page:  query.Page,
		Limit: query.Limit,
	})
}

// UpdatePolicy godoc
// @Summary Update a policy
// @Description Update an existing policy. Partial updates supported.
// @Tags Policy
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Policy ID" format(uuid)
// @Param policy body UpdatePolicyRequest true "Policy update"
// @Success 200 {object} Policy
// @Failure 400 {object} dto.AppError
// @Failure 401 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /policies/{id} [put]
func (h *PolicyHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("invalid_uuid"))
		return
	}

	var req UpdatePolicyRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, dto.NewBadRequestError("invalid_request"))
		return
	}

	domainReq := UpdatePolicyRequest{
		Name:        req.Name,
		Description: req.Description,
		Subjects:    mapToDomainSubjects(req.Subjects),
		Resources:   mapToDomainResources(req.Resources),
	}
	if req.Effect != nil {
		e := Effect(*req.Effect)
		domainReq.Effect = &e
	}
	if req.Priority != nil {
		domainReq.Priority = req.Priority
	}
	if req.Enabled != nil {
		domainReq.Enabled = req.Enabled
	}
	if req.Conditions != nil {
		c := mapToDomainConditions(*req.Conditions)
		domainReq.Conditions = &c
	}

	policy, err := h.service.Update(r.Context(), id, domainReq)
	if err != nil {
		if err.Error() == "policy not found" {
			dto.SendError(w, dto.NewNotFoundError("not_found"))
			return
		}
		dto.SendError(w, dto.NewBadRequestError(err))
		return
	}

	dto.SendSuccess(w, http.StatusOK, mapToResponse(policy))
}

// DeletePolicy godoc
// @Summary Delete a policy
// @Description Delete a policy and distribute the change to gateways.
// @Tags Policy
// @Produce json
// @Security BearerAuth
// @Param id path string true "Policy ID" format(uuid)
// @Success 204
// @Failure 400 {object} dto.AppError
// @Failure 401 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /policies/{id} [delete]
func (h *PolicyHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("invalid_uuid"))
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		if err.Error() == "policy not found" {
			dto.SendError(w, dto.NewNotFoundError("not_found"))
			return
		}
		dto.SendError(w, dto.NewBadRequestError(err))
		return
	}

	dto.SendSuccess(w, http.StatusNoContent, nil)
}

// GetPolicyHistory godoc
// @Summary Get policy history
// @Description Retrieve the mutation history for a policy.
// @Tags Policy
// @Produce json
// @Security BearerAuth
// @Param id path string true "Policy ID" format(uuid)
// @Success 200 {array} Mutation
// @Failure 400 {object} dto.AppError
// @Failure 401 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Router /policies/{id}/history [get]
func (h *PolicyHandler) GetPolicyHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	parsedID, err := uuid.Parse(id)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("invalid_uuid"))
		return
	}

	mutations, err := h.service.GetHistory(r.Context(), parsedID)
	if err != nil {
		if err.Error() == "policy not found" {
			dto.SendError(w, dto.NewNotFoundError("not_found"))
			return
		}
		dto.SendError(w, dto.NewBadRequestError(err))
		return
	}

	dto.SendSuccess(w, http.StatusOK, mutations)
}

// // =============================================================================
// // ROUTER REGISTRATION
// // =============================================================================

// Routes registers the organization-related routes to the provided router group.
func (h *PolicyHandler) Routes(rg chi.Router) {
	rg.Post("/", h.CreatePolicy)
	rg.Get("/", h.ListPolicies)
	rg.Get("/{id}", h.GetPolicy)
	rg.Get("/{id}/history", h.GetPolicyHistory)
	rg.Put("/{id}", h.UpdatePolicy)
	rg.Delete("/{id}", h.DeletePolicy)
}

