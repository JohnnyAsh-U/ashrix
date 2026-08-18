package identity

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/oidc"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type IDPHandler struct {
	idpService *IDPService
	log        *slog.Logger
}

func NewIDPHandler(idpService *IDPService, log *slog.Logger) *IDPHandler {
	return &IDPHandler{
		idpService: idpService,
		log:        log,
	}
}

// @Summary Resolve IDP for tenant and app
// @Description Resolve IDP for the specified tenant and app.
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param gid query string true "Gateway ID"
// @Param aid query string true "App ID"
// @Success 201 {object} APIIDPResolverResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /authorize/providers [get]
func (i *IDPHandler) IDPResolverHandler(w http.ResponseWriter, r *http.Request) {
	// /login?id=X&app=Y
	ctx := r.Context()
	gatewayID := r.URL.Query().Get("gid")
	appID := r.URL.Query().Get("aid")

	if appID == "" || gatewayID == "" {
		dto.SendError(w, dto.NewBadRequestError("Not Valid"))
		return
	}
	appUUID, err := uuid.Parse(appID)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Not Valid Client App"))
		return
	}

	gatewayUUID, err := uuid.Parse(gatewayID)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Not Valid Client App"))
		return
	}

	//Resolve the AppIDPProviders
	adapters, err := i.idpService.ResolveAppIDP(ctx, appUUID, gatewayUUID)
	if err != nil {
		dto.SendError(w, dto.NewNotFoundError("Not Found"))
		return
	}
	adapterResponse := make([]IDPResolverProvider, len(adapters))

	for i, a := range adapters {
		adapterResponse[i] = IDPResolverProvider{
			ID:   a.GetProviderID(),
			Name: a.GetProviderType(),
		}
	}
	response := &APIIDPResolverResponse{
		// RedirectURI: redirectURI,
		Providers:   adapterResponse,
		AppID:       appUUID.String(),
	}
	dto.SendSuccess(w, http.StatusOK, response)
}

// @Summary Login IDP
// @Description Login IDP.
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param idpdetails body IDPLoginRequest true "IDP Details"
// @Success 200
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /authorize/login [post]
func (i *IDPHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req IDPLoginRequest
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	IDPUUID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Not Valid Client App"))
		return
	}

	IdpConfig, err := i.idpService.GetIDPByID(ctx, IDPUUID)

	if err != nil {
		dto.SendError(w, dto.NewForbiddenError("An Error Occurred"))
		return
	}

	oidcUrl, err := i.idpService.BuildOAuthUrl(ctx, IdpConfig, req.GatewayID)

	if err != nil {
		dto.SendError(w, dto.NewNotFoundError("Not Valid Client"))
		return
	}

	dto.SendSuccess(w, http.StatusCreated, oidcUrl)
}


// @Summary Create IDP
// @Description Create IDP.
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param idpdetails body CreateIDPConfig true "IDP"
// @Success 200 {object} IDPConfigResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /idp-configs [post]
func (i *IDPHandler) CreateIdentityConfig(w http.ResponseWriter, r *http.Request) {
	orgIDStr := middleware.OrgIDFromCtx(r.Context())
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	var req CreateIDPConfig
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	cfg, err := i.idpService.CreateTenantIdentityConfig(r.Context(), orgID, req)
	if err != nil {
		dto.SendError(w, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil))
		return
	}

	dto.SendSuccess(w, http.StatusCreated, IDPConfigToResponse(cfg))
}



// @Summary List idps by organization
// @Description Retrieve a list of idps for a admin organization.
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} IDPConfigResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /idp-configs [get]
func (i *IDPHandler) ListIdentityConfigs(w http.ResponseWriter, r *http.Request) {
	orgIDStr := middleware.OrgIDFromCtx(r.Context())
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	configs, err := i.idpService.ListIdentityConfigsForTenant(r.Context(), orgID)
	if err != nil {
		dto.SendError(w, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil))
		return
	}

	resp := make([]IDPConfigResponse, 0, len(configs))
	for _, cfg := range configs {
		resp = append(resp, IDPConfigToResponse(cfg))
	}

	dto.SendSuccess(w, http.StatusOK, resp)
}



// @Summary Update Identity Config
// @Description Update Identity Config.
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "IDP ID"
// @Param idp body UpdateIDPConfig true "IDP update details"
// @Success 200 {object} IDPConfigResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /idp-configs/{id} [put]
func (i *IDPHandler) UpdateIdentityConfig(w http.ResponseWriter, r *http.Request) {
	orgIDStr := middleware.OrgIDFromCtx(r.Context())
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid ID format"))
		return
	}

	var req UpdateIDPConfig
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	cfg, err := i.idpService.UpdateIdentityConfig(r.Context(), id, orgID, req)
	if err != nil {
		dto.SendError(w, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil))
		return
	}

	dto.SendSuccess(w, http.StatusOK, IDPConfigToResponse(cfg))
}



// @Summary Delete an identityconfig
// @Description Delete an identityconfigD.
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "App ID"
// @Success 200 {object} IDPConfigResponse
// @Failure 400 {object} dto.AppError
// @Failure 404 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /idp-configs/{id} [delete]
func (i *IDPHandler) DeleteIdentityConfig(w http.ResponseWriter, r *http.Request) {
	orgIDStr := middleware.OrgIDFromCtx(r.Context())
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Organization ID format"))
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid ID format"))
		return
	}

	cfg, err := i.idpService.DeleteIdentityConfig(r.Context(), id, orgID)
	if err != nil {
		dto.SendError(w, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil))
		return
	}

	dto.SendSuccess(w, http.StatusOK, IDPConfigToResponse(cfg))
}

func (i *IDPHandler) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		dto.SendError(w, dto.NewBadRequestError("Not Valid"))
		return
	}

	_, token, err := i.idpService.ExchangeService(ctx, state, code)
	if err != nil {
		dto.SendError(w, dto.NewAppError(500, dto.CodeForbidden, err.Error(), nil))
		return
	}

	//Redirect to Gateway with token
	redirectUrl := fmt.Sprintf("%s/_auth/callback?state=%s", "http://gateway.ashrix.io:8000", url.QueryEscape(token))
	// dto.SendSuccess(w, http.StatusCreated, redirectUrl)
	http.Redirect(w,r,redirectUrl, http.StatusTemporaryRedirect)
}


// @Summary Revoke Session Handler
// @Description Revoke Session Handler.
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /revoke-session [post]
func (h *IDPHandler) RevokeUserSessionHandler(w http.ResponseWriter, r *http.Request) {

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Invalid Session ID format"))
		return
	}

	_, err = h.idpService.RevokeUserSession(r.Context(), id)
	if err != nil {
		dto.SendError(w, dto.NewAppError(500, dto.CodeInternal, err.Error(), nil))
		return
	}

	dto.SendSuccess(w, http.StatusOK, "All sessions revoked successfully")
}




func (h *IDPHandler) IdentityAuthRoutes(rg chi.Router) {
	rg.Get("/providers", h.IDPResolverHandler)
	rg.Post("/login", h.LoginHandler)
	rg.Get("/callback", h.CallbackHandler)
}
func (h *IDPHandler) IdentityRoutes(rg chi.Router) {
    rg.Post("/", h.CreateIdentityConfig)
    rg.Get("/", h.ListIdentityConfigs)
    rg.Put("/{id}", h.UpdateIdentityConfig)
    rg.Delete("/{id}", h.DeleteIdentityConfig)
	rg.Delete("/revoke-session/{id}", h.RevokeUserSessionHandler)
}