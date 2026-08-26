package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Okta needs no adapter-specific behavior beyond the base client —
// it's genuinely the most spec-compliant of the four. The adapter
// exists anyway to satisfy the ProviderAdapter interface uniformly
// and to leave an obvious place for future Okta-specific quirks
// without disturbing the factory or other adapters.
type oktaAdapter struct {
	base  *baseOIDCClient
	cfg   *IdentityProvider
	extra extraOktaConfig
}

type extraOktaConfig struct {
	Domain     string `json:"domain"`
	APIToken string `json:"apiToken"`
    AuthServerID string `json:"authServerId"`
}

func newOktaAdapter(base *baseOIDCClient, cfg *IdentityProvider) (*oktaAdapter, error) {
	var extra extraOktaConfig
	if len(cfg.ExtraConfig) > 0 {
		if err := json.Unmarshal(cfg.ExtraConfig, &extra); err != nil {
			return nil, fmt.Errorf("okta extra_config invalid: %w", err)
		}
	}
	return &oktaAdapter{base: base, cfg: cfg, extra: extra}, nil
}

func (a *oktaAdapter) AuthCodeURL(state, nonce,code_challenge string) string {
	return a.base.authCodeURL(state, nonce, code_challenge)
}

func (a *oktaAdapter) Exchange(ctx context.Context, code, expectedNonce, verifier string) (*NormalizedIdentity, error) {
	claims, err := a.base.exchangeAndVerify(ctx, code, expectedNonce, verifier)
	if err != nil {
		return nil, err
	}
	email, name, sub, groups, err := a.base.extractCommonFields(claims)
	if err != nil {
		return nil, err
	}
	return &NormalizedIdentity{
		TenantID: a.cfg.TenantID, ProviderID: a.cfg.ID,
		UserID: sub, Email: email, Name: name, Groups: groups,
		Provider: a.cfg.Type, AuthTime: time.Now(),
	}, nil
}

func (a *oktaAdapter) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
    if limit <= 0 {
        limit = 20
    }

    searchURL := fmt.Sprintf("https://%s/api/v1/users", a.extra.Domain)
    params := url.Values{}
    params.Add("search", fmt.Sprintf("profile.email sw %q or profile.firstName sw %q or profile.lastName sw %q", query, query, query))
    params.Add("limit", fmt.Sprint(limit))

    req, err := http.NewRequestWithContext(ctx, "GET", searchURL+"?"+params.Encode(), nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "SSWS "+a.extra.APIToken)
    req.Header.Set("Accept", "application/json")

    resp, err := a.base.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("okta user search failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("okta API error: %d", resp.StatusCode)
    }

    var oktaUsers []struct {
        ID       string `json:"id"`
        Profile  struct {
            Email     string `json:"email"`
            FirstName string `json:"firstName"`
            LastName  string `json:"lastName"`
        } `json:"profile"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&oktaUsers); err != nil {
        return nil, err
    }

    users := make([]User, len(oktaUsers))
    for i, u := range oktaUsers {
        users[i] = User{
            ID:       u.ID,
            Email:    u.Profile.Email,
            Username: fmt.Sprintf("%s %s", u.Profile.FirstName, u.Profile.LastName),
        }
    }

    return users, nil
}

func (a *oktaAdapter) SearchGroups(ctx context.Context, query string, limit int) ([]Group, error) {
    if limit <= 0 {
        limit = 20
    }

    searchURL := fmt.Sprintf("https://%s/api/v1/groups", a.extra.Domain)
    params := url.Values{}
    params.Add("q", query)
    params.Add("limit", fmt.Sprint(limit))

    req, err := http.NewRequestWithContext(ctx, "GET", searchURL+"?"+params.Encode(), nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "SSWS "+a.extra.APIToken)
    req.Header.Set("Accept", "application/json")

    resp, err := a.base.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("okta group search failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("okta API error: %d", resp.StatusCode)
    }

    var oktaGroups []struct {
        ID   string `json:"id"`
        Profile struct {
            Name        string `json:"name"`
            Description string `json:"description"`
        } `json:"profile"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&oktaGroups); err != nil {
        return nil, err
    }

    groups := make([]Group, len(oktaGroups))
    for i, g := range oktaGroups {
        groups[i] = Group{
            ID:          g.ID,
            Name:        g.Profile.Name,
            Description: g.Profile.Description,
        }
    }

    return groups, nil
}

func (a *oktaAdapter) GetUserGroups(ctx context.Context, userID string) ([]string, error) {
    url := fmt.Sprintf("https://%s/api/v1/users/%s/groups", a.extra.Domain, userID)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "SSWS "+a.extra.APIToken)
    req.Header.Set("Accept", "application/json")

    resp, err := a.base.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to get user groups: %w", err)
    }
    defer resp.Body.Close()

    var groups []struct {
        ID string `json:"id"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
        return nil, err
    }

    groupIDs := make([]string, len(groups))
    for i, g := range groups {
        groupIDs[i] = g.ID
    }

    return groupIDs, nil
}

func (a *oktaAdapter) GetProviderType() string {
    return a.base.GetProviderType()
}

func (a *oktaAdapter) GetProviderID() string {
	return a.base.GetProviderID()
}

