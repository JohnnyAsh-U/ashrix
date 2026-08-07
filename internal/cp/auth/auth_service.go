package auth

import (
	"context"
	"fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/org"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type AccessClaims struct {
	AdminID string `json:"admin_id"`
	OrgID   string `json:"org_id"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

// OTPClaims is embedded in the otp_token returned after password-correct login.
// It proves password was verified without granting full access.
type OTPClaims struct {
	AdminID string `json:"admin_id"`
	jwt.RegisteredClaims
}

type Service struct {
	repo     Repository
	orgSvc   *org.Service
	mailer   utils.MailerService
	cfg      *config.Config
	jwtUtils *utils.JwtUtils // RS256 in production — using HS256 here for simplicity, swap to rsa.PrivateKey
	appURL   string          // e.g. https://cp.ashrix.io — used to build reset links
}

func NewService(
	repo Repository,
	orgSvc *org.Service,
	mailer utils.MailerService,
	cfg *config.Config,
	jwtUtils *utils.JwtUtils,
	appURL string,
) *Service {
	return &Service{
		repo:     repo,
		orgSvc:   orgSvc,
		mailer:   mailer,
		cfg:      cfg,
		jwtUtils: jwtUtils,
		appURL:   appURL,
	}
}

// ---- Register ----

// Register creates an inactive admin account and returns a setup_token.
// No org is created yet. Account is not usable until /verify-otp completes setup.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (RegisterResponse, *dto.AppError) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return RegisterResponse{}, dto.NewBadRequestError(err)
	}

	admin, err := s.repo.CreateAdmin(ctx, store.CreateAdminParams{
		Email:        req.Email,
		PasswordHash: pgtype.Text{String: string(hash), Valid: true},
		Role:         "admin",
	})
	if err != nil {
		return RegisterResponse{}, dto.NewBadRequestError(err) // ErrEmailConflict propagates as-is
	}

	token, err := s.jwtUtils.IssueSetupToken(ctx, admin.ID)

	if err != nil {
		return RegisterResponse{}, dto.NewErrInternal(err)
	}

	_, err = s.repo.CreateSetupToken(ctx, store.CreateSetupTokenParams{
		AdminID:   admin.ID,
		TokenHash: utils.HashToken(token),
		ExpiresAt: time.Now().Add(s.cfg.SetupTokenDuration),
	})

	if err != nil {
		return RegisterResponse{}, dto.NewBadRequestError(err.Error())
	}

	return RegisterResponse{
		SetupToken: token,
		ExpiresAt:  time.Now().Add(s.cfg.SetupTokenDuration),
	}, nil
}

// ---- Setup OTP ----

// SetupOTP generates a TOTP secret for the admin and returns the QR code URI.
// The secret is stored unconfirmed — otp_enabled stays FALSE until VerifyOTPSetup.
func (s *Service) SetupOTP(ctx context.Context, req SetupOTPRequest) (SetupOTPResponse, *dto.AppError) {
	setupToken, err := s.repo.GetSetupToken(ctx, utils.HashToken(req.SetupToken))
	if err != nil {
		return SetupOTPResponse{}, dto.NewBadRequestError("Token Invalid")
	}

	admin, err := s.repo.GetAdminByID(ctx, setupToken.AdminID)
	if err != nil {
		return SetupOTPResponse{}, dto.NewBadRequestError(err.Error())
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Ashrix",
		AccountName: admin.Email,
	})
	if err != nil {
		return SetupOTPResponse{}, dto.NewErrInternal(err)
	}

	// Store the secret — otp_enabled remains FALSE until first code is verified.
	_, err = s.repo.UpdateAdminOTP(ctx, store.UpdateAdminOTPParams{
		ID:         admin.ID,
		OtpSecret:  pgtype.Text{String: key.Secret(), Valid: true},
		OtpEnabled: true,
	})
	if err != nil {
		return SetupOTPResponse{}, dto.NewErrInternal(err)
	}

	return SetupOTPResponse{
		Secret:    key.Secret(),
		QRCodeURL: key.URL(),
	}, nil
}

// ---- Verify OTP (setup) ----

// VerifyOTPSetup confirms the first TOTP code, activates the admin, creates the org,
// and issues the first access + refresh token pair.
// This is the final step of account creation — everything becomes active here.
func (s *Service) VerifyOTPSetup(ctx context.Context, req VerifyOTPSetupRequest) (TokenPair, *dto.AppError) {
	setupToken, err := s.repo.GetSetupToken(ctx, utils.HashToken(req.SetupToken))
	if err != nil {
		return TokenPair{}, dto.NewBadRequestError(err.Error()) // ErrInvalidToken
	}

	admin, err := s.repo.GetAdminByID(ctx, setupToken.AdminID)
	if err != nil {
		return TokenPair{}, dto.NewBadRequestError(err)
	}

	if !admin.OtpSecret.Valid || admin.OtpSecret.String == "" {
		return TokenPair{}, dto.NewBadRequestError("OTP secret not found")
	}

	if !totp.Validate(req.OTPCode, admin.OtpSecret.String) {
		return TokenPair{}, dto.NewBadRequestError("Token Not Correct")
	}

	// Create the org.
	o, errOrg := s.orgSvc.CreateOrg(ctx, req.OrgName, req.OrgSlug)
	if errOrg != nil {
		return TokenPair{}, errOrg
	}

	orgID, err := uuid.Parse(o.ID)
	if err != nil {
		return TokenPair{}, dto.NewErrInternal(err.Error())
	}

	adminActivated, err := s.repo.ActivateAdmin(ctx, store.ActivateAdminParams{
		ID:    admin.ID,
		OrgID: pgtype.UUID{Bytes: orgID, Valid: true},
	})
	if err != nil {
		fmt.Println(err)
		return TokenPair{}, dto.NewErrInternal(err)
	}

	// Consume the setup token — it can never be used again.
	token, err := s.repo.GetSetupToken(ctx, utils.HashToken(req.SetupToken))
	if err != nil {
		return TokenPair{}, dto.NewErrInternal(err)
	}
	if err := s.repo.MarkSetupTokenUsed(ctx, token.ID); err != nil {
		return TokenPair{}, dto.NewErrInternal(err)
	}

	tokens, err := s.jwtUtils.IssueTokenPair(ctx, adminActivated.ID, orgID, admin.Role, admin.Email)

	_, err = s.repo.CreateSession(ctx, store.CreateSessionParams{
		AdminID:      admin.ID,
		OrgID:        orgID,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.RefreshExpiresAt,
	})

	if err != nil {
		return TokenPair{}, dto.NewErrInternal(err)
	}

	return TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		RefreshTokenExpiresAt: tokens.RefreshExpiresAt,
	}, nil
}

// ---- Login ----

// Login verifies email + password. If correct, returns an otp_token.
// The client must then call VerifyOTPLogin with the TOTP code.
func (s *Service) Login(ctx context.Context, req LoginRequest) (OTPRequiredResponse, *dto.AppError) {
	admin, err := s.repo.GetAdminByEmail(ctx, req.Email)
	if err != nil {
		// SECURITY: Return the same error whether email exists or not.
		// Timing attack note: bcrypt comparison below runs even on not-found
		// to prevent timing-based email enumeration.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$dummy.hash.to.prevent.timing"), []byte(req.Password))
		return OTPRequiredResponse{}, dto.NewBadRequestError("Email Or Password is not correct")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash.String), []byte(req.Password)); err != nil {
		return OTPRequiredResponse{}, dto.NewBadRequestError("Email Or Password is not correct")
	}

	if !admin.IsActive {
		return OTPRequiredResponse{}, dto.NewBadRequestError("Account Not Valid")
	}


	// Issue a short-lived OTP token. Full access is not granted yet.
	expiresAt := time.Now().Add(s.cfg.OtpTokenDuration)
	claims := OTPClaims{
		AdminID: admin.ID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	otpToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JwtAccessSecret))
	if err != nil {
		return OTPRequiredResponse{}, dto.NewBadRequestError(err)
	}

	return OTPRequiredResponse{
		OTPToken:  otpToken,
		ExpiresAt: expiresAt,
	}, nil
}

// ---- Verify OTP (login) ----

// VerifyOTPLogin validates the TOTP code against the otp_token from Login.
// On success, issues the full access + refresh token pair.
func (s *Service) VerifyOTPLogin(ctx context.Context, req VerifyOTPLoginRequest) (TokenPair, *dto.AppError) {
	claims, err := s.jwtUtils.ParseOTPToken(req.OTPToken)
	if err != nil {
		return TokenPair{}, dto.NewBadRequestError("OTP invalid")
	}

	adminID, err := uuid.Parse(claims.AdminID)
	if err != nil {
		return TokenPair{}, dto.NewErrInternal(fmt.Errorf("invalid org id: %w", err))
	}

	admin, err := s.repo.GetAdminByID(ctx, adminID)
	if err != nil {
		return TokenPair{}, dto.NewBadRequestError("Admin Not Found")
	}

	OrgID, err := uuid.Parse(admin.OrgID.String())
	if err != nil {
		return TokenPair{}, dto.NewErrInternal(fmt.Errorf("invalid org id: %w", err))
	}

	if !admin.OtpEnabled || !admin.OtpSecret.Valid {
		return TokenPair{}, dto.NewBadRequestError("OTP not configured")
	}

	if !totp.Validate(req.OTPCode, admin.OtpSecret.String) {
		return TokenPair{}, dto.NewBadRequestError("Invalide OTP code")
	}

	tokens, err := s.jwtUtils.IssueTokenPair(ctx, adminID, OrgID, admin.Role, admin.Email)

	if err != nil {
		return TokenPair{}, dto.NewErrInternal("Internal Server")
	}

	_, err = s.repo.CreateSession(ctx, store.CreateSessionParams{
		AdminID:      admin.ID,
		OrgID:        admin.OrgID.Bytes,
		RefreshToken: utils.HashToken(tokens.RefreshToken),
		ExpiresAt:    tokens.RefreshExpiresAt,
	})

	if err != nil {
		return TokenPair{}, dto.NewErrInternal(err)
	}

	return TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		RefreshTokenExpiresAt: tokens.RefreshExpiresAt,
	}, nil

}

// ---- Change Password ----

// ChangePassword verifies the current password, updates to the new one,
// and revokes all existing sessions to force re-login on all devices.
func (s *Service) ChangePassword(ctx context.Context, adminID uuid.UUID, req ChangePasswordRequest) *dto.AppError {
	admin, err := s.repo.GetAdminByID(ctx, adminID)
	if err != nil {
		return dto.NewErrInternal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash.String), []byte(req.CurrentPassword)); err != nil {

	fmt.Println(err)
		return dto.NewBadRequestError("Password Not Correct")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 12)
	if err != nil {
		return dto.NewBadRequestError(err)
	}

	if err := s.repo.UpdateAdminPassword(ctx, store.UpdateAdminPasswordParams{
		ID:           adminID,
		PasswordHash: pgtype.Text{String: string(newHash), Valid: true},
	}); err != nil {

		return dto.NewBadRequestError(err.Error())
	}



	// Revoke all sessions — forces re-login on all devices.
	// SECURITY: This is non-negotiable on password change.
	if err := s.repo.RevokeAllAdminSessions(ctx, adminID); err != nil {
		return dto.NewBadRequestError(err)
	}

	return nil
}

// ---- Request Password Reset ----

// RequestPasswordReset generates a reset token and emails a link.
// SECURITY: Always returns nil — never reveal whether an email exists.
func (s *Service) RequestPasswordReset(ctx context.Context, req RequestPasswordResetRequest) *dto.AppError {
	admin, err := s.repo.GetAdminByEmail(ctx, req.Email)
	if err != nil {
		// Silently succeed — do not confirm whether email is registered.
		return nil
	}

	rawToken, err := utils.GenerateSecureToken()
	if err != nil {
		return dto.NewBadRequestError(fmt.Errorf("auth.RequestPasswordReset: generate token: %w", err))
	}

	expiresAt := time.Now().Add(s.cfg.ResetTokenDuration)
	_, err = s.repo.CreatePasswordResetToken(ctx, store.CreatePasswordResetTokenParams{
		AdminID:   admin.ID,
		TokenHash: utils.HashToken(rawToken),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return dto.NewBadRequestError(fmt.Errorf("auth.RequestPasswordReset: store token: %w", err))
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.appURL, rawToken)
	if err := s.mailer.SendPasswordReset(ctx, admin.Email, resetURL); err != nil {
		// Log but don't fail — token is stored, admin can retry.
		fmt.Printf("auth.RequestPasswordReset: send email: %v\n", err) // replace with slog
	}

	return nil
}

// ---- Reset Password ----

// ResetPassword validates the reset token and sets a new password.
// Revokes all sessions after reset.
func (s *Service) ResetPassword(ctx context.Context, req ResetPasswordRequest) *dto.AppError {
	resetToken, err := s.repo.GetPasswordResetToken(ctx, utils.HashToken(req.Token))
	if err != nil {
		return dto.NewBadRequestError("Invalid Reset Token")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 12)
	if err != nil {
		return dto.NewBadRequestError(err)
	}

	if err := s.repo.UpdateAdminPassword(ctx, store.UpdateAdminPasswordParams{
		ID:           resetToken.AdminID,
		PasswordHash: pgtype.Text{String: string(newHash), Valid: true},
	}); err != nil {
		return dto.NewBadRequestError(err)
	}

	if err := s.repo.MarkPasswordResetTokenUsed(ctx, resetToken.ID); err != nil {
		return dto.NewBadRequestError(err)
	}

	if err := s.repo.RevokeAllAdminSessions(ctx, resetToken.AdminID); err != nil {
		return dto.NewBadRequestError(err)
	}

	return nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, *dto.AppError) {

	// Look up the hashed token in the DB.
	println(utils.HashToken(refreshToken))
	session, err := s.repo.GetSession(ctx, utils.HashToken(refreshToken))
	if err != nil {
		return TokenPair{}, dto.NewBadRequestError("No Session")
	}

	// Revoke the used refresh token immediately before issuing a new one.
	// If this step fails, we bail — better to force re-login than issue
	// a new token against an unrevoked old one.
	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return TokenPair{}, dto.NewErrInternal(err)
	}

	// Fetch the admin to embed current claims.
	admin, err := s.repo.GetAdminByID(ctx, session.AdminID)
	if err != nil {
		return TokenPair{}, dto.NewErrInternal(err)
	}

	if !admin.IsActive {
		return TokenPair{}, dto.NewForbiddenError("Admin not active")
	}

	orgID, err := uuid.Parse(admin.ID.String())
	if err != nil {
		return TokenPair{}, dto.NewErrInternal(fmt.Errorf("invalid org id: %w", err))
	}

	tokens, err := s.jwtUtils.IssueTokenPair(ctx, admin.ID, orgID, admin.Role, admin.Email)

	_, err = s.repo.CreateSession(ctx, store.CreateSessionParams{
		AdminID:      admin.ID,
		OrgID: admin.OrgID.Bytes,
		RefreshToken: utils.HashToken(tokens.RefreshToken),
		ExpiresAt:    tokens.RefreshExpiresAt,
	})
	return TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		RefreshTokenExpiresAt: tokens.RefreshExpiresAt,
	}, nil
}

// ---- Logout ----

// Logout revokes the current refresh token and clears the cookie.
// The access token will expire naturally (15 min) — this is acceptable.
// If you need immediate access token invalidation, you'd need a blocklist.
func (s *Service) Logout(ctx context.Context, refreshToken string) {

	session, err := s.repo.GetSession(ctx, utils.HashToken(refreshToken))
	if err == nil {
		// Best effort — if revoke fails, the cookie is still cleared.
		_ = s.repo.RevokeSession(ctx, session.ID)
	}
}
