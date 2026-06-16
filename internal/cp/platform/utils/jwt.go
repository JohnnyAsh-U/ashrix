package utils

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JwtUtils struct {
	accessKey string
	refreshKey string
	issuer    string
	accessDuration time.Duration
	refreshDuration time.Duration
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	AccessExpiresAt    time.Time `json:"expires_at"` // access token expiry
	RefreshExpiresAt   time.Time `json:"refresh_expires_at"` // refresh token expiry
}

type AccessClaims struct {
	AdminID string `json:"admin_id"`
	OrgID   string `json:"org_id"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// OTPClaims is embedded in the otp_token returned after password-correct login.
// It proves password was verified without granting full access.
type OTPClaims struct {
	AdminID string `json:"admin_id"`
	jwt.RegisteredClaims
}




func NewJwtUtils(accessKey, refreshKey string, accessDuration, refreshDuration time.Duration) *JwtUtils {
	return &JwtUtils{
		accessKey: accessKey,
		refreshKey: refreshKey,
		issuer:    "ashrix",
		accessDuration: accessDuration,
		refreshDuration: refreshDuration,
	}
}


func (j *JwtUtils) IssueTokenPair(ctx context.Context, AdminId uuid.UUID, OrgId uuid.UUID, role string) (TokenPair, error) {
	// Access token
	accessExpiry := time.Now().Add(j.accessDuration)
	accessClaims := AccessClaims{
		AdminID: AdminId.String(),
		OrgID:   OrgId.String(),
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   AdminId.String(),
			Issuer:    j.issuer,
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(j.accessKey))
	
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth.issueTokenPair: sign access token: %w", err)
	}

	// Refresh token — opaque, stored hashed
	rawRefresh, err := GenerateSecureToken()
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth.issueTokenPair: generate refresh token: %w", err)
	}

	refreshExpiry := time.Now().Add(j.refreshDuration)
	

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		AccessExpiresAt:    accessExpiry,
		RefreshExpiresAt:   refreshExpiry,
	}, nil
}


func (j *JwtUtils) IssueSetupToken(ctx context.Context, adminID uuid.UUID) (string, error) {
	rawToken, err := GenerateSecureToken()
	if err != nil {
		return "", fmt.Errorf("auth.issueSetupToken: %w", err)
	}
	return rawToken, nil
}

func (s *JwtUtils) ParseOTPToken(tokenStr string) (*OTPClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &OTPClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.accessKey), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("%s", err.Error())
	}

	claims, ok := token.Claims.(*OTPClaims)
	if !ok {
		return nil, errors.New("auth: token is invalid or expired")
	}

	return claims, nil
}



// generateSecureToken produces a cryptographically random URL-safe token.
func GenerateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}



// hashToken SHA-256 hashes a raw token before storing in the DB.
// We never store raw tokens — if the DB is compromised, tokens are useless.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
