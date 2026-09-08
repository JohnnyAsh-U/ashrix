package connector

import (
	// "net/http"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	// "github.com/google/uuid"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

type ConnectorHandler struct {
	service *Service
	signer  pki.CASigner
}

func NewConnectorHandler(service *Service, signer pki.CASigner) *ConnectorHandler {
	return &ConnectorHandler{service: service, signer: signer}
}

// @Summary Create a new connector
// @Description Create a new connector.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param connector body CreateConnector true "Connector details"
// @Success 201 {object} ConnectorResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /connectors [post]
func (h *ConnectorHandler) CreateConnector(w http.ResponseWriter, r *http.Request) {
	var req CreateConnector
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
	connector, appErr := h.service.CreateConnector(r.Context(), req.Name, req.GatewayID, req.SecondaryGatewayID, req.OpenSock)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusCreated, connector)
}

// @Summary List connectors by organization
// @Description Retrieve a list of connectors for a admin organization.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} ConnectorResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /connectors [get]
func (h *ConnectorHandler) ListConnectorssByOrg(w http.ResponseWriter, r *http.Request) {
	OrgID := middleware.OrgIDFromCtx(r.Context())

	orgID, parseErr := uuid.Parse(OrgID)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	gateways, appErr := h.service.ListConnectorsByOrg(r.Context(), orgID)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, gateways)
}

// @Summary Re-create a connectors
// @Description Re-create an existing connectors.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Connector ID"
// @Param reenrollment body ReEnrollConnectorRequest true "Connector re-enrollment details"
// @Success 200 {object} ConnectorResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /connectors/{id}/re-create [put]
func (h *ConnectorHandler) ReCreateConnector(w http.ResponseWriter, r *http.Request) {
	connID := chi.URLParam(r, "id")
	connIDParsed, parseErr := uuid.Parse(connID)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Gateway ID format"))
		return
	}

	var req ReEnrollConnectorRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	connector, appErr := h.service.ReCreateConnector(r.Context(), connIDParsed, req.Name, req.GatewayID, req.SecondaryGatewayID, req.OpenSock)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, connector)
}

// @Summary Enroll a connector
// @Description Enroll a connector using a token and CSR.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param enrollment body gen.ConnectorEnrollRequest true "Enrollment details"
// @Success 200 {object} gen.ConnectorEnrollResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /internal/connectors/enroll [post]
func (h *ConnectorHandler) EnrollConnector(w http.ResponseWriter, r *http.Request) {

	var req gen.ConnectorEnrollRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(&req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	enrollmentResponse, appErr := h.service.EnrollConnector(r.Context(), req.Token, req.CsrPem, h.signer)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, &enrollmentResponse)
}

// @Summary Get Connector Status
// @Description Get Connector Status.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Ashrix-Connector-ID header string true "Connector ID"
// @Param X-Ashrix-Timestamp header string true "Connector Timestamp"
// @Param X-Ashrix-Signature header string true "Connector Signature"
// @Param X-Ashrix-Nonce header string true "Connector Nonce"
// @Success 200 {object} gen.ConnectorStatusResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /connectors/status [get]
func (h *ConnectorHandler) ConnectorStatus(w http.ResponseWriter, r *http.Request) {

	connectorID := r.Header.Get("X-Ashrix-Connector-ID")
	timestampStr := r.Header.Get("X-Ashrix-Timestamp")
	signature := r.Header.Get("X-Ashrix-Signature")
	nonce := r.Header.Get("X-Ashrix-Nonce")

	if connectorID == "" || timestampStr == "" || signature == "" || nonce == "" {
		dto.SendError(w, &dto.AppError{Code: "400", Message: "Missing Auth header", Status: 400})
		return
	}

	//Convert ConnectorID to uuid
	connectorIDUUID, err := uuid.Parse(connectorID)

	if err != nil {
		dto.SendError(w, &dto.AppError{Code: "400", Message: "Bad ID", Status: 400})
		return
	}

	ts, err := strconv.ParseInt(timestampStr, 10, 64)

	if err != nil {
		dto.SendError(w, &dto.AppError{Code: "400", Message: "Bad Timestamp", Status: 400})
		return
	}

	now := time.Now().Unix()

	if math.Abs(float64(now-ts)) > 300 {
		dto.SendError(w, &dto.AppError{Code: "400", Message: "Expired Timestamp", Status: 400})
		return
	}

	canonicalString := fmt.Sprintf("%s\n%s\n%s\n%s\n%d", r.Method, r.URL.Path, connectorID, nonce, ts)

	connectorStatus, appErr := h.service.GetConnectorStatus(r.Context(), connectorIDUUID, canonicalString, signature)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}

	dto.SendSuccess(w, http.StatusOK, &connectorStatus)
}

// @Summary Renew connector certificate
// @Description Renew a connector certificate.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param renewal body gen.ConnectorRenewCertRequest true "Certificate renewal details"
// @Success 200 {object} gen.ConnectorRenewCertResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /internal/connectors/renew [post]
func (h *ConnectorHandler) RenewConnectorCert(w http.ResponseWriter, r *http.Request) {

	var req gen.ConnectorRenewCertRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}
	if validationErrors := dto.ValidateStruct(&req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	connID, parseErr := uuid.Parse(req.ConnectorId)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Conn ID format"))
		return
	}

	//Parse timestamp to int64
	// Convert it back to an int64 Unix timestamp
	timestampSeconds := req.Timestamp.AsTime().Unix()
	enrollmentResponse, appErr := h.service.RenewConnectorCert(r.Context(), connID, req.Signature, req.CsrPem, timestampSeconds, h.signer)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, &enrollmentResponse)
}

// @Summary Revoke connector certificate
// @Description Revoke a specific connector certificate.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Connector ID"
// @Param revocation body RevokeConnectorCertRequest true "Certificate revocation details"
// @Success 200 {object} ConnectorResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /connectors/{id}/certs/revoke [put]
func (h *ConnectorHandler) RevokeConnectorCert(w http.ResponseWriter, r *http.Request) {
	connectorIDStr := chi.URLParam(r, "id")
	connectorID, parseErr := uuid.Parse(connectorIDStr)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Gateway ID format"))
		return
	}

	var req RevokeConnectorCertRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	// Assuming componentID in RevokeConnectorCert is the connectorID.
	conn, appErr := h.service.RevokeConnectorCert(r.Context(), connectorID, req.RevokeReason)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, conn)
}

// @Summary Revoke a connector
// @Description Revoke an entire connector.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Gateway ID"
// @Param revocation body RevokeConnectorRequest true "Gateway revocation details"
// @Success 200 {object} ConnectorResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /connectors/{id}/revoke [put]
func (h *ConnectorHandler) RevokeConnector(w http.ResponseWriter, r *http.Request) {
	connectorIDStr := chi.URLParam(r, "id")
	connectorID, parseErr := uuid.Parse(connectorIDStr)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Gateway ID format"))
		return
	}

	var req RevokeConnectorRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	connector, appErr := h.service.RevokeConnector(r.Context(), connectorID)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, connector)
}


// RotateConnectorCert handles the request to rotate a connector's certificate.
// @Summary Rotate a connector's certificate
// @Description Rotate a connector's certificate.
// @Tags Connectors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Connector ID"
// @Success 200 {object} ConnectorResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /connectors/{id}/certs/rotate [put]
func (h *ConnectorHandler) SendRotateConnectorCmd(w http.ResponseWriter, r *http.Request) {
	connectorIDStr := chi.URLParam(r, "id")
	connectorID, parseErr := uuid.Parse(connectorIDStr)
	if parseErr != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Connector ID format"))
		return
	}
	
	connector, appErr := h.service.SendRotateConnectorCmd(r.Context(), connectorID)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, connector)
}

// Routes registers the gateway-related routes to the provided router group.
func (h *ConnectorHandler) WithoutAuthRoutes(rg chi.Router) {
	rg.Use(middleware.InternalOnlyMiddleware)
	rg.Post("/enroll", h.EnrollConnector)
	rg.Post("/renew", h.RenewConnectorCert)
	rg.Get("/status", h.ConnectorStatus)
}

// Routes registers the gateway-related routes to the provided router group.
func (h *ConnectorHandler) WithAuthRoutes(rg chi.Router) {
	rg.Post("/", h.CreateConnector)
	rg.Get("/", h.ListConnectorssByOrg)
	rg.Put("/{id}/re-create", h.ReCreateConnector)
	rg.Put("/{id}/certs/revoke", h.RevokeConnectorCert)
	rg.Put("/{id}/revoke", h.RevokeConnector)
	rg.Put("/{id}/certs/rotate", h.SendRotateConnectorCmd)
}
