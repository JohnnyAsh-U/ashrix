package oidc

import (
	"encoding/json"
	"fmt"
)

func GooglePreset(tenantID, clientID, clientSecret string, extraConfig json.RawMessage) *IdentityProvider {

	var ExtraConfig googleExtraConfig
	if len(extraConfig) > 0 {
		_ = json.Unmarshal(extraConfig, &ExtraConfig)
	}

	return &IdentityProvider{
		TenantID:    tenantID,
		Type:        "google",
		DisplayName: "Google",
		Enabled:     true,
		ClientSecretEnc: clientSecret,
		IssuerURL:   "https://accounts.google.com",
		ClientID:    clientID,
		Scopes:      []string{"openid", "email", "profile"},
		EmailClaim:  "email",
		NameClaim:   "name",
		ExtraConfig: json.RawMessage(
			fmt.Sprintf(`{"hosted_domain": "%s"}`, ExtraConfig.HostedDomain),
		),
	}
}

func EntraPreset(tenantID, clientID, clientSecret string, extraConfig json.RawMessage) *IdentityProvider {
	var ExtraConfig entraExtraConfig
	if len(extraConfig) > 0 {
		_ = json.Unmarshal(extraConfig, &ExtraConfig)
	}
	return &IdentityProvider{
		TenantID:    tenantID,
		Type:        "entra",
		DisplayName: "Microsoft Entra ID",
		Enabled:     true,
		ClientSecretEnc: clientSecret,
		IssuerURL:   fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", ExtraConfig.GraphTenantID),
		ClientID:    clientID,
		Scopes:      []string{"openid", "email", "profile"},
		EmailClaim:  "preferred_username",
		NameClaim:   "name",
		GroupsClaim: "groups",
		ExtraConfig: json.RawMessage(
			fmt.Sprintf(`{"graph_tenant_id": "%s", "graph_client_id": "%s", "graph_secret_enc": "%s"}`,
				ExtraConfig.GraphTenantID,
				ExtraConfig.GraphClientID,
				ExtraConfig.GraphSecretEnc,
			),
		),
	}
}

func OktaPreset(tenantID, clientID, clientSecret string, extraConfig json.RawMessage) *IdentityProvider {

	var ExtraConfig extraOktaConfig
	if len(extraConfig) > 0 {
		_ = json.Unmarshal(extraConfig, &ExtraConfig)
	}

	issuer := fmt.Sprintf("https://%s/oauth2/default", ExtraConfig.Domain)
	if ExtraConfig.AuthServerID != "" {
		issuer = fmt.Sprintf("https://%s/oauth2/%s", ExtraConfig.Domain, ExtraConfig.AuthServerID)
	}
	return &IdentityProvider{
		TenantID:    tenantID,
		Type:        "okta",
		DisplayName: "Okta",
		Enabled:     true,
		ClientSecretEnc: clientSecret,
		// OIDCConfig: OIDCProviderConfig{
		IssuerURL:   issuer,
		ClientID:    clientID,
		Scopes:      []string{"openid", "email", "profile", "groups"},
		EmailClaim:  "email",
		NameClaim:   "name",
		GroupsClaim: "groups",
		ExtraConfig: json.RawMessage(
			fmt.Sprintf(`{"domain": "%s", "apiToken": "%s"}`,
				ExtraConfig.Domain,
				ExtraConfig.APIToken,
			),
		),
	}
}

func KeycloakPreset(tenantID, clientID, clientSecret string, extraConfig json.RawMessage) *IdentityProvider {
		var ExtraConfig KeycloakExtraConfig
	if len(extraConfig) > 0 {
		_ = json.Unmarshal(extraConfig, &ExtraConfig)
	}

	return &IdentityProvider{
		TenantID:    tenantID,
		Type:        "keycloak",
		DisplayName: "Keycloak",
		Enabled:     true,
		// OIDCConfig: OIDCProviderConfig{
		IssuerURL:   fmt.Sprintf("%s/realms/%s", ExtraConfig.AdminURL, ExtraConfig.Realm),
		ClientSecretEnc: clientSecret,
		ClientID:    clientID,
		Scopes:      []string{"openid", "email", "profile", "groups"},
		EmailClaim:  "email",
		NameClaim:   "name",
		GroupsClaim: "groups",
		ExtraConfig: json.RawMessage(
			fmt.Sprintf(`{"admin_url": "%s", "realm": "%s", "admin_user": "%s", "admin_pass": "%s"}`,
				ExtraConfig.AdminURL,
				ExtraConfig.Realm,
				ExtraConfig.AdminUser,
				ExtraConfig.AdminPass,
			),
		),
	}
}

func GenericOIDCPreset(
	tenantID, issuerURL, clientID, clientSecret string,
	emailClaim, nameClaim, groupsClaim string,
	extraClaims json.RawMessage,
) *IdentityProvider {

	return &IdentityProvider{
		TenantID:        tenantID,
		Type:            "generic",
		DisplayName:     "Custom OIDC Provider",
		Enabled:         true,
		IssuerURL:       issuerURL,
		ClientSecretEnc: clientSecret,
		ClientID:        clientID,
		Scopes:          []string{"openid", "email", "profile"},
		EmailClaim:      emailClaim,
		NameClaim:       nameClaim,
		GroupsClaim:     groupsClaim,
		ExtraConfig:     extraClaims,
	}
}
