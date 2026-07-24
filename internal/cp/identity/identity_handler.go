package identity

import (
	"log/slog"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/oidc"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
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
// @Param X-ID header string true "Org ID"
// @Param X-App-ID header string true "App ID"
// @Param redirect_uri query string true "Redirect URI"
// @Success 201 {object} APIIDPResolverResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /authorize/providers [get]
func (i *IDPHandler) IDPResolverHandler(w http.ResponseWriter, r *http.Request) {
	// /login?id=X&app=Y
	ctx := r.Context()
	tenantID := r.Header.Get("X-ID")
	appID := r.Header.Get("X-App-ID")
	redirectURI := r.URL.Query().Get("redirect_uri")

	if tenantID == "" || appID == "" || redirectURI == "" {
		dto.SendError(w, dto.NewBadRequestError("Not Valid"))
		return
	}
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Not Valid Client App"))
		return
	}

	appUUID, err := uuid.Parse(appID)
	if err != nil {
		dto.SendError(w, dto.NewBadRequestError("Not Valid Client App"))
		return
	}

	//Resolve the AppIDPProviders
	adapters, err := i.idpService.ResolveAppIDP(ctx, tenantUUID, appUUID)
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
		RedirectURI: redirectURI,
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
// @Success 200 string
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

	redirectURIValid, IdpConfig := i.idpService.IsRedirectURIValidByIDP(ctx, IDPUUID, req.RedirectURI)

	if !redirectURIValid {
		dto.SendError(w, dto.NewForbiddenError("An Error Occurred"))
		return
	}

	oidcUrl, err := i.idpService.BuildOAuthUrl(ctx, IdpConfig, req.AppID, req.RedirectURI)

	if err != nil {
		dto.SendError(w, dto.NewNotFoundError("Not Valid Client"))
		return
	}

	dto.SendSuccess(w, http.StatusCreated, oidcUrl)
}




func (i *IDPHandler) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		dto.SendError(w, dto.NewBadRequestError("Not Valid"))
		return
	}

	identity, redirectURI, err := i.idpService.ExchangeService(ctx, state, code)
	if err != nil {
		dto.SendError(w, dto.NewAppError(500, dto.CodeForbidden, err.Error(), nil))
		return
	}
	Response := struct {
		RedirectURI string
		Identity *oidc.NormalizedIdentity

	}{
		RedirectURI: redirectURI,
		Identity: identity,
	}
	dto.SendSuccess(w, http.StatusCreated, Response)
}

func (h *IDPHandler) Routes(rg chi.Router) {
	rg.Get("/providers", h.IDPResolverHandler)
	rg.Post("/login", h.LoginHandler)
	rg.Get("/callback", h.CallbackHandler)
}
