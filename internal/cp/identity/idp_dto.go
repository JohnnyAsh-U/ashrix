package identity

import (
	"encoding/json"
	// "time"
)

type CreateIDPConfig struct {
	Type            string   `json:"type" validate:"required"`
	DisplayName     string   `json:"name" validate:"required"`
	IssuerURL       string   `json:"issuer_url" validate:"required"`
	ClientID        string   `json:"client_id" validate:"required"`
	ClientSecretEnc string   `json:"client_secret" validate:"required"`
	Scopes          []string `json:"scopes" validate:"required"`
	EmailClaim      string   `json:"email_claim" validate:"required"`
	NameClaim       string   `json:"name_claim" validate:"required"`
	GroupsClaim     string   `json:"groups_claim" validate:"required"`

	// Provider-specific fields that don't warrant their own
	// column, per the extra_config decision from last message.
	// Each adapter parses only the keys it expects.
	ExtraConfig json.RawMessage `json:"extra_config,omitempty"`
}

type UpdateIDPConfig struct {
	Type            string   `json:"type" validate:"required"`
	DisplayName     string   `json:"name" validate:"required"`
	IssuerURL       string   `json:"issuer_url" validate:"required"`
	ClientID        string   `json:"client_id" validate:"required"`
	ClientSecretEnc string   `json:"client_secret" validate:"required"`
	Scopes          []string `json:"scopes" validate:"required"`
	EmailClaim      string   `json:"email_claim" validate:"required"`
	NameClaim       string   `json:"name_claim" validate:"required"`
	GroupsClaim     string   `json:"groups_claim" validate:"required"`

	// Provider-specific fields that don't warrant their own
	// column, per the extra_config decision from last message.
	// Each adapter parses only the keys it expects.
	ExtraConfig json.RawMessage `json:"extra_config,omitempty"`
}

type IDPResolverProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type APIIDPResolverResponse struct {
	AppID string `json:"app_id"`
	// RedirectURI string `json:"redirect_uri"`
	Providers   []IDPResolverProvider
}

type IDPLoginRequest struct {
	// RedirectURI string `json:"redirect_uri" validate:"required"`
	ProviderID string `json:"provider_id" validate:"required"`
	AppID string `json:"app_id"`
	GatewayID string `json:"gateway_id"`
}
