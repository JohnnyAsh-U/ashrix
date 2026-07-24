package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type keycloakAdapter struct {
	base *baseOIDCClient
	cfg  *IdentityProvider
	extra KeycloakExtraConfig
}

type KeycloakExtraConfig struct {
    AdminURL    string `json:"admin_url"`
    Realm       string `json:"realm"`
    AdminUser   string `json:"admin_user"`
    AdminPass   string `json:"admin_pass"`
}


func newKeycloakAdapter(base *baseOIDCClient, cfg *IdentityProvider) (*keycloakAdapter, error) {
	
    // var extra KeycloakExtraConfig
	// if len(cfg.ExtraConfig) > 0 {
	// 	if err := json.Unmarshal(cfg.ExtraConfig, &extra); err != nil {
	// 		return nil, fmt.Errorf("keycloak extra_config invalid: %w", err)
	// 	}
	// }
	return &keycloakAdapter{base: base, cfg: cfg, extra: KeycloakExtraConfig{}}, nil
}

func (a *keycloakAdapter) AuthCodeURL(state, nonce string) string {
	return a.base.authCodeURL(state, nonce)
}

func (a *keycloakAdapter) Exchange(ctx context.Context, code, expectedNonce string) (*NormalizedIdentity, error) {
	claims, err := a.base.exchangeAndVerify(ctx, code, expectedNonce)
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


func (a *keycloakAdapter) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
    token, err := a.getAdminToken(ctx)
    if err != nil {
        return nil, err
    }

    searchURL := fmt.Sprintf("%s/admin/realms/%s/users", a.extra.AdminURL, a.extra.Realm)
    params := url.Values{}
    params.Add("search", query)
    params.Add("max", fmt.Sprint(limit))

    req, _ := http.NewRequestWithContext(ctx, "GET", searchURL+"?"+params.Encode(), nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Accept", "application/json")

    resp, err := a.base.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var keycloakUsers []struct {
        ID       string `json:"id"`
        Username string `json:"username"`
        Email    string `json:"email"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&keycloakUsers); err != nil {
        return nil, err
    }

    users := make([]User, len(keycloakUsers))
    for i, u := range keycloakUsers {
        users[i] = User{
            ID:       u.ID,
            Email:    u.Email,
            Username: u.Username,
        }
    }

    return users, nil
}

func (a *keycloakAdapter) SearchGroups(ctx context.Context, query string, limit int) ([]Group, error) {
    token, err := a.getAdminToken(ctx)
    if err != nil {
        return nil, err
    }

    searchURL := fmt.Sprintf("%s/admin/realms/%s/groups", a.extra.AdminURL, a.extra.Realm)
    params := url.Values{}
    params.Add("search", query)
    params.Add("max", fmt.Sprint(limit))

    req, _ := http.NewRequestWithContext(ctx, "GET", searchURL+"?"+params.Encode(), nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Accept", "application/json")

    resp, err := a.base.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var keycloakGroups []struct {
        ID   string `json:"id"`
        Name string `json:"name"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&keycloakGroups); err != nil {
        return nil, err
    }

    groups := make([]Group, len(keycloakGroups))
    for i, g := range keycloakGroups {
        groups[i] = Group{
            ID:   g.ID,
            Name: g.Name,
        }
    }

    return groups, nil
}

func (a *keycloakAdapter) GetUserGroups(ctx context.Context, userID string) ([]string, error) {
    token, err := a.getAdminToken(ctx)
    if err != nil {
        return nil, err
    }

    url := fmt.Sprintf("%s/admin/realms/%s/users/%s/groups", a.extra.AdminURL, a.extra.Realm, userID)

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Accept", "application/json")

    resp, err := a.base.httpClient.Do(req)
    if err != nil {
        return nil, err
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


func (a *keycloakAdapter) getAdminToken(ctx context.Context) (string, error) {
    tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", a.extra.AdminURL)

    data := url.Values{}
    data.Set("client_id", "admin-cli")
    data.Set("username", a.extra.AdminUser)
    data.Set("password", a.extra.AdminPass)
    data.Set("grant_type", "password")

    resp, err := a.base.httpClient.PostForm(tokenURL, data)
    if err != nil {
        return "", fmt.Errorf("failed to get admin token: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        AccessToken string `json:"access_token"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }

    return result.AccessToken, nil
}


func (a *keycloakAdapter) GetProviderType() string {
    return a.base.GetProviderType()
}

func (a *keycloakAdapter) GetProviderID() string {
	return a.base.GetProviderID()
}
