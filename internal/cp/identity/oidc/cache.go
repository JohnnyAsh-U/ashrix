// internal/identity/oidc/cache.go
package oidc

import (
	"context"
	"fmt"
	"sync"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
)

// AdapterCache avoids re-running OIDC discovery on every login.
// Keyed by provider ID, invalidated when an admin edits that
// provider's config.
type AdapterCache struct {
	mu       sync.RWMutex
	adapters map[string]ProviderAdapter
}

func NewAdapterCache() *AdapterCache {
	return &AdapterCache{adapters: make(map[string]ProviderAdapter)}
}

func (c *AdapterCache) Get(ctx context.Context, cfg *identity.IdentityProvider, clientSecret string) (ProviderAdapter, error) {
	c.mu.RLock()
	if a, ok := c.adapters[cfg.ID]; ok {
		c.mu.RUnlock()
		return a, nil
	}
	c.mu.RUnlock()

	adapter, err := NewAdapter(ctx, cfg, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("build adapter for provider %s: %w", cfg.ID, err)
	}

	c.mu.Lock()
	c.adapters[cfg.ID] = adapter
	c.mu.Unlock()

	return adapter, nil
}

func (c *AdapterCache) Invalidate(providerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.adapters, providerID)
}