package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/graph"
)



type entraExtraConfig struct {
	GraphTenantID  string `json:"graph_tenant_id"`
	GraphClientID  string `json:"graph_client_id"`
	GraphSecretEnc string `json:"graph_secret_enc"` // caller decrypts
	// before constructing
	// this adapter — see
	// factory wiring note
}

type entraAdapter struct {
	base        *baseOIDCClient
	cfg         *IdentityProvider
	graphClient *graph.EntraClient // nil if Graph resolution not configured
}






func newEntraAdapter(base *baseOIDCClient, cfg *IdentityProvider) (*entraAdapter, error) {
	var extra entraExtraConfig
	if len(cfg.ExtraConfig) > 0 {
		if err := json.Unmarshal(cfg.ExtraConfig, &extra); err != nil {
			return nil, fmt.Errorf("entra extra_config invalid: %w", err)
		}
	}

	var graphClient *graph.EntraClient
	if extra.GraphTenantID != "" {
		// Note: GraphSecretEnc arrives already encrypted in
		// ExtraConfig. Decryption happens at the STORE layer
		// (Store.GetProvider equivalent), same pattern
		// as ClientSecretEnc — the adapter factory should receive
		// already-decrypted secrets, never handle encryption
		// itself. Flagged: the wiring in the store layer (below)
		// must decrypt this field the same way it decrypts
		// ClientSecretEnc, which the earlier store.go did not
		// yet do for extra_config sub-fields — real gap, fixed
		// in store.go below.
		graphClient = graph.NewEntraClient(extra.GraphTenantID, extra.GraphClientID, extra.GraphSecretEnc)
	}

	return &entraAdapter{base: base, cfg: cfg, graphClient: graphClient}, nil
}

func (a *entraAdapter) AuthCodeURL(state, nonce, code_challenge string) string {
	return a.base.authCodeURL(state, nonce, code_challenge)
}

func (a *entraAdapter) Exchange(ctx context.Context, code, expectedNonce,verifier string) (*NormalizedIdentity, error) {
	claims, err := a.base.exchangeAndVerify(ctx, code, expectedNonce, verifier)
	if err != nil {
		return nil, err
	}

	// Entra-specific: email is unreliable in the standard `email`
	// claim, preferred_username is the documented, reliable field.
	// This is handled by the config's EmailClaim already being set
	// to "preferred_username" at provider-creation time — the
	// adapter itself doesn't need a special case, the COLUMN VALUE
	// carries the difference. Worth naming why this works cleanly:
	// the common-field extraction mechanism plus per-provider
	// CONFIGURATION (not code) already handles this case.
	email, name, sub, groups, err := a.base.extractCommonFields(claims)
	if err != nil {
		return nil, err
	}

	// Entra returns group membership as GUIDs, not names. Resolving
	// them requires a SEPARATE Graph API call with its own
	// credentials — genuine additional integration surface, not
	// something "generic OIDC support" can paper over.
	if a.graphClient != nil && len(groups) > 0 {
		resolved, err := a.graphClient.ResolveGroupNames(ctx, groups)
		if err != nil {
			// Fail closed: a user with unresolved groups gets an
			// error, not silently empty groups (which would look
			// like "no group memberships" to policy evaluation —
			// dangerous if any ABAC rule treats absence of a
			// required group as deny, since it could ALSO mean
			// "we couldn't check" rather than "genuinely has none").
			return nil, fmt.Errorf("entra group resolution failed: %w", err)
		}
		groups = resolved
	}

	return &NormalizedIdentity{
		TenantID:   a.cfg.TenantID,
		ProviderID: a.cfg.ID,
		UserID:     sub,
		Email:      email,
		Name:       name,
		Groups:     groups,
		Provider:   a.cfg.Type,
		AuthTime:   time.Now(),
	}, nil
}



func (a *entraAdapter) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	token, err := a.getGraphToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph token: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}

	graphURL := "https://graph.microsoft.com/v1.0/users"
	params := url.Values{}
	params.Add("$filter", fmt.Sprintf("startswith(displayName,'%s') or startswith(userPrincipalName,'%s')", query, query))
	params.Add("$top", fmt.Sprint(limit))
	params.Add("$select", "id,displayName,userPrincipalName,mail")

	req, err := http.NewRequestWithContext(ctx, "GET", graphURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := a.base.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph API error: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Value []struct {
			ID                string `json:"id"`
			DisplayName       string `json:"displayName"`
			UserPrincipalName string `json:"userPrincipalName"`
			Mail              string `json:"mail"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	users := make([]User, len(result.Value))
	for i, u := range result.Value {
		email := u.Mail
		if email == "" {
			email = u.UserPrincipalName
		}
		users[i] = User{
			ID:       u.ID,
			Email:    email,
			Username: u.DisplayName,
		}
	}

	return users, nil
}

func (a *entraAdapter) SearchGroups(ctx context.Context, query string, limit int) ([]Group, error) {
	token, err := a.getGraphToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph token: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}

	graphURL := "https://graph.microsoft.com/v1.0/groups"
	params := url.Values{}
	params.Add("$filter", fmt.Sprintf("startswith(displayName,'%s')", query))
	params.Add("$top", fmt.Sprint(limit))
	params.Add("$select", "id,displayName,description")

	req, _ := http.NewRequestWithContext(ctx, "GET", graphURL+"?"+params.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := a.base.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Value []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	groups := make([]Group, len(result.Value))
	for i, g := range result.Value {
		groups[i] = Group{
			ID:          g.ID,
			Name:        g.DisplayName,
			Description: g.Description,
		}
	}

	return groups, nil
}

func (a *entraAdapter) GetUserGroups(ctx context.Context, userID string) ([]string, error) {
	token, err := a.getGraphToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/memberOf", userID)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := a.base.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Value []struct {
			ID string `json:"id"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	groupIDs := make([]string, len(result.Value))
	for i, g := range result.Value {
		groupIDs[i] = g.ID
	}

	return groupIDs, nil
}

func (a *entraAdapter) getGraphToken(ctx context.Context) (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", a.base.cfg.TenantID)

	data := url.Values{}
	data.Set("client_id", a.base.oauth2Config.ClientID)
	data.Set("client_secret", a.base.oauth2Config.ClientSecret)
	data.Set("scope", "https://graph.microsoft.com/.default")
	data.Set("grant_type", "client_credentials")

	resp, err := a.base.httpClient.PostForm(tokenURL, data)
	if err != nil {
		return "", err
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

func (a *entraAdapter) GetProviderType() string {
	return a.base.GetProviderType()
}

func (b *entraAdapter) GetProviderID() string {
	return b.base.GetProviderID()
}

