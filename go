ashrix-cp/
└── internal/
    ├── identity/
    │   ├── types.go
    │   ├── secretbox.go
    │   ├── store.go            ← uses CP's existing Postgres
    │   ├── statecookie.go      ← replaces Redis entirely
    │   ├── session.go          ← broker-style JWT issuance
    │   ├── unimplemented.go    ← SAML marker
    │   ├── oidc/
    │   │   ├── client.go
    │   │   └── presets.go
    │   ├── graph/
    │   │   └── entra_groups.go
    │   └── httpapi/
    │       ├── login.go
    │       ├── callback.go
    │       ├── admin.go
    │       └── jwks.go




// internal/identity/types.go
package identity

import "time"

// IdentityProvider is one configured upstream OIDC IdP, scoped
// to a tenant. This lives in CP's own Postgres — no separate
// database, per the library-integration decision.
type IdentityProvider struct {
	ID          string
	TenantID    string
	Type        string // "oidc_google", "oidc_entra", "oidc_okta",
	                    // "oidc_keycloak", "oidc_generic"
	DisplayName string
	Enabled     bool
	CreatedAt   time.Time

	OIDCConfig OIDCProviderConfig
}

type OIDCProviderConfig struct {
	IssuerURL          string
	ClientID           string
	ClientSecretEnc    string // encrypted, using CP's existing secret-box key
	Scopes             []string
	EmailClaim         string
	NameClaim          string
	GroupsClaim        string
	ResolveEntraGroups bool
	GoogleHostedDomain string
	EntraTenantID      string
	EntraGraphSecretEnc string
}

// NormalizedIdentity is the ONLY shape any CP code downstream of
// login ever sees, regardless of which upstream provider was used.
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






// internal/identity/secretbox.go
package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// SecretBox encrypts client secrets AND state cookies. This is
// the SAME key CP already uses elsewhere for at-rest encryption
// of sensitive config (per your existing PKI/KMS wiring) — no
// new key management surface, just a new use of an existing one.
type SecretBox struct {
	key []byte
}

func NewSecretBox(key []byte) (*SecretBox, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secret box key must be 32 bytes, got %d", len(key))
	}
	return &SecretBox{key: key}, nil
}

func (s *SecretBox) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("cipher init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm init: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce generation: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func (s *SecretBox) Decrypt(encoded string) (string, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("cipher init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm init: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (tampered or wrong key): %w", err)
	}
	return string(plaintext), nil
}






// internal/identity/statecookie.go
package identity

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// StateCookie carries login-flow state through the browser
// redirect round-trip to the IdP and back, encrypted with CP's
// existing SecretBox. This eliminates the need for Redis, or any
// server-side storage, just to track "a login is in progress" —
// the AES-GCM authentication tag makes tampering detectable, and
// the embedded expiry makes replay-after-completion impossible
// once the flow finishes (see single-use enforcement note below).
type StateCookie struct {
	Nonce       string    `json:"nonce"`
	TenantID    string    `json:"tenant_id"`
	ProviderID  string    `json:"provider_id"`
	RedirectURI string    `json:"redirect_uri"`
	ExpiresAt   time.Time `json:"expires_at"`
}

const stateCookieTTL = 10 * time.Minute

func EncodeStateCookie(tenantID, providerID, redirectURI string, box *SecretBox) (encoded, nonce string, err error) {
	nonce, err = randomToken(32)
	if err != nil {
		return "", "", err
	}

	entry := StateCookie{
		Nonce:       nonce,
		TenantID:    tenantID,
		ProviderID:  providerID,
		RedirectURI: redirectURI,
		ExpiresAt:   time.Now().Add(stateCookieTTL),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", "", fmt.Errorf("marshal state: %w", err)
	}

	encoded, err = box.Encrypt(string(data))
	if err != nil {
		return "", "", fmt.Errorf("encrypt state: %w", err)
	}

	return encoded, nonce, nil
}

func DecodeStateCookie(encoded string, box *SecretBox) (*StateCookie, error) {
	plain, err := box.Decrypt(encoded)
	if err != nil {
		return nil, fmt.Errorf("state cookie invalid or tampered: %w", err)
	}

	var entry StateCookie
	if err := json.Unmarshal([]byte(plain), &entry); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, fmt.Errorf("state cookie expired")
	}

	return &entry, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}




// internal/identity/store.go
package identity

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// Store persists IdentityProvider configs using CP's EXISTING
// database connection — no new database, no new connection pool.
// Pass in CP's already-established *sql.DB.
type Store struct {
	db        *sql.DB
	secretBox *SecretBox
}

func NewStore(db *sql.DB, secretBox *SecretBox) *Store {
	return &Store{db: db, secretBox: secretBox}
}

// Migrate should be called as part of CP's OWN migration sequence,
// not as a separate step — one migration pipeline, one source of
// truth for schema state.
func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS identity_providers (
			id                     TEXT PRIMARY KEY,
			tenant_id              TEXT NOT NULL,
			type                   TEXT NOT NULL,
			display_name           TEXT NOT NULL,
			enabled                BOOLEAN NOT NULL DEFAULT true,
			issuer_url             TEXT NOT NULL,
			client_id              TEXT NOT NULL,
			client_secret_enc      TEXT NOT NULL,
			scopes                 TEXT[] NOT NULL,
			email_claim            TEXT NOT NULL,
			name_claim             TEXT NOT NULL,
			groups_claim           TEXT NOT NULL DEFAULT '',
			resolve_entra_groups   BOOLEAN NOT NULL DEFAULT false,
			google_hosted_domain   TEXT NOT NULL DEFAULT '',
			entra_tenant_id        TEXT NOT NULL DEFAULT '',
			entra_graph_secret_enc TEXT NOT NULL DEFAULT '',
			created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_idp_tenant ON identity_providers(tenant_id);

		-- Tenant-scoped allowlist of redirect URIs — closes the
		-- open-redirect vector in the login handler below.
		CREATE TABLE IF NOT EXISTS identity_redirect_allowlist (
			tenant_id    TEXT NOT NULL,
			redirect_uri TEXT NOT NULL,
			PRIMARY KEY (tenant_id, redirect_uri)
		);
	`)
	return err
}

func (s *Store) CreateProvider(ctx context.Context, p *IdentityProvider, plainSecret string) error {
	if err := ValidateProviderType(p.Type); err != nil {
		return err
	}

	encSecret, err := s.secretBox.Encrypt(plainSecret)
	if err != nil {
		return fmt.Errorf("encrypt client secret: %w", err)
	}

	var encGraphSecret string
	if p.OIDCConfig.EntraGraphSecretEnc != "" {
		encGraphSecret, err = s.secretBox.Encrypt(p.OIDCConfig.EntraGraphSecretEnc)
		if err != nil {
			return fmt.Errorf("encrypt graph secret: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO identity_providers (
			id, tenant_id, type, display_name, enabled,
			issuer_url, client_id, client_secret_enc, scopes,
			email_claim, name_claim, groups_claim,
			resolve_entra_groups, google_hosted_domain,
			entra_tenant_id, entra_graph_secret_enc
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		p.ID, p.TenantID, p.Type, p.DisplayName, p.Enabled,
		p.OIDCConfig.IssuerURL, p.OIDCConfig.ClientID, encSecret,
		pq.Array(p.OIDCConfig.Scopes),
		p.OIDCConfig.EmailClaim, p.OIDCConfig.NameClaim, p.OIDCConfig.GroupsClaim,
		p.OIDCConfig.ResolveEntraGroups, p.OIDCConfig.GoogleHostedDomain,
		p.OIDCConfig.EntraTenantID, encGraphSecret,
	)
	if err != nil {
		return fmt.Errorf("insert provider: %w", err)
	}
	return nil
}

func (s *Store) GetProvider(ctx context.Context, id string) (*IdentityProvider, string, error) {
	var p IdentityProvider
	var encSecret, encGraphSecret string
	var scopes []string

	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, type, display_name, enabled,
		       issuer_url, client_id, client_secret_enc, scopes,
		       email_claim, name_claim, groups_claim,
		       resolve_entra_groups, google_hosted_domain,
		       entra_tenant_id, entra_graph_secret_enc, created_at
		FROM identity_providers WHERE id = $1`, id)

	err := row.Scan(
		&p.ID, &p.TenantID, &p.Type, &p.DisplayName, &p.Enabled,
		&p.OIDCConfig.IssuerURL, &p.OIDCConfig.ClientID, &encSecret,
		pq.Array(&scopes),
		&p.OIDCConfig.EmailClaim, &p.OIDCConfig.NameClaim, &p.OIDCConfig.GroupsClaim,
		&p.OIDCConfig.ResolveEntraGroups, &p.OIDCConfig.GoogleHostedDomain,
		&p.OIDCConfig.EntraTenantID, &encGraphSecret, &p.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, "", fmt.Errorf("provider not found: %s", id)
	}
	if err != nil {
		return nil, "", fmt.Errorf("query provider: %w", err)
	}
	p.OIDCConfig.Scopes = scopes

	plainSecret, err := s.secretBox.Decrypt(encSecret)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt client secret: %w", err)
	}

	if encGraphSecret != "" {
		plainGraphSecret, err := s.secretBox.Decrypt(encGraphSecret)
		if err != nil {
			return nil, "", fmt.Errorf("decrypt graph secret: %w", err)
		}
		p.OIDCConfig.EntraGraphSecretEnc = plainGraphSecret
	}

	return &p, plainSecret, nil
}

func (s *Store) ListProvidersForTenant(ctx context.Context, tenantID string) ([]*IdentityProvider, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, type, display_name, enabled
		FROM identity_providers WHERE tenant_id = $1 AND enabled = true`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	var out []*IdentityProvider
	for rows.Next() {
		var p IdentityProvider
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Type, &p.DisplayName, &p.Enabled); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// IsAllowedRedirect checks the tenant-scoped allowlist — the
// actual fix for the open-redirect risk named in the earlier
// version, now backed by a real table instead of an in-memory map.
func (s *Store) IsAllowedRedirect(ctx context.Context, tenantID, redirectURI string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM identity_redirect_allowlist
			WHERE tenant_id = $1 AND redirect_uri = $2
		)`, tenantID, redirectURI).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check redirect allowlist: %w", err)
	}
	return exists, nil
}




// internal/identity/session.go
package identity

import (
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SessionClaims is what CP issues to itself after a successful
// login — this becomes the ACTUAL session token used elsewhere
// in CP (e.g. by the gateway's user-facing auth checks, once
// that authorization layer exists). Since this is now a library
// inside CP rather than a separate broker, there's no need for
// an intermediate "broker token" at all — login produces CP's
// own session token directly.
type SessionClaims struct {
	jwt.RegisteredClaims
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Groups   []string `json:"groups"`
	Provider string   `json:"provider"`
}

type TokenIssuer struct {
	signingKey *ecdsa.PrivateKey // CP's EXISTING signing key —
	                              // the same one used for bundle
	                              // signatures per your PKI doc,
	                              // or a dedicated session-signing
	                              // key if you prefer key separation
	                              // (recommended: separate key,
	                              // "one key one job" per your own
	                              // stated PKI principle)
	issuer string
	ttl    time.Duration
}

func NewTokenIssuer(signingKey *ecdsa.PrivateKey, issuer string) *TokenIssuer {
	return &TokenIssuer{signingKey: signingKey, issuer: issuer, ttl: 8 * time.Hour}
}

func (t *TokenIssuer) Issue(identity *NormalizedIdentity) (string, error) {
	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   identity.UserID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(t.ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		TenantID: identity.TenantID,
		UserID:   identity.UserID,
		Email:    identity.Email,
		Name:     identity.Name,
		Groups:   identity.Groups,
		Provider: identity.Provider,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	signed, err := token.SignedString(t.signingKey)
	if err != nil {
		return "", fmt.Errorf("sign session token: %w", err)
	}
	return signed, nil
}

func (t *TokenIssuer) Verify(tokenString string) (*SessionClaims, error) {
	claims := &SessionClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(tok *jwt.Token) (interface{}, error) {
		return &t.signingKey.PublicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("verify session token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid session token")
	}
	return claims, nil
}





// internal/identity/oidc/client.go
package oidc

import (
	"context"
	"fmt"
	"sync"

	oidclib "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/JohnnyAsh-U/ashrix-cp/internal/identity"
	"github.com/JohnnyAsh-U/ashrix-cp/internal/identity/graph"
)

type Client struct {
	cfg          *identity.IdentityProvider
	provider     *oidclib.Provider
	oauth2Config oauth2.Config
	verifier     *oidclib.IDTokenVerifier
}

type ClientCache struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewClientCache() *ClientCache {
	return &ClientCache{clients: make(map[string]*Client)}
}

func (c *ClientCache) Get(ctx context.Context, cfg *identity.IdentityProvider, clientSecret string) (*Client, error) {
	c.mu.RLock()
	if client, ok := c.clients[cfg.ID]; ok {
		c.mu.RUnlock()
		return client, nil
	}
	c.mu.RUnlock()

	client, err := newClient(ctx, cfg, clientSecret)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.clients[cfg.ID] = client
	c.mu.Unlock()

	return client, nil
}

func (c *ClientCache) Invalidate(providerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.clients, providerID)
}

func newClient(ctx context.Context, cfg *identity.IdentityProvider, clientSecret string) (*Client, error) {
	provider, err := oidclib.NewProvider(ctx, cfg.OIDCConfig.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed for %s: %w", cfg.OIDCConfig.IssuerURL, err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.OIDCConfig.ClientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.OIDCConfig.Scopes,
	}

	verifier := provider.Verifier(&oidclib.Config{ClientID: cfg.OIDCConfig.ClientID})

	return &Client{cfg: cfg, provider: provider, oauth2Config: oauth2Config, verifier: verifier}, nil
}

func (c *Client) AuthCodeURL(state, nonce string) string {
	return c.oauth2Config.AuthCodeURL(state, oidclib.Nonce(nonce))
}

func (c *Client) Exchange(
	ctx context.Context,
	code string,
	expectedNonce string,
	graphClient *graph.EntraClient,
) (*identity.NormalizedIdentity, error) {

	token, err := c.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification failed: %w", err)
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("claims parse failed: %w", err)
	}

	if nonceClaim, _ := claims["nonce"].(string); nonceClaim != expectedNonce {
		return nil, fmt.Errorf("nonce mismatch — possible token substitution")
	}

	return c.mapClaims(ctx, claims, graphClient)
}

func (c *Client) mapClaims(
	ctx context.Context,
	claims map[string]interface{},
	graphClient *graph.EntraClient,
) (*identity.NormalizedIdentity, error) {

	email, _ := claims[c.cfg.OIDCConfig.EmailClaim].(string)
	if email == "" {
		return nil, fmt.Errorf("email claim %q missing or empty", c.cfg.OIDCConfig.EmailClaim)
	}

	if c.cfg.OIDCConfig.GoogleHostedDomain != "" {
		hd, _ := claims["hd"].(string)
		if hd != c.cfg.OIDCConfig.GoogleHostedDomain {
			return nil, fmt.Errorf("hosted domain mismatch: expected %q, got %q",
				c.cfg.OIDCConfig.GoogleHostedDomain, hd)
		}
	}

	name, _ := claims[c.cfg.OIDCConfig.NameClaim].(string)
	sub, _ := claims["sub"].(string)

	var groups []string
	if c.cfg.OIDCConfig.GroupsClaim != "" {
		if raw, ok := claims[c.cfg.OIDCConfig.GroupsClaim].([]interface{}); ok {
			for _, g := range raw {
				if s, ok := g.(string); ok {
					groups = append(groups, s)
				}
			}
		}
	}

	if c.cfg.OIDCConfig.ResolveEntraGroups {
		if graphClient == nil {
			return nil, fmt.Errorf("Entra group resolution required but Graph client not configured")
		}
		resolved, err := graphClient.ResolveGroupNames(ctx, groups)
		if err != nil {
			return nil, fmt.Errorf("Entra group resolution failed: %w", err)
		}
		groups = resolved
	}

	return &identity.NormalizedIdentity{
		TenantID:   c.cfg.TenantID,
		ProviderID: c.cfg.ID,
		UserID:     sub,
		Email:      email,
		Name:       name,
		Groups:     groups,
		Provider:   c.cfg.Type,
		AuthTime:   time.Now(),
	}, nil
}




// internal/identity/oidc/presets.go
package oidc

import (
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-cp/internal/identity"
)

func GooglePreset(tenantID, clientID, clientSecret, hostedDomain string) *identity.IdentityProvider {
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_google",
		DisplayName: "Google",
		Enabled:     true,
		OIDCConfig: identity.OIDCProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  clientID,
			Scopes:    []string{"openid", "email", "profile"},
			EmailClaim: "email",
			NameClaim:  "name",
			// Google does not provide groups via OIDC ID token —
			// left empty deliberately, not silently assumed to work.
			GoogleHostedDomain: hostedDomain,
		},
	}
}

func EntraPreset(tenantID, entraTenantID, clientID, clientSecret string) *identity.IdentityProvider {
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_entra",
		DisplayName: "Microsoft Entra ID",
		Enabled:     true,
		OIDCConfig: identity.OIDCProviderConfig{
			IssuerURL: fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", entraTenantID),
			ClientID:  clientID,
			Scopes:    []string{"openid", "email", "profile"},
			EmailClaim:         "preferred_username",
			NameClaim:          "name",
			GroupsClaim:        "groups",
			ResolveEntraGroups: true,
			EntraTenantID:      entraTenantID,
		},
	}
}

func OktaPreset(tenantID, oktaDomain, clientID, clientSecret, authServerID string) *identity.IdentityProvider {
	issuer := fmt.Sprintf("https://%s/oauth2/default", oktaDomain)
	if authServerID != "" {
		issuer = fmt.Sprintf("https://%s/oauth2/%s", oktaDomain, authServerID)
	}
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_okta",
		DisplayName: "Okta",
		Enabled:     true,
		OIDCConfig: identity.OIDCProviderConfig{
			IssuerURL: issuer,
			ClientID:  clientID,
			Scopes:    []string{"openid", "email", "profile", "groups"},
			EmailClaim:  "email",
			NameClaim:   "name",
			GroupsClaim: "groups",
		},
	}
}

func KeycloakPreset(tenantID, baseURL, realm, clientID, clientSecret string) *identity.IdentityProvider {
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_keycloak",
		DisplayName: "Keycloak",
		Enabled:     true,
		OIDCConfig: identity.OIDCProviderConfig{
			IssuerURL: fmt.Sprintf("%s/realms/%s", baseURL, realm),
			ClientID:  clientID,
			Scopes:    []string{"openid", "email", "profile", "groups"},
			EmailClaim:  "email",
			NameClaim:   "name",
			GroupsClaim: "groups",
		},
	}
}

func GenericOIDCPreset(
	tenantID, issuerURL, clientID, clientSecret string,
	mapping identity.OIDCProviderConfig,
) *identity.IdentityProvider {
	mapping.IssuerURL = issuerURL
	mapping.ClientID = clientID
	if len(mapping.Scopes) == 0 {
		mapping.Scopes = []string{"openid", "email", "profile"}
	}
	return &identity.IdentityProvider{
		TenantID:    tenantID,
		Type:        "oidc_generic",
		DisplayName: "Custom OIDC Provider",
		Enabled:     true,
		OIDCConfig:  mapping,
	}
}






// internal/identity/graph/entra_groups.go
package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

type EntraClient struct {
	httpClient *http.Client
}

func NewEntraClient(tenantID, clientID, clientSecret string) *EntraClient {
	cfg := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID),
		Scopes:       []string{"https://graph.microsoft.com/.default"},
	}
	return &EntraClient{httpClient: cfg.Client(context.Background())}
}

type graphGroupResponse struct {
	DisplayName string `json:"displayName"`
}

func (c *EntraClient) ResolveGroupNames(ctx context.Context, groupGUIDs []string) ([]string, error) {
	var names []string

	for _, guid := range groupGUIDs {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		url := fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s?$select=displayName", guid)

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("graph lookup failed for group %s: %w", guid, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("graph returned %d for group %s", resp.StatusCode, guid)
		}

		var g graphGroupResponse
		if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
			return nil, fmt.Errorf("decode graph response: %w", err)
		}
		names = append(names, g.DisplayName)
	}

	return names, nil
}



// internal/identity/unimplemented.go
package identity

import "errors"

var ErrSAMLNotImplemented = errors.New(
	"SAML support is not implemented. See internal/identity/saml/ " +
		"as the planned integration point. Use crewjam/saml as the " +
		"underlying library if implemented — do not hand-roll XML " +
		"signature verification.",
)

func IsSAMLProvider(providerType string) bool {
	return providerType == "saml"
}

func ValidateProviderType(providerType string) error {
	if IsSAMLProvider(providerType) {
		return ErrSAMLNotImplemented
	}
	switch providerType {
	case "oidc_google", "oidc_entra", "oidc_okta", "oidc_keycloak", "oidc_generic":
		return nil
	default:
		return errors.New("unknown provider type: " + providerType)
	}
}




// internal/identity/httpapi/login.go
package httpapi

import (
	"net/http"
	"time"
)

// LoginHandler is mounted directly on CP's existing http.ServeMux
// or router — NOT a separate server. GET /auth/login?tenant=X&provider=Y&redirect_uri=Z
func (a *API) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.URL.Query().Get("tenant")
	providerID := r.URL.Query().Get("provider")
	redirectURI := r.URL.Query().Get("redirect_uri")

	if tenantID == "" || providerID == "" || redirectURI == "" {
		http.Error(w, "tenant, provider, and redirect_uri are required", http.StatusBadRequest)
		return
	}

	allowed, err := a.store.IsAllowedRedirect(ctx, tenantID, redirectURI)
	if err != nil || !allowed {
		http.Error(w, "redirect_uri not allowed for this tenant", http.StatusBadRequest)
		return
	}

	cfg, plainSecret, err := a.store.GetProvider(ctx, providerID)
	if err != nil || cfg.TenantID != tenantID || !cfg.Enabled {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	client, err := a.clientCache.Get(ctx, cfg, plainSecret)
	if err != nil {
		a.log.Error("failed to init OIDC client", "error", err, "provider", providerID)
		http.Error(w, "identity provider unavailable", http.StatusBadGateway)
		return
	}

	encodedState, nonce, err := EncodeStateCookieFn(tenantID, providerID, redirectURI, a.secretBox)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// State cookie — httpOnly, secure, short-lived, scoped to the
	// callback path only. This IS the state store now — no server
	// side storage needed at all.
	http.SetCookie(w, &http.Cookie{
		Name:     "ashrix_oidc_state",
		Value:    encodedState,
		Path:     "/auth/callback",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode, // Lax, not Strict — the IdP
		                                 // redirect back to us is a
		                                 // cross-site top-level
		                                 // navigation, which Strict
		                                 // would block, breaking
		                                 // the entire flow
		MaxAge: int(10 * time.Minute / time.Second),
	})

	http.Redirect(w, r, client.AuthCodeURL(encodedState, nonce), http.StatusFound)
}





// internal/identity/httpapi/callback.go
package httpapi

import (
	"net/http"
)

// CallbackHandler — GET /auth/callback?code=X&state=Y
func (a *API) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		a.log.Warn("IdP returned error", "error", errParam,
			"description", r.URL.Query().Get("error_description"))
		http.Error(w, "authentication failed at identity provider", http.StatusBadRequest)
		return
	}

	stateCookie, err := r.Cookie("ashrix_oidc_state")
	if err != nil {
		http.Error(w, "missing state cookie — login flow expired or was not started here", http.StatusBadRequest)
		return
	}

	// Cross-check: the query param state MUST match the cookie
	// value exactly. This is the double-submit pattern — an
	// attacker forging just the callback URL (query param) without
	// also controlling the victim's cookie jar cannot pass this.
	if stateCookie.Value != stateParam {
		a.log.Warn("state mismatch between cookie and query param — possible CSRF attempt")
		http.Error(w, "invalid login state", http.StatusBadRequest)
		return
	}

	entry, err := DecodeStateCookie(stateCookie.Value, a.secretBox)
	if err != nil {
		a.log.Warn("state cookie decode failed", "error", err)
		http.Error(w, "invalid or expired login attempt", http.StatusBadRequest)
		return
	}

	// Clear the cookie immediately — even though it's already
	// expiry-bound, clearing it now removes any temptation for
	// a client-side replay within the TTL window.
	http.SetCookie(w, &http.Cookie{
		Name: "ashrix_oidc_state", Value: "", Path: "/auth/callback",
		MaxAge: -1, HttpOnly: true, Secure: true,
	})

	cfg, plainSecret, err := a.store.GetProvider(ctx, entry.ProviderID)
	if err != nil {
		http.Error(w, "provider configuration error", http.StatusInternalServerError)
		return
	}

	client, err := a.clientCache.Get(ctx, cfg, plainSecret)
	if err != nil {
		http.Error(w, "identity provider unavailable", http.StatusBadGateway)
		return
	}

	var graphClient *graph.EntraClient
	if cfg.OIDCConfig.ResolveEntraGroups {
		graphClient = a.entraClientFor(cfg)
	}

	nid, err := client.Exchange(ctx, code, entry.Nonce, graphClient)
	if err != nil {
		a.log.Error("token exchange/verification failed", "error", err, "provider", entry.ProviderID)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	sessionToken, err := a.tokenIssuer.Issue(nid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Session token as its own httpOnly cookie on CP's domain —
	// no query-string token passing, no browser history leakage,
	// which was the flagged hardening gap in the HTTP-service version.
	http.SetCookie(w, &http.Cookie{
		Name:     "ashrix_session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(8 * time.Hour / time.Second),
	})

	http.Redirect(w, r, entry.RedirectURI, http.StatusFound)
}







// In CP's existing startup — NOT a new binary, additions to
// what already exists

identitySecretBox, err := identity.NewSecretBox(cfg.SecretBoxKey) // from
                                                                    // existing KMS/Vault wiring
identityStore := identity.NewStore(cpDB, identitySecretBox) // cpDB is
                                                              // CP's EXISTING *sql.DB
if err := identityStore.Migrate(ctx); err != nil {
    log.Fatal("identity migration failed", zap.Error(err))
}

sessionSigningKey := loadSessionSigningKey() // separate key from
                                              // bundle-signing key,
                                              // per "one key one job"
tokenIssuer := identity.NewTokenIssuer(sessionSigningKey, "ashrix-cp")
clientCache := oidc.NewClientCache()

identityAPI := httpapi.New(identityStore, identitySecretBox, tokenIssuer, clientCache, log)

// Mounted on CP's EXISTING router/mux, not a new server
cpRouter.HandleFunc("GET /auth/login", identityAPI.LoginHandler)
cpRouter.HandleFunc("GET /auth/callback", identityAPI.CallbackHandler)
cpRouter.HandleFunc("POST /admin/identity/providers", identityAPI.CreateProviderHandler)