package oidc

import (
	"encoding/json"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
)

func GooglePreset(tenantID, clientID, clientSecret, hostedDomain string) *identity.IdentityProvider {
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_google",
		DisplayName: "Google",
		Enabled:     true,
		IssuerURL:   "https://accounts.google.com",
		ClientID:    clientID,
		Scopes:      []string{"openid", "email", "profile"},
		EmailClaim:  "email",
		NameClaim:   "name",
		ExtraConfig: json.RawMessage(
			fmt.Sprintf(`{"hosted_domain": "%s"}`, hostedDomain),
		),
	}
}

func EntraPreset(tenantID, entraTenantID, clientID, clientSecret string) *identity.IdentityProvider {
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_entra",
		DisplayName: "Microsoft Entra ID",
		Enabled:     true,
		IssuerURL:   fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", entraTenantID),
		ClientID:    clientID,
		Scopes:      []string{"openid", "email", "profile"},
		EmailClaim:  "preferred_username",
		NameClaim:   "name",
		GroupsClaim: "groups",
		ExtraConfig: json.RawMessage(
			fmt.Sprintf(`{"graph_tenant_id": "%s", "graph_client_id": "%s", "graph_secret_enc": "%s"}`,
				entraTenantID,
				clientID,
				clientSecret,
			),
		),
	}
}

func OktaPreset(tenantID, oktaDomain, clientID, clientSecret, authServerID, oktaAPIToken string) *identity.IdentityProvider {
	issuer := fmt.Sprintf("https://%s/oauth2/default", oktaDomain)
	if authServerID != "" {
		issuer = fmt.Sprintf("https://%s/oauth2/%s", oktaDomain, authServerID)
	}
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_okta",
		DisplayName: "Okta",
		Enabled:     true,
		// OIDCConfig: identity.OIDCProviderConfig{
		IssuerURL:   issuer,
		ClientID:    clientID,
		Scopes:      []string{"openid", "email", "profile", "groups"},
		EmailClaim:  "email",
		NameClaim:   "name",
		GroupsClaim: "groups",
		ExtraConfig: json.RawMessage(
			fmt.Sprintf(`{"domain": "%s", "apiToken": "%s"}`,
				oktaDomain,
				oktaAPIToken,
			),
		),
	}
}

func KeycloakPreset(tenantID, baseURL, realm, clientID, clientSecret, adminUser, adminPass string) *identity.IdentityProvider {
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_keycloak",
		DisplayName: "Keycloak",
		Enabled:     true,
		// OIDCConfig: identity.OIDCProviderConfig{
			IssuerURL:   fmt.Sprintf("%s/realms/%s", baseURL, realm),
			ClientID:    clientID,
			Scopes:      []string{"openid", "email", "profile", "groups"},
			EmailClaim:  "email",
			NameClaim:   "name",
			GroupsClaim: "groups",
			ExtraConfig: json.RawMessage(
				fmt.Sprintf(`{"admin_url": "%s", "realm": "%s", "admin_user": "%s", "admin_pass": "%s"}`,
					baseURL,
					realm,
					adminUser,
					adminPass,
				),
			),
	}
}

func GenericOIDCPreset(
	tenantID, issuerURL, clientID, clientSecret string,
	emailClaim, nameClaim, groupsClaim string,
	extraClaims json.RawMessage,
) *identity.IdentityProvider {
	
	
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_generic",
		DisplayName: "Custom OIDC Provider",
		Enabled:     true,
		IssuerURL: issuerURL,
		ClientID: clientID,
		ClientSecretEnc: clientSecret,
		Scopes: []string{"openid", "email", "profile"},
		EmailClaim: emailClaim,
		NameClaim: nameClaim,
		GroupsClaim: groupsClaim,
		ExtraConfig: extraClaims,
	}
}
