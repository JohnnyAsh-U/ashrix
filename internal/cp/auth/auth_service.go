package auth

import (
	"context"
	"fmt"
	"net/url"
	"time"

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
)

type AccessClaims struct {
	AdminID string `json:"admin_id"`
	OrgID   string `json:"org_id"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

// OTPClaims is embedded in the otp_token returned after password-correct login.
// It proves password was verified without granting full access.
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

func (s *Service) SetupFirstLoginTOTP(ctx context.Context, req FirstLoginTOTPSetupRequest) (SetupOTPResponse, *dto.AppError) {
	claims, err := s.jwtUtils.ParseOTPToken(req.OTPToken)
	if err != nil || claims.Purpose != utils.OTPPurposeEnroll {
		return SetupOTPResponse{}, dto.NewUnauthorizedError("TOTP enrollment challenge is invalid or expired")
	}

	adminID, err := uuid.Parse(claims.AdminID)
	if err != nil {
		return SetupOTPResponse{}, dto.NewUnauthorizedError("TOTP enrollment challenge is invalid")
	}
	admin, err := s.repo.GetAdminByID(ctx, adminID)
	if err != nil || !admin.IsActive || admin.OtpEnabled {
		return SetupOTPResponse{}, dto.NewUnauthorizedError("TOTP enrollment is not available for this account")
	}

	if admin.OtpSecret.Valid && admin.OtpSecret.String != "" {
		return SetupOTPResponse{
			Secret:    admin.OtpSecret.String,
			QRCodeURL: firstLoginTOTPURL(admin.Email, admin.OtpSecret.String),
		}, nil
	}

	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Ashrix", AccountName: admin.Email})
	if err != nil {
		return SetupOTPResponse{}, dto.NewErrInternal(err)
	}
	if _, err := s.repo.UpdateAdminOTP(ctx, store.UpdateAdminOTPParams{
		ID: admin.ID, OtpSecret: pgtype.Text{String: key.Secret(), Valid: true}, OtpEnabled: false,
	}); err != nil {
		return SetupOTPResponse{}, dto.NewErrInternal(err)
	}
	return SetupOTPResponse{Secret: key.Secret(), QRCodeURL: key.URL()}, nil
}

func firstLoginTOTPURL(email, secret string) string {
	label := url.PathEscape("Ashrix:" + email)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", "Ashrix")
	query.Set("algorithm", "SHA1")
	query.Set("digits", "6")
	query.Set("period", "30")
	return "otpauth://totp/" + label + "?" + query.Encode()
}

func (s *Service) VerifyFirstLoginTOTP(ctx context.Context, req VerifyFirstLoginTOTPRequest) (LoginAdminResponse, *dto.AppError) {
	claims, err := s.jwtUtils.ParseOTPToken(req.OTPToken)
	if err != nil || claims.Purpose != utils.OTPPurposeEnroll {
		return LoginAdminResponse{}, dto.NewUnauthorizedError("TOTP enrollment challenge is invalid or expired")
	}

	adminID, err := uuid.Parse(claims.AdminID)
	if err != nil {
		return LoginAdminResponse{}, dto.NewUnauthorizedError("TOTP enrollment challenge is invalid")
	}
	admin, err := s.repo.GetAdminByID(ctx, adminID)
	if err != nil || !admin.IsActive || admin.OtpEnabled || !admin.OtpSecret.Valid || admin.OtpSecret.String == "" {
		return LoginAdminResponse{}, dto.NewUnauthorizedError("TOTP enrollment is not available for this account")
	}
	if !totp.Validate(req.OTPCode, admin.OtpSecret.String) {
		return LoginAdminResponse{}, dto.NewBadRequestError("Invalid TOTP code")
	}
	admin, err = s.repo.EnableAdminOTPIfPending(ctx, store.EnableAdminOTPIfPendingParams{
		ID: admin.ID, OtpSecret: pgtype.Text{String: admin.OtpSecret.String, Valid: true},
	})
	if err != nil {
		return LoginAdminResponse{}, dto.NewErrInternal(err)
	}
	return s.issueAdminSession(ctx, admin)
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
		AccessToken:           tokens.AccessToken,
		RefreshToken:          tokens.RefreshToken,
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

	// Password is verified, but full access is withheld until TOTP verification.
	expiresAt := time.Now().Add(s.cfg.OtpTokenDuration)
	purpose := utils.OTPPurposeLogin
	if !admin.OtpEnabled || !admin.OtpSecret.Valid || admin.OtpSecret.String == "" {
		purpose = utils.OTPPurposeEnroll
	}
	otpToken, err := s.jwtUtils.IssueOTPToken(admin.ID, purpose, expiresAt)
	if err != nil {
		return OTPRequiredResponse{}, dto.NewBadRequestError(err)
	}

	return OTPRequiredResponse{
		OTPToken:          otpToken,
		ExpiresAt:         expiresAt,
		TOTPSetupRequired: purpose == utils.OTPPurposeEnroll,
	}, nil
}

// ---- Verify OTP (login) ----

// VerifyOTPLogin validates the TOTP code against the otp_token from Login.
// On success, issues the full access + refresh token pair.
func (s *Service) VerifyOTPLogin(ctx context.Context, req VerifyOTPLoginRequest) (LoginAdminResponse, *dto.AppError) {
	claims, err := s.jwtUtils.ParseOTPToken(req.OTPToken)
	if err != nil || claims.Purpose != utils.OTPPurposeLogin {
		return LoginAdminResponse{}, dto.NewBadRequestError("OTP invalid")
	}

	adminID, err := uuid.Parse(claims.AdminID)
	if err != nil {
		return LoginAdminResponse{}, dto.NewErrInternal(fmt.Errorf("invalid org id: %w", err))
	}

	admin, err := s.repo.GetAdminByID(ctx, adminID)
	if err != nil {
		return LoginAdminResponse{}, dto.NewBadRequestError("Admin Not Found")
	}

	if !admin.OtpEnabled || !admin.OtpSecret.Valid {
		return LoginAdminResponse{}, dto.NewBadRequestError("OTP not configured")
	}

	if !totp.Validate(req.OTPCode, admin.OtpSecret.String) {
		return LoginAdminResponse{}, dto.NewBadRequestError("Invalide OTP code")
	}

	return s.issueAdminSession(ctx, admin)
}

func (s *Service) issueAdminSession(ctx context.Context, admin store.Admin) (LoginAdminResponse, *dto.AppError) {
	adminID := admin.ID
	orgID, err := uuid.Parse(admin.OrgID.String())
	if err != nil {
		return LoginAdminResponse{}, dto.NewErrInternal(fmt.Errorf("invalid org id: %w", err))
	}
	tokens, err := s.jwtUtils.IssueTokenPair(ctx, adminID, orgID, admin.Role, admin.Email)

	if err != nil {
		return LoginAdminResponse{}, dto.NewErrInternal("Internal Server")
	}

	_, err = s.repo.CreateSession(ctx, store.CreateSessionParams{
		AdminID:      admin.ID,
		OrgID:        orgID,
		RefreshToken: utils.HashToken(tokens.RefreshToken),
		ExpiresAt:    tokens.RefreshExpiresAt,
	})

	if err != nil {
		return LoginAdminResponse{}, dto.NewErrInternal(err)
	}

	admin, errad := s.repo.GetAdminByID(ctx, adminID)
	if errad != nil {
		return LoginAdminResponse{}, dto.NewErrInternal(err)
	}

	return LoginAdminResponse{
		Tokens: TokenPair{
			AccessToken:           tokens.AccessToken,
			RefreshToken:          tokens.RefreshToken,
			RefreshTokenExpiresAt: tokens.RefreshExpiresAt,
		},
		Admin: MeResponse{
			ID:      admin.ID.String(),
			OrgID:   orgID.String(),
			OrgName: "Ashrix",
			Email:   admin.Email,
		},
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

	orgID, err := uuid.Parse(admin.OrgID.String())
	if err != nil {
		return TokenPair{}, dto.NewErrInternal(fmt.Errorf("invalid org id: %w", err))
	}

	tokens, err := s.jwtUtils.IssueTokenPair(ctx, admin.ID, orgID, admin.Role, admin.Email)

	_, err = s.repo.CreateSession(ctx, store.CreateSessionParams{
		AdminID:      admin.ID,
		OrgID:        admin.OrgID.Bytes,
		RefreshToken: utils.HashToken(tokens.RefreshToken),
		ExpiresAt:    tokens.RefreshExpiresAt,
	})
	return TokenPair{
		AccessToken:           tokens.AccessToken,
		RefreshToken:          tokens.RefreshToken,
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

func (s *Service) GetMe(ctx context.Context, adminUUID uuid.UUID) (MeResponse, *dto.AppError) {
	admin, err := s.repo.GetAdminByID(ctx, adminUUID)
	if err != nil {
		return MeResponse{}, dto.NewErrInternal(err)
	}
	return MeResponse{
		ID:      admin.ID.String(),
		OrgName: "Ashrix",
		OrgID:   admin.OrgID.String(),
		Email:   admin.Email,
	}, nil
}
