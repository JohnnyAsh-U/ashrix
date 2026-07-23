package oidc

import (
	"context"
	"strings"
	"sync"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/broker"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type IdPResolverCache struct {
	repo      identity.Repository
	cache     *redis.Client
	mu        sync.RWMutex
	adapters  map[string]ProviderAdapter
	adapterMu sync.RWMutex // Separate mutex for adapter operations
    identitySecretBox *broker.SecretBox
}

func NewIdPResolverCache(repo identity.Repository, cache *redis.Client, idpsecretkey *broker.SecretBox) *IdPResolverCache {
	return &IdPResolverCache{
		repo:     repo,
		cache:    cache,
		adapters: make(map[string]ProviderAdapter),
        identitySecretBox: idpsecretkey,
	}
}

// ResolveAppIDP resolves identity providers for a given email/domain
// It first checks for app-specific IDP configs, then falls back to tenant-level configs
func (r *IdPResolverCache) ResolveAppIDP(ctx context.Context, email string, orgID, appID uuid.UUID) ([]ProviderAdapter, error) {
	// Extract domain from email
	domain := extractDomain(email)
	if domain == "" {
		return nil, fmt.Errorf("invalid email format")
	}

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

	var configs []*identity.IdentityProvider
	for _, appCfg := range idpconfigs {
		// Create IdentityProvider from app config + provider details
		cfg := &identity.IdentityProvider{
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
		configs = append(configs, cfg)
	}

	// Build adapters for each config
	adapters := make([]ProviderAdapter, 0, len(configs))
	for _, cfg := range configs {
		adapter, err := r.getOrCreateAdapter(ctx, cfg)
		if err != nil {
			// Log error but continue with other providers
			continue
		}
		adapters = append(adapters, adapter)
	}

	if len(adapters) == 0 {
		return nil, fmt.Errorf("no valid IDP adapters found for email %s", email)
	}

	return adapters, nil
}


// getOrCreateAdapter retrieves an existing adapter from cache or creates a new one
func (r *IdPResolverCache) getOrCreateAdapter(ctx context.Context, cfg *identity.IdentityProvider) (ProviderAdapter, error) {
	r.adapterMu.RLock()
	if a, ok := r.adapters[cfg.ID]; ok {
		r.adapterMu.RUnlock()
		return a, nil
	}
	r.adapterMu.RUnlock()

	// Create new adapter
	clientSecret, err := r.identitySecretBox.Decrypt(cfg.ClientSecretEnc)
	if err != nil {
		return nil, fmt.Errorf("get client secret for provider %s: %w", cfg.ID, err)
	}

	adapter, err := NewAdapter(ctx, cfg, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("build adapter for provider %s: %w", cfg.ID, err)
	}

	r.adapterMu.Lock()
	r.adapters[cfg.ID] = adapter
	r.adapterMu.Unlock()

	return adapter, nil
}

// Invalidate removes a specific provider adapter from cache
func (r *IdPResolverCache) Invalidate(providerID string) {
	r.adapterMu.Lock()
	defer r.adapterMu.Unlock()
	delete(r.adapters, providerID)
}

// InvalidateAll clears all cached adapters
func (r *IdPResolverCache) InvalidateAll() {
	r.adapterMu.Lock()
	defer r.adapterMu.Unlock()
	r.adapters = make(map[string]ProviderAdapter)
}

// extractDomain extracts the domain from an email address
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}
