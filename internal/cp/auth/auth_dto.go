package auth

import "time"

// ---- Request DTOs ----

type RegisterRequest struct {
	Email    string `json:"email"    validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type SetupOTPRequest struct {
	SetupToken string `json:"setup_token" validate:"required"`
}

type VerifyOTPSetupRequest struct {
	SetupToken string `json:"setup_token" validate:"required"`
	OTPCode    string `json:"otp_code"    validate:"required,len=6"`
	OrgName    string `json:"org_name"    validate:"required,min=2,max=100"`
	OrgSlug    string `json:"org_slug"    validate:"required,max=20"`
}

type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type VerifyOTPLoginRequest struct {
	OTPToken string `json:"otp_token" validate:"required"`
	OTPCode  string `json:"otp_code"  validate:"required,len=6"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password"     validate:"required,min=8,max=72"`
}

type RequestPasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}

// ---- Response DTOs ----

type RegisterResponse struct {
	SetupToken string `json:"setup_token"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type SetupOTPResponse struct {
	Secret    string `json:"secret"`     // base32 TOTP secret
	QRCodeURL string `json:"qr_code_url"` // otpauth:// URI for QR rendering
}

type LoginAdminResponse struct {
	Tokens TokenPair `json:"tokens"`
	Admin MeResponse `json:"admin"`

}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	RefreshTokenExpiresAt    time.Time `json:"expires_at"` // access token expiry
}

// OTPRequiredResponse is returned by /login when OTP verification is needed.
// The client must POST to /verify-otp with this token + the TOTP code.
type OTPRequiredResponse struct {
	OTPToken  string    `json:"otp_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AdminResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	OrgID     string    `json:"org_id"`
	OTPEnabled bool     `json:"otp_enabled"`
	CreatedAt time.Time `json:"created_at"`
}


type MeResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	OrgID     string    `json:"org_id"`
	OrgName string     `json:"org_name"`
}