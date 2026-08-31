package auth

import (
	"fmt"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AuthHandler struct {
	svc       *Service
	accessKey string
}

func NewAuthHandler(svc *Service, accessKey string) *AuthHandler {
	return &AuthHandler{svc: svc, accessKey: accessKey}
}

// @Summary Register
// @Description Admin Registers
// @Tags Auth
// @Accept json
// @Produce json
// @Param org body RegisterRequest true "Register details"
// @Success 201 {object} RegisterResponse
// @Failure 400 {object} dto.AppError
// @Router /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if apiErr := dto.DecodeJSON(w, r, &req); apiErr != nil {
		dto.SendError(w, apiErr)
		return
	}
	if validationErrors := dto.ValidateStruct(req); validationErrors != nil {
		dto.SendError(w, dto.NewBadRequestError(validationErrors))
		return
	}

	resp, err := h.svc.Register(r.Context(), req)
	if err != nil {
		dto.SendError(w, err)
		return
	}

	dto.SendSuccess(w, http.StatusCreated, resp)
}

// @Summary SetupOTP
// @Description Admin SetupOTP After Registration
// @Tags Auth
// @Accept json
// @Produce json
// @Param org body SetupOTPRequest true "Register details"
// @Success 200 {object} SetupOTPResponse
// @Failure 400 {object} dto.AppError
// @Router /auth/setup-otp [post]
func (h *AuthHandler) SetupOTP(w http.ResponseWriter, r *http.Request) {
	var req SetupOTPRequest
	if apiErr := dto.DecodeJSON(w, r, &req); apiErr != nil {
		dto.SendError(w, apiErr)
		return
	}

	if apiErr := dto.ValidateStruct(req); apiErr != nil {
		dto.SendError(w, dto.NewBadRequestError(apiErr))
		return
	}

	resp, err := h.svc.SetupOTP(r.Context(), req)
	if err != nil {
		dto.SendError(w, err)
		return
	}
	dto.SendSuccess(w, http.StatusOK, resp)
}

// @Summary VerifyOTPSetup
// @Description Admin VerifyOTPSetup After Registration
// @Tags Auth
// @Accept json
// @Produce json
// @Param org body VerifyOTPSetupRequest true "Register details"
// @Success 200 {object} TokenPair
// @Failure 400 {object} dto.AppError
// @Router /auth/verify-otp/setup [post]
func (h *AuthHandler) VerifyOTPSetup(w http.ResponseWriter, r *http.Request) {
	var req VerifyOTPSetupRequest
	if apiErr := dto.DecodeJSON(w, r, &req); apiErr != nil {
		dto.SendError(w, apiErr)
		return
	}
	if apiErr := dto.ValidateStruct(req); apiErr != nil {
		dto.SendError(w, dto.NewBadRequestError(apiErr))
		return
	}

	resp, err := h.svc.VerifyOTPSetup(r.Context(), req)
	if err != nil {
		dto.SendError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/v1/auth/refresh", // scoped — only sent to this endpoint
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  resp.RefreshTokenExpiresAt,
	})
	dto.SendSuccess(w, http.StatusOK, resp)
}

// @Summary Login
// @Description Admin Login
// @Tags Auth
// @Accept json
// @Produce json
// @Param org body LoginRequest true "Register details"
// @Success 200 {object} OTPRequiredResponse
// @Failure 400 {object} dto.AppError
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if apiErr := dto.DecodeJSON(w, r, &req); apiErr != nil {
		dto.SendError(w, apiErr)
		return
	}
	if apiErr := dto.ValidateStruct(req); apiErr != nil {
		dto.SendError(w, dto.NewBadRequestError(apiErr))
		return
	}

	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		fmt.Println(err)
		dto.SendError(w, err)
		return
	}

	dto.SendSuccess(w, http.StatusOK, resp)
}

// @Summary Login OTP Verify
// @Description Admin Login OTP Verify
// @Tags Auth
// @Accept json
// @Produce json
// @Param org body VerifyOTPLoginRequest true "Register details"
// @Success 200 {object} LoginAdminResponse
// @Failure 400 {object} dto.AppError
// @Router /auth/verify-otp/login [post]
func (h *AuthHandler) VerifyOTPLogin(w http.ResponseWriter, r *http.Request) {
	var req VerifyOTPLoginRequest
	if apiErr := dto.DecodeJSON(w, r, &req); apiErr != nil {
		dto.SendError(w, apiErr)
		return
	}
	if apiErr := dto.ValidateStruct(req); apiErr != nil {
		dto.SendError(w, dto.NewBadRequestError(apiErr))
		return
	}

	resp, err := h.svc.VerifyOTPLogin(r.Context(), req)
	if err != nil {
		dto.SendError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.Tokens.RefreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  resp.Tokens.RefreshTokenExpiresAt,
	})

	dto.SendSuccess(w, http.StatusOK, resp)
}

// @Summary Change Password
// @Description Change Password
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param org body ChangePasswordRequest true "Register details"
// @Success 204 {object} nil
// @Failure 400 {object} dto.AppError
// @Router /auth/change-password [post]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	adminID := middleware.AdminIDFromCtx(r.Context())
	if adminID == "" {
		dto.SendError(w, dto.NewUnauthorizedError("Not Authorized"))
		return
	}

	var req ChangePasswordRequest
	if apiErr := dto.DecodeJSON(w, r, &req); apiErr != nil {
		dto.SendError(w, apiErr)
		return
	}
	if apiErr := dto.ValidateStruct(req); apiErr != nil {
		dto.SendError(w, dto.NewBadRequestError(apiErr))
		return
	}

	adminUUID, err := uuid.Parse(adminID)

	if err != nil {
		dto.SendError(w, dto.NewBadRequestError(err))
		return
	}

	changePassErr := h.svc.ChangePassword(r.Context(), adminUUID, req)
	if changePassErr != nil {
		dto.SendError(w, changePassErr)
		return
	}

	dto.SendSuccess(w, 204, nil)
}

// @Summary Reset Password Request
// @Description Reset Password Request
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param org body RequestPasswordResetRequest true "Register details"
// @Success 204 {object} nil
// @Failure 400 {object} dto.AppError
// @Router /auth/reset-password [post]
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req RequestPasswordResetRequest
	if apiErr := dto.DecodeJSON(w, r, &req); apiErr != nil {
		dto.SendError(w, apiErr)
		return
	}
	if apiErr := dto.ValidateStruct(req); apiErr != nil {
		dto.SendError(w, dto.NewBadRequestError(apiErr))
		return
	}

	// Service always returns nil — failure is silent by design.
	_ = h.svc.RequestPasswordReset(r.Context(), req)

	dto.SendSuccess(w, 204, nil)
}

// @Summary Reset Password
// @Description Reset Password
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param org body ResetPasswordRequest true "Register details"
// @Success 204 {object} nil
// @Failure 400 {object} dto.AppError
// @Router /auth/reset-password [put]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if apiErr := dto.DecodeJSON(w, r, &req); apiErr != nil {
		dto.SendError(w, apiErr)
		return
	}
	if apiErr := dto.ValidateStruct(req); apiErr != nil {
		dto.SendError(w, dto.NewBadRequestError(apiErr))
		return
	}

	err := h.svc.ResetPassword(r.Context(), req)
	if err != nil {
		dto.SendError(w, err)
		return
	}

	dto.SendSuccess(w, 204, nil)
}

// @Summary Refresh Token
// @Description Refresh Token
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 204 {object} nil
// @Failure 400 {object} dto.AppError
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Read refresh token from HttpOnly cookie — never from the request body.
	cookie, err := r.Cookie("refresh_token")

	if err != nil {
		dto.SendError(w, dto.NewUnauthorizedError(err))
		return
	}

	rawToken := cookie.Value
	if rawToken == "" {
		dto.SendError(w, dto.NewUnauthorizedError("Refresh Token Invalid"))
		return
	}

	// Look up the hashed token in the DB.
	tokens, refreshError := h.svc.Refresh(r.Context(), rawToken)
	if refreshError != nil {
		dto.SendError(w, refreshError)
		return
	}

	// Set the new refresh token as an HttpOnly cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    tokens.RefreshToken,
		Path:     "/", // scoped — only sent to this endpoint
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  tokens.RefreshTokenExpiresAt,
	})
	dto.SendSuccess(w, http.StatusOK, tokens)
}

// @Summary Logout
// @Description Logout
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 204 {object} nil
// @Failure 400 {object} dto.AppError
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		// Already logged out — return success.
		dto.SendSuccess(w, http.StatusOK, nil)
		return
	}

	h.svc.Logout(r.Context(), cookie.Value)

	// Clear the cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	dto.SendSuccess(w, 204, nil)
}

// @Summary Get ME
// @Description Get ME
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MeResponse
// @Failure 400 {object} dto.AppError
// @Router /auth/me [get]
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request){
	adminID := middleware.AdminIDFromCtx(r.Context())
	if adminID == "" {
		dto.SendError(w, dto.NewUnauthorizedError("Not Authorized"))
		return
	}

	adminUUID, err := uuid.Parse(adminID)

	if err != nil {
		dto.SendError(w, dto.NewBadRequestError(err))
		return
	}

	admin, appErr := h.svc.GetMe(r.Context(), adminUUID)
	if appErr != nil {
		dto.SendError(w, appErr)
		return
	}

	dto.SendSuccess(w, 201, admin)
}

func (h *AuthHandler) Routes(r chi.Router) {
	// Public — no auth middleware
	r.Post("/register", h.Register)
	r.Post("/setup-otp", h.SetupOTP)
	r.Post("/verify-otp/setup", h.VerifyOTPSetup)
	r.Post("/login", h.Login)
	r.Post("/verify-otp/login", h.VerifyOTPLogin)
	r.Post("/reset-password", h.RequestPasswordReset)
	r.Put("/reset-password", h.ResetPassword)
	r.Post("/refresh", h.Refresh)
	r.Post("/logout", h.Logout)

	// Protected — requires valid access token
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware([]byte(h.accessKey)))
		r.Post("/change-password", h.ChangePassword)
		r.Get("/me", h.GetMe)
	})
}
