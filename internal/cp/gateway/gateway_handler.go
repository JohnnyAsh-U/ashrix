package gateway

import (
	"fmt"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type GatewayHandler struct {
	service *Service
	signer  pki.CASigner
}

func NewGatewayHandler(service *Service, signer pki.CASigner) *GatewayHandler {
	return &GatewayHandler{service: service, signer: signer}
}

// @Summary Create a new gateway
// @Description Create a new gateway.
// @Tags Gateways
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param gateway body CreateGateway true "Gateway details"
// @Success 201 {object} GatewayResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /gateways [post]
func (h *GatewayHandler) CreateGateway(w http.ResponseWriter, r *http.Request) {
	var req CreateGateway
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	// Assuming OrgID is passed in the request body.
	// If OrgID is to be extracted from context (e.g., from JWT), this needs adjustment.
	gateway, appErr := h.service.CreateGateway(r.Context(), req.OrgID, req.Name, req.IPAddress, req.PublicURL)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusCreated, gateway)
}

// @Summary List gateways by organization
// @Description Retrieve a list of gateways for a admin organization.
// @Tags Gateways
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} GatewayResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /gateways [get]
func (h *GatewayHandler) ListGatewaysByOrg(w http.ResponseWriter, r *http.Request) {
	OrgID := middleware.OrgIDFromCtx(r.Context())

	orgID, parseErr := uuid.Parse(OrgID)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	gateways, appErr := h.service.ListGatewaysByOrg(r.Context(), orgID)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, gateways)
}

// @Summary Re-create a gateway
// @Description Re-create an existing gateway.
// @Tags Gateways
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Gateway ID"
// @Param reenrollment body ReEnrollGatewayRequest true "Gateway re-enrollment details"
// @Success 200 {object} GatewayResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /gateways/{id}/re-create [put]
func (h *GatewayHandler) ReCreateGateway(w http.ResponseWriter, r *http.Request) {
	gatewayIDStr := chi.URLParam(r, "id")
	gatewayID, parseErr := uuid.Parse(gatewayIDStr)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Gateway ID format"))
		return
	}

	var req ReEnrollGatewayRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	gateway, appErr := h.service.ReCreateGateway(r.Context(), gatewayID, req.Name, req.IPAddress, req.PublicURL)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, gateway)
}

// @Summary Enroll a gateway
// @Description Enroll a gateway using a token and CSR.
// @Tags Gateways
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param enrollment body gen.GatewayEnrollRequest true "Enrollment details"
// @Success 200 {object} gen.GatewayEnrollResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /internal/gateways/enroll [post]
func (h *GatewayHandler) EnrollGateway(w http.ResponseWriter, r *http.Request) {
	var req gen.GatewayEnrollRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(&req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	enrollmentResponse, appErr := h.service.EnrollGateway(r.Context(), req.Token, req.CsrPem, h.signer)
	if appErr != nil {
		fmt.Println("Error enrolling gateway:", appErr)
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, &enrollmentResponse)
}

// @Summary Renew gateway certificate
// @Description Renew a gateway certificate.
// @Tags Gateways
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param renewal body gen.GatewayRenewCertRequest true "Certificate renewal details"
// @Success 200 {object} gen.GatewayRenewCertResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /internal/gateways/renew [post]
func (h *GatewayHandler) RenewGatewayCert(w http.ResponseWriter, r *http.Request) {

	var req gen.GatewayRenewCertRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}
	if validationErrors := dto.ValidateStruct(&req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	gatewayID, parseErr := uuid.Parse(req.GatewayId)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Gateway ID format"))
		return
	}

	//timestamp to seconds
	timestampSeconds := req.Timestamp.AsTime().Unix()

	enrollmentResponse, appErr := h.service.RenewGatewayCert(r.Context(), gatewayID, req.Signature, req.CsrPem, timestampSeconds, h.signer)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, &enrollmentResponse)
}

// @Summary Revoke gateway certificate
// @Description Revoke a specific gateway certificate.
// @Tags Gateways
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Gateway ID"
// @Param revocation body RevokeGatewayCertRequest true "Certificate revocation details"
// @Success 200 {object} GatewayResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /gateways/{id}/certs/revoke [put]
func (h *GatewayHandler) RevokeGatewayCert(w http.ResponseWriter, r *http.Request) {
	gatewayIDStr := chi.URLParam(r, "id")
	gatewayID, parseErr := uuid.Parse(gatewayIDStr)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Gateway ID format"))
		return
	}

	var req RevokeGatewayCertRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	// Assuming componentID in RevokeGatewayCertRequest is the gatewayID.
	gateway, appErr := h.service.RevokeGatewayCert(r.Context(), gatewayID, req.ComponentType, req.RevokeReason)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, gateway)
}

// @Summary Revoke a gateway
// @Description Revoke an entire gateway.
// @Tags Gateways
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Gateway ID"
// @Param revocation body RevokeGatewayRequest true "Gateway revocation details"
// @Success 200 {object} GatewayResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /gateways/{id}/revoke [put]
func (h *GatewayHandler) RevokeGateway(w http.ResponseWriter, r *http.Request) {
	gatewayIDStr := chi.URLParam(r, "id")
	gatewayID, parseErr := uuid.Parse(gatewayIDStr)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Gateway ID format"))
		return
	}

	var req RevokeGatewayRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	gateway, appErr := h.service.RevokeGateway(r.Context(), gatewayID)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, gateway)
}

// Routes registers the gateway-related routes to the provided router group.
func (h *GatewayHandler) WithoutAuthRoutes(rg chi.Router) {
	rg.Use(middleware.InternalOnlyMiddleware)
	rg.Post("/enroll", h.EnrollGateway)
	rg.Post("/renew", h.RenewGatewayCert)
}

// Routes registers the gateway-related routes to the provided router group.
func (h *GatewayHandler) WithAuthRoutes(rg chi.Router) {
	rg.Post("/", h.CreateGateway)
	rg.Get("/", h.ListGatewaysByOrg)
	rg.Put("/{id}/re-create", h.ReCreateGateway)
	rg.Put("/{id}/certs/revoke", h.RevokeGatewayCert)
	rg.Put("/{id}/revoke", h.RevokeGateway)
}
