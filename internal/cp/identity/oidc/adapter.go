// internal/identity/oidc/adapter.go
package oidc

import (
	"context"
	"fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
)

// ProviderAdapter is what each IdP "flavor" implements. All
// provider-specific behavior lives behind this interface —
// nothing calling an adapter needs to know or branch on which
// concrete provider it's talking to.
type ProviderAdapter interface {
	// AuthCodeURL builds the redirect URL to the IdP. Some
	// providers need extra params beyond the OIDC standard
	// (Google's hd hint for domain restriction, Entra's
	// domain_hint) — each adapter adds its own.
	AuthCodeURL(state, nonce string) string

	// Exchange trades the authorization code for tokens,
	// verifies the ID token, and normalizes claims into the
	// ONE shape everything downstream consumes.
	Exchange(ctx context.Context, code, expectedNonce string) (*identity.NormalizedIdentity, error)

	GetProviderType() string

	SearchUsers(ctx context.Context, query string, limit int) ([]identity.User, error)
	SearchGroups(ctx context.Context, query string, limit int) ([]identity.Group, error)
	GetUserGroups(ctx context.Context, userID string) ([]string, error)
}


// NewAdapter is the Factory Method: one place that decides which
// concrete adapter to construct based on IdentityProvider.Type.
// Adding a new provider means adding one case here plus one new
// adapter type — nothing else in the codebase needs to change.
func NewAdapter(ctx context.Context, cfg *identity.IdentityProvider, clientSecret string) (ProviderAdapter, error) {
	base, err := newBaseClient(ctx, cfg, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("base OIDC client init failed: %w", err)
	}

	switch cfg.Type {
	case "oidc_google":
		return newGoogleAdapter(base, cfg)
	case "oidc_entra":
		return newEntraAdapter(base, cfg)
	case "oidc_okta":
		return newOktaAdapter(base, cfg)
	case "oidc_keycloak":
		return newKeycloakAdapter(base, cfg)
	case "oidc_generic":
		return newGenericAdapter(base, cfg)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}

type UserInfo struct {
	Sub    string   `json:"sub"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}
