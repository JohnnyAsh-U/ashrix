// broker/internal/broker/session.go
package broker

import (
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
	"github.com/golang-jwt/jwt/v5"
)

type BrokerClaims struct {
	jwt.RegisteredClaims
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Groups   []string `json:"groups"`
	Provider string   `json:"provider"`
}

type TokenIssuer struct {
	signingKey *ecdsa.PrivateKey
	issuer     string
	ttl        time.Duration
}

func NewTokenIssuer(signingKey *ecdsa.PrivateKey, issuer string) *TokenIssuer {
	return &TokenIssuer{signingKey: signingKey, issuer: issuer, ttl: 15 * time.Minute}
}

func (t *TokenIssuer) Issue(identity *identity.NormalizedIdentity) (string, error) {
	claims := BrokerClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   identity.UserID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(t.ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		TenantID: identity.TenantID,
		UserID:   identity.UserID,
		Email:    identity.Email,
		Name:     identity.Name,
		Groups:   identity.Groups,
		Provider: identity.Provider,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	signed, err := token.SignedString(t.signingKey)
	if err != nil {
		return "", fmt.Errorf("sign broker token: %w", err)
	}
	return signed, nil
}


//For Gateway to verify; to be moved to gateway
func (t *TokenIssuer) Verify(tokenString string) (*BrokerClaims, error) {
	claims := &BrokerClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(tok *jwt.Token) (interface{}, error) {
		return &t.signingKey.PublicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("verify session token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid session token")
	}
	return claims, nil
}
// PublicKeyPEM should be exposed via a JWKS-style endpoint so
// Ashrix CP (and anything else trusting broker tokens) can verify
// signatures without a shared secret — same pattern as your CP's
// own embedded-public-key model from the PKI architecture.