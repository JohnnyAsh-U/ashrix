// internal/identity/oidc/base_client.go
package oidc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
	oidclib "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// baseOIDCClient holds the parts of an OIDC integration that are
// genuinely identical across every spec-compliant provider:
// discovery, JWKS-backed verification, code exchange. This is
// composed INTO each adapter, not inherited — Go has no
// inheritance, and composition is the correct fit here anyway,
// since adapters need to override/extend behavior, not just reuse it.
type baseOIDCClient struct {
	cfg          *IdentityProvider
	provider     *oidclib.Provider
	oauth2Config oauth2.Config
	httpClient *http.Client
	verifier     *oidclib.IDTokenVerifier
}

func newBaseClient(ctx context.Context, cfg *IdentityProvider, clientSecret string) (*baseOIDCClient, error) {
	provider, err := oidclib.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed for %s: %w", cfg.IssuerURL, err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
	}

	verifier := provider.Verifier(&oidclib.Config{ClientID: cfg.ClientID})

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &baseOIDCClient{
		cfg:          cfg,
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
		httpClient: httpClient,
	}, nil
}

func (b *baseOIDCClient) authCodeURL(state, nonce string, extra ...oauth2.AuthCodeOption) string {
	opts := append([]oauth2.AuthCodeOption{oidclib.Nonce(nonce)}, extra...)
	return b.oauth2Config.AuthCodeURL(state, opts...)
}


func (b *baseOIDCClient) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	return nil, nil
}
func (b *baseOIDCClient) SearchGroups(ctx context.Context, query string, limit int) ([]Group, error) {
	return nil, nil
}
func (b *baseOIDCClient) GetUserGroups(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}
func (b *baseOIDCClient) GetProviderID() string {
	return b.cfg.ID
}


// exchangeAndVerify does the genuinely universal part: code → token
// → verified claims map. Returns raw claims — each adapter's own
// mapClaims takes it from there, since THAT part is where provider
// differences actually live.
func (b *baseOIDCClient) exchangeAndVerify(ctx context.Context, code, expectedNonce string) (map[string]interface{}, error) {
	token, err := b.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := b.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification failed: %w", err)
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("claims parse failed: %w", err)
	}

	if nonceClaim, _ := claims["nonce"].(string); nonceClaim != expectedNonce {
		return nil, fmt.Errorf("nonce mismatch — possible token substitution")
	}

	return claims, nil
}

// extractCommonFields pulls the fields every provider config
// declares via named columns (EmailClaim, NameClaim, GroupsClaim)
// — shared logic, since the CLAIM NAMES differ per provider but
// the EXTRACTION MECHANISM does not.
func (b *baseOIDCClient) extractCommonFields(claims map[string]interface{}) (email, name, sub string, groups []string, err error) {
	email, _ = claims[b.cfg.EmailClaim].(string)
	if email == "" {
		return "", "", "", nil, fmt.Errorf("email claim %q missing or empty", b.cfg.EmailClaim)
	}
	name, _ = claims[b.cfg.NameClaim].(string)
	sub, _ = claims["sub"].(string)

	if b.cfg.GroupsClaim != "" {
		if raw, ok := claims[b.cfg.GroupsClaim].([]interface{}); ok {
			for _, g := range raw {
				if s, ok := g.(string); ok {
					groups = append(groups, s)
				}
			}
		}
	}
	return email, name, sub, groups, nil
}

func (b *baseOIDCClient) GetProviderType() string {
	return b.cfg.Type
}