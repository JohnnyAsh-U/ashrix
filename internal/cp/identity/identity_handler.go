package identity

import (
	"embed"
	"fmt"
	"html/template"
	iofs "io/fs"
	"log/slog"
	"net/http"
	"net/url"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/oidc"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

//go:embed web/*
var idpTemplateFS embed.FS

type IDPHandler struct {
	idpService *IDPService
	log        *slog.Logger
	templates  *template.Template
}

func NewIDPHandler(idpService *IDPService, log *slog.Logger) *IDPHandler {

	funcs := template.FuncMap{
		"providerDisplayName": providerDisplayName,
		"providerIcon":        providerIcon,
	}

	tmpl := template.Must(
		template.New("idp").Funcs(funcs).ParseFS(
			idpTemplateFS,
			"web/templates/idp_login.html",
			"web/templates/error.html",
		),
	)

	return &IDPHandler{
		idpService: idpService,
		log:        log,
		templates:  tmpl,
	}
}

// @Summary Resolve IDP for tenant and app
// @Description Resolve IDP for the specified tenant and app.
// @Tags IDP
// @Accept json
// @Produce html
// @Security BearerAuth
// @Param gid query string true "Gateway ID"
// @Param aid query string true "App ID"
// @Success 201 {object} string
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
	providers, err := i.idpService.ResolveAppIDP(ctx, appUUID, gatewayUUID)
	if err != nil {
		w.Header().Set("Content-Type","text/html; charset=utf-8")
		i.templates.ExecuteTemplate(w, "error.html", nil)
		// dto.SendError(w, dto.NewNotFoundError(err.Error()))
		return
	}

	page := IDPLoginPageData{
		Providers: providers,
	}

	w.Header().Set(
		"Content-Type",
		"text/html; charset=utf-8",
	)

	if err := i.templates.ExecuteTemplate(
		w,
		"idp_login.html",
		page,
	); err != nil {

		slog.Error(
			"failed to render IDP login page",
			"error", err,
		)

		// Don't attempt to write another status if
		// the template already started writing.
	}

	// dto.SendSuccess(w, http.StatusOK, response)
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

// @Summary Add APP to IDP
// @Description Add App to IDP
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param app body CreateAppIDPRelation true "Add App IDP"
// @Success 200
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /idp-configs/app [post]
func (h *IDPHandler) AddAppToIDP(w http.ResponseWriter, r *http.Request) {

	var req CreateAppIDPRelation
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	appidp, idpErr := h.idpService.AddAppToIDP(r.Context(), req.AppID, req.IDPID, req.IsRequired)
	if idpErr != nil {
		dto.SendError(w, idpErr)
		return
	}
	dto.SendSuccess(w, http.StatusOK, appidp)
}

// @Summary Remove APP from IDP
// @Description Remove App from IDP
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param app body RemoveAppIDPRelation true "Add App IDP"
// @Success 204
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /idp-configs/app [delete]
func (h *IDPHandler) RemoveAppFromIDP(w http.ResponseWriter, r *http.Request) {

	var req RemoveAppIDPRelation
	if err := dto.DecodeJSON(w, r, &req); err != nil {
		dto.SendError(w, err)
		return
	}

	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	appidp, idpErr := h.idpService.RemoveAppFromIDP(r.Context(), req.AppID, req.IDPID)
	if idpErr != nil {
		dto.SendError(w, idpErr)
		return
	}
	dto.SendSuccess(w, http.StatusNoContent, appidp)
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

	gatewayurl, token, err := i.idpService.ExchangeService(ctx, state, code)
	if err != nil {
		dto.SendError(w, dto.NewAppError(500, dto.CodeForbidden, err.Error(), nil))
		return
	}

	gatewayRedirectUrl := fmt.Sprintf("https://%s:8443", gatewayurl)
	//Redirect to Gateway with token
	redirectUrl := fmt.Sprintf("%s/_ashrix/auth/callback?state=%s", gatewayRedirectUrl, url.QueryEscape(token))
	// dto.SendSuccess(w, http.StatusCreated, redirectUrl)
	http.Redirect(w, r, redirectUrl, http.StatusTemporaryRedirect)
}

// @Summary Revoke Session Handler
// @Description Revoke Session Handler.
// @Tags IDP
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /idp-configs/revoke-session/{id} [delete]
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

func (i *IDPHandler) StaticHandler() http.Handler {
	// Serve the public static assets, not the private templates
	staticFS, err := iofs.Sub(idpTemplateFS, "web/static")

	if err != nil {
		panic(err)
	}

	return http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
}

func (h *IDPHandler) IdentityAuthRoutes(rg chi.Router) {
	rg.Get("/providers", h.IDPResolverHandler)
	rg.Get("/callback", h.CallbackHandler)
}


func (h *IDPHandler) IdentityRoutes(rg chi.Router) {
	rg.Post("/", h.CreateIdentityConfig)
	rg.Get("/", h.ListIdentityConfigs)
	rg.Put("/{id}", h.UpdateIdentityConfig)
	rg.Post("/app", h.AddAppToIDP)
	rg.Delete("/{id}", h.DeleteIdentityConfig)
	rg.Delete("/revoke-session/{id}", h.RevokeUserSessionHandler)
}
