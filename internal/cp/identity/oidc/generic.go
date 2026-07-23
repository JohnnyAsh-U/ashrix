// internal/identity/oidc/generic.go — same shape, exists so
// "unknown/custom OIDC provider" has an explicit, honest home
// rather than being silently forced through one of the named presets
package oidc

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
)

type genericAdapter struct {
	base *baseOIDCClient
	cfg  *identity.IdentityProvider
}

func newGenericAdapter(base *baseOIDCClient, cfg *identity.IdentityProvider) (*genericAdapter, error) {
	return &genericAdapter{base: base, cfg: cfg}, nil
}

func (a *genericAdapter) AuthCodeURL(state, nonce string) string {
	return a.base.authCodeURL(state, nonce)
}

func (a *genericAdapter) Exchange(ctx context.Context, code, expectedNonce string) (*identity.NormalizedIdentity, error) {
	claims, err := a.base.exchangeAndVerify(ctx, code, expectedNonce)
	if err != nil {
		return nil, err
	}
	email, name, sub, groups, err := a.base.extractCommonFields(claims)
	if err != nil {
		return nil, err
	}
	return &identity.NormalizedIdentity{
		TenantID: a.cfg.TenantID, ProviderID: a.cfg.ID,
		UserID: sub, Email: email, Name: name, Groups: groups,
		Provider: a.cfg.Type, AuthTime: time.Now(),
	}, nil
}


func (a *genericAdapter) SearchUsers(ctx context.Context, query string, limit int) ([]identity.User, error) {
	return nil, fmt.Errorf("user search not supported for generic provider")
}

func (a *genericAdapter) SearchGroups(ctx context.Context, query string, limit int) ([]identity.Group, error) {
	return nil, fmt.Errorf("group search not supported for generic provider")
}

func (a *genericAdapter) GetUserGroups(ctx context.Context, userID string) ([]string, error) {
	return nil, fmt.Errorf("get user groups not supported for generic provider")
}
func (a *genericAdapter) GetProviderType() string {
	return a.base.GetProviderType()
}