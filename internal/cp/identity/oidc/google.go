// internal/identity/oidc/google.go
package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

type googleExtraConfig struct {
	HostedDomain string `json:"hosted_domain,omitempty"`
}

type googleAdapter struct {
	base  *baseOIDCClient
	cfg   *IdentityProvider
	extra googleExtraConfig
}

func newGoogleAdapter(base *baseOIDCClient, cfg *IdentityProvider) (*googleAdapter, error) {
	var extra googleExtraConfig
	if len(cfg.ExtraConfig) > 0 {
		if err := json.Unmarshal(cfg.ExtraConfig, &extra); err != nil {
			return nil, fmt.Errorf("google extra_config invalid: %w", err)
		}
	}
	return &googleAdapter{base: base, cfg: cfg, extra: extra}, nil
}

func (a *googleAdapter) AuthCodeURL(state, nonce, code_challenge string) string {
	var opts []oauth2.AuthCodeOption
	if a.extra.HostedDomain != "" {
		// Google-specific hint — restricts the account picker to
		// a workspace domain. This is a UX nicety, NOT a security
		// boundary — the actual enforcement happens below, in
		// Exchange, by checking the hd claim on the returned token.
		// A user could bypass the hint by navigating to a different
		// Google login URL manually, which is exactly why the
		// claim check exists as the real gate, not this hint alone.
		opts = append(opts, oauth2.SetAuthURLParam("hd", a.extra.HostedDomain))
	}
	return a.base.authCodeURL(state, nonce, code_challenge, opts...)
}

func (a *googleAdapter) Exchange(ctx context.Context, code, expectedNonce, verifier string) (*NormalizedIdentity, error) {
	claims, err := a.base.exchangeAndVerify(ctx, code, expectedNonce, verifier)
	if err != nil {
		return nil, err
	}

	email, name, sub, _, err := a.base.extractCommonFields(claims)
	if err != nil {
		return nil, err
	}

	// THE actual enforcement — checked against the verified token
	// claim, not the redirect hint above.
	if a.extra.HostedDomain != "" {
		hd, _ := claims["hd"].(string)
		if hd != a.extra.HostedDomain {
			return nil, fmt.Errorf("hosted domain mismatch: expected %q, got %q", a.extra.HostedDomain, hd)
		}
	}

	// Google does not provide groups via OIDC ID token at all.
	// Returning nil here is deliberate and honest — NOT a bug,
	// NOT a TODO. A tenant relying solely on Google as IdP cannot
	// do group-based ABAC policy without a SEPARATE Workspace
	// Admin SDK integration, which is out of scope here and should
	// be a documented limitation, not a silent gap someone
	// discovers by confused debugging later.
	return &NormalizedIdentity{
		TenantID:   a.cfg.TenantID,
		ProviderID: a.cfg.ID,
		UserID:     sub,
		Email:      email,
		Name:       name,
		Groups:     nil,
		Provider:   a.cfg.Type,
		AuthTime:   time.Now(),
	}, nil
}


func (a *googleAdapter) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	return nil, nil
}
func (a *googleAdapter) SearchGroups(ctx context.Context, query string, limit int) ([]Group, error) {
	return nil, nil
}
func (a *googleAdapter) GetUserGroups(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (a *googleAdapter) GetProviderType() string {
	return a.base.GetProviderType()
}

func (b *googleAdapter) GetProviderID() string {
	return b.base.GetProviderID()
}
