package identity

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/oidc"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type IDPService struct {
	repo       Repository
	idpSession *IDPSession
	cache      *redis.Client
	mu         sync.RWMutex
	adapters   map[string]oidc.ProviderAdapter
	adapterMu  sync.RWMutex // Separate mutex for adapter operations
	key        []byte       // 32 bytes, from KMS/Vault in production — for encrypting and decrypting the client_secret
}

func NewIDPService(repo Repository, idpSession *IDPSession, cache *redis.Client, key []byte) *IDPService {

	return &IDPService{
		repo:       repo,
		idpSession: idpSession,
		cache:      cache,
		adapters:   make(map[string]oidc.ProviderAdapter),
		key:        key,
	}
}

// ResolveAppIDP resolves identity providers for a given tenant/app
// It first checks for app-specific IDP configs, then falls back to tenant-level configs
func (r *IDPService) ResolveAppIDP(ctx context.Context, orgID, appID uuid.UUID) ([]oidc.ProviderAdapter, error) {

	// First try to get app-specific IDP configs
	appConfigs, err := r.repo.ListAppIdPConfigs(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("list app IDP configs: %w", err)
	}

	var idpconfigs []store.IdpConfig

	if len(appConfigs) > 0 {
		// Use app-specific configs
		idpconfigs = appConfigs
	} else {
		// Fall back to tenant-level configs
		tenantConfigs, err := r.repo.ListIdentityConfigsForTenant(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("list tenant IDP configs: %w", err)
		}
		idpconfigs = tenantConfigs
	}

	var configs []*oidc.IdentityProvider
	for _, appCfg := range idpconfigs {

		// Create IdentityProvider from app config + provider details
		cfg := oidc.IdentityProvider{
			ID:          appCfg.ID.String(),
			TenantID:    appCfg.OrgID.String(),
			Type:        appCfg.ProviderType,
			DisplayName: appCfg.Name,
			// OIDC specific fields from provider
			IssuerURL:       appCfg.IssuerUrl,
			ClientID:        appCfg.ClientID,     // Use app-specific client ID
			ClientSecretEnc: appCfg.ClientSecret, // Use app-specific client secret
			Scopes:          appCfg.Scopes,
			EmailClaim:      appCfg.EmailClaim,
			NameClaim:       appCfg.NameClaim,
			GroupsClaim:     appCfg.GroupClaim,
			ExtraConfig:     appCfg.ExtraConfig,
		}
		configs = append(configs, &cfg)
	}

	// Build adapters for each config
	adapters := make([]oidc.ProviderAdapter, 0, len(configs))
	for _, cfg := range configs {

		adapter, err := r.getOrCreateAdapter(ctx, cfg)
		if err != nil {
			// Log error but continue with other providers
			continue
		}
		adapters = append(adapters, adapter)
	}

	if len(adapters) == 0 {
		return nil, fmt.Errorf("no valid IDP adapters found")
	}

	return adapters, nil
}

func (r *IDPService) IsRedirectURIValidByIDP(ctx context.Context, IDPUUID uuid.UUID, redirectURI string) bool {
	//List all apps by the org of the idp config
	IDPConfig, err := r.repo.GetIdentityConfigByID(ctx, IDPUUID)
	if err != nil {
		return false
	}

	appIDPs, err := r.repo.ListAppsByOrg(ctx, IDPConfig.OrgID)
	if err != nil {
		return false
	}

	orgObj, err := r.repo.GetOrgByID(ctx, IDPConfig.OrgID)

	if err != nil {
		return false
	}

	if len(appIDPs) == 0 {
		return false
	}
	//TODO: CHECK FOR ORG IDP

	for _, appIDP := range appIDPs {
		AppURL := fmt.Sprintf("https://%s.%s", appIDP.Subdomain, orgObj.CustomDomain.String)
		if strings.HasPrefix(redirectURI, AppURL) {
			return true
		}
	}
	return false
}

// func (r *IDPService)

// getOrCreateAdapter retrieves an existing adapter from cache or creates a new one
func (r *IDPService) getOrCreateAdapter(ctx context.Context, cfg *oidc.IdentityProvider) (oidc.ProviderAdapter, error) {
	r.adapterMu.RLock()
	if a, ok := r.adapters[cfg.ID]; ok {
		r.adapterMu.RUnlock()
		return a, nil
	}
	r.adapterMu.RUnlock()

	// Create new adapter
	clientSecret, err := r.decryptSecret(cfg.ClientSecretEnc)
	if err != nil {
		return nil, fmt.Errorf("get client secret for provider %s: %w", cfg.ID, err)
	}

	adapter, err := oidc.NewAdapter(ctx, cfg, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("build adapter for provider %s: %w", cfg.ID, err)
	}

	r.adapterMu.Lock()
	r.adapters[cfg.ID] = adapter
	r.adapterMu.Unlock()

	return adapter, nil
}

// Invalidate removes a specific provider adapter from cache
func (r *IDPService) InvalidateCache(providerID string) {
	r.adapterMu.Lock()
	defer r.adapterMu.Unlock()
	delete(r.adapters, providerID)
}

// InvalidateAll clears all cached adapters
func (r *IDPService) InvalidateAllCache() {
	r.adapterMu.Lock()
	defer r.adapterMu.Unlock()
	r.adapters = make(map[string]oidc.ProviderAdapter)
}

func (s *IDPService) encryptSecret(plaintext string) (string, error) {
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
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *IDPService) decryptSecret(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
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
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	return string(plaintext), nil
}
