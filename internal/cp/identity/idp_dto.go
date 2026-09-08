package identity

import (
	"encoding/json"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
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

	ExtraConfig json.RawMessage `json:"extra_config,omitempty"`
}

type UpdateIDPConfig struct {
	DisplayName     string `json:"name" validate:"required"`
	ClientID        string `json:"client_id" validate:"required"`
	ClientSecretEnc string `json:"client_secret" validate:"required"`
	IssuerURL       string `json:"issuer_url" validate:"required"`
	IsActive        bool   `json:"is_active"`
}

type IDPLoginPageData struct {
	Providers []IDPLoginProvider
}

type IDPLoginProvider struct {
	ID   string
	Name string
	URL  string
}

type IDPConfigResponse struct {
	ID           string          `json:"id"`
	OrgID        string          `json:"org_id"`
	Name         string          `json:"name"`
	ProviderType string          `json:"provider_type"`
	ClientID     string          `json:"client_id"`
	IssuerURL    string          `json:"issuer_url"`
	Scopes       []string        `json:"scopes"`
	EmailClaim   string          `json:"email_claim"`
	NameClaim    string          `json:"name_claim"`
	GroupsClaim  string          `json:"groups_claim"`
	ExtraConfig  json.RawMessage `json:"extra_config,omitempty"`
	IsActive     bool            `json:"is_active"`
	IsVerified   bool            `json:"is_verified"`
	CreatedAt    time.Time       `json:"created_at"`
}

func IDPConfigToResponse(cfg store.IdpConfig) IDPConfigResponse {
	return IDPConfigResponse{
		ID:           cfg.ID.String(),
		OrgID:        cfg.OrgID.String(),
		Name:         cfg.Name,
		ProviderType: cfg.ProviderType,
		ClientID:     cfg.ClientID,
		IssuerURL:    cfg.IssuerUrl,
		Scopes:       cfg.Scopes,
		EmailClaim:   cfg.EmailClaim,
		NameClaim:    cfg.NameClaim,
		GroupsClaim:  cfg.GroupClaim,
		ExtraConfig:  cfg.ExtraConfig,
		IsActive:     cfg.IsActive,
		IsVerified:   cfg.IsVerified,
		CreatedAt:    cfg.CreatedAt,
	}
	// RedirectURI string `json:"redirect_uri"`
	// Providers   []IDPResolverProvider
}

type IDPLoginRequest struct {
	// RedirectURI string `json:"redirect_uri" validate:"required"`
	ProviderID string `json:"provider_id" validate:"required"`
	GatewayID  string `json:"gateway_id"`
}

type CreateAppIDPRelation struct {
	AppID      string `json:"app_id" validate:"required"`
	IDPID      string `json:"idp_id" validate:"required"`
	IsRequired bool   `json:"is_required"`
}

type RemoveAppIDPRelation struct {
	AppID string `json:"app_id" validate:"required"`
	IDPID string `json:"idp_id" validate:"required"`
}

type ActiveUserSessionResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	GatewayID string    `json:"gateway_id"`
	GatewayName string `json:"gateway_name"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
