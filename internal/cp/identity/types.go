package identity

import (
	"encoding/json"
	"time"
)

type IdentityProvider struct {
	ID          string
	TenantID    string
	Type        string
	DisplayName string
	Enabled     bool
	CreatedAt   time.Time

	IssuerURL       string
	ClientID        string
	ClientSecretEnc string
	Scopes          []string
	EmailClaim      string
	NameClaim       string
	GroupsClaim     string

	// Provider-specific fields that don't warrant their own
	// column, per the extra_config decision from last message.
	// Each adapter parses only the keys it expects.
	ExtraConfig json.RawMessage
}

type NormalizedIdentity struct {
	TenantID   string
	ProviderID string
	UserID     string
	Email      string
	Name       string
	Groups     []string
	Provider   string
	AuthTime   time.Time
}





type IdPConfig struct {
    ID           string                 `json:"id"`
    OrgID        string                 `json:"org_id"`
    ProviderType string                 `json:"provider_type"`
    BaseConfig   BaseConfig             `json:"base_config"`
    ExtraConfig  map[string]interface{} `json:"extra_config"`
}

type BaseConfig struct {
    ClientID     string   `json:"client_id"`
    ClientSecret string   `json:"client_secret"`
    IssuerURL    string   `json:"issuer_url"`
    Scopes       []string `json:"scopes"`
    RedirectURI  string   `json:"redirect_uri"`
}

type User struct {
    ID       string `json:"id"`
    Email    string `json:"email"`
    Username string `json:"username"`
}

type Group struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
}
