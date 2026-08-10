package identity

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	// "strings"
	"sync"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/gateway"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/app"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/oidc"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/org"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type IDPService struct {
	repo    Repository
	appRepo app.Repository
	// orgRepo     org.Repository
	gatewayRepo gateway.Repository
	idpSession  *IDPSession
	cache       *redis.Client
	mu          sync.RWMutex
	cfg         *config.Config
	adapters    map[string]oidc.ProviderAdapter
	adapterMu   sync.RWMutex // Separate mutex for adapter operations
	key         []byte       // 32 bytes, from KMS/Vault in production — for encrypting and decrypting the client_secret
}

func NewIDPService(
	repo Repository,
	appRepo app.Repository,
	// orgRepo org.Repository,
	gateRepo gateway.Repository,
	idpSession *IDPSession,
	cache *redis.Client,
	cfg *config.Config,
	key []byte,
) *IDPService {

	return &IDPService{
		repo:    repo,
		appRepo: appRepo,
		// orgRepo:     orgRepo,
		gatewayRepo: gateRepo,
		idpSession:  idpSession,
		cache:       cache,
		cfg:         cfg,
		adapters:    make(map[string]oidc.ProviderAdapter),
		key:         key,
	}
}

// ResolveAppIDP resolves identity providers for a given tenant/app
// It first checks for app-specific IDP configs, then falls back to tenant-level configs
func (r *IDPService) ResolveAppIDP(ctx context.Context, appID, gatewayID uuid.UUID) ([]oidc.ProviderAdapter, error) {

	// First try to get app-specific IDP configs
	appConfigs, err := r.repo.ListAppIdPConfigs(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("list app IDP configs: %w", err)
	}

	appObj, err := r.appRepo.GetByID(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("App Not Found: %w", err)
	}

	//Make sure the gateway  is either ashrix hosted or belongs to the tenant
	isValid, err := r.CheckGatewayBelongsToTenant(ctx, gatewayID, appObj.OrgID)
	if err != nil || !isValid {
		return nil, fmt.Errorf("Gateway Not Found: %w", err)
	}

	var idpconfigs []store.IdpConfig

	if len(appConfigs) > 0 {
		// Use app-specific configs
		idpconfigs = appConfigs
	} else {
		// Fall back to tenant-level configs
		tenantConfigs, err := r.repo.ListIdentityConfigsForTenant(ctx, appObj.OrgID)
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

func (r *IDPService) CheckGatewayBelongsToTenant(ctx context.Context, gatewayID, OrgID uuid.UUID) (bool, error) {
	//Get the GatewayURL

	gateway, err := r.gatewayRepo.GetGatewayByID(ctx, gatewayID)
	if err != nil {
		return false, fmt.Errorf("gateway error: %w", err)
	}
	if gateway.DeploymentType == "hosted" || gateway.OrgID == OrgID {
		return true, nil
	}
	return false, fmt.Errorf("Gateway not found")

}

func (r *IDPService) GetIDPByID(ctx context.Context, IDPUUID uuid.UUID) (store.IdpConfig, error) {
	IDPConfig, err := r.repo.GetIdentityConfigByID(ctx, IDPUUID)
	if err != nil {
		return store.IdpConfig{}, err
	}
	return IDPConfig, nil
}

func (r *IDPService) CreateTenantIdentityConfig(ctx context.Context, orgID uuid.UUID, req CreateIDPConfig) (store.IdpConfig, error) {
	encryptedSecret, err := r.encryptSecret(req.ClientSecretEnc)
	if err != nil {
		return store.IdpConfig{}, fmt.Errorf("encrypt client secret: %w", err)
	}

	return r.repo.CreateTenantIdentityConfig(ctx, store.CreateIDPConfigParams{
		OrgID:        orgID,
		Name:         req.DisplayName,
		ProviderType: req.Type,
		ClientID:     req.ClientID,
		ClientSecret: encryptedSecret,
		IssuerUrl:    req.IssuerURL,
		Scopes:       req.Scopes,
		EmailClaim:   req.EmailClaim,
		NameClaim:    req.NameClaim,
		GroupClaim:   req.GroupsClaim,
		ExtraConfig:  req.ExtraConfig,
	})
}

func (r *IDPService) UpdateIdentityConfig(ctx context.Context, id, orgID uuid.UUID, req UpdateIDPConfig) (store.IdpConfig, error) {
	encryptedSecret, err := r.encryptSecret(req.ClientSecretEnc)
	if err != nil {
		return store.IdpConfig{}, fmt.Errorf("encrypt client secret: %w", err)
	}

	return r.repo.UpdateIdentityConfig(ctx, store.UpdateIDPConfigParams{
		ID:           id,
		OrgID:        orgID,
		Name:         req.DisplayName,
		ClientID:     req.ClientID,
		ClientSecret: encryptedSecret,
		IssuerUrl:    req.IssuerURL,
		IsActive:     req.IsActive,
	})
}

func (r *IDPService) ListIdentityConfigsForTenant(ctx context.Context, orgID uuid.UUID) ([]store.IdpConfig, error) {
	return r.repo.ListIdentityConfigsForTenant(ctx, orgID)
}

func (r *IDPService) DeleteIdentityConfig(ctx context.Context, id, orgID uuid.UUID) (store.IdpConfig, error) {
	return r.repo.DeleteIdentityConfig(ctx, store.DeleteIDPConfigParams{ID: id, OrgID: orgID})
}

func (r *IDPService) BuildOAuthUrl(ctx context.Context, idp store.IdpConfig, gatewayID string) (string, error) {
	//Create the state, and build the OAUTh url
	state, code_challenge, nonce, err := r.idpSession.CreateState(
		ctx,
		idp.OrgID.String(),
		idp.ID.String(),
		gatewayID,
	)
	identityProvider := &oidc.IdentityProvider{
		ID:          idp.ID.String(),
		TenantID:    idp.OrgID.String(),
		Type:        idp.ProviderType,
		DisplayName: idp.Name,
		// OIDC specific fields from provider
		IssuerURL:       idp.IssuerUrl,
		ClientID:        idp.ClientID,     // Use app-specific client ID
		ClientSecretEnc: idp.ClientSecret, // Use app-specific client secret
		Scopes:          idp.Scopes,
		EmailClaim:      idp.EmailClaim,
		NameClaim:       idp.NameClaim,
		GroupsClaim:     idp.GroupClaim,
		ExtraConfig:     idp.ExtraConfig,
	}

	adapter, err := r.getOrCreateAdapter(ctx, identityProvider)

	if err != nil {
		return "", err
	}

	return adapter.AuthCodeURL(state, nonce, code_challenge), nil
}

func (r *IDPService) ExchangeService(ctx context.Context, state, code string) (string, string, error) {
	// Get the state from the session
	stateData, err := r.idpSession.ConsumeState(ctx, state)
	if err != nil {
		return "", "", fmt.Errorf("get state: %w", err)
	}

	//Get the OIDC Provider
	providerID, err := uuid.Parse(stateData.ProviderID)
	if err != nil {
		return "", "", fmt.Errorf("provider parsing: %w", err)
	}

	//Get the  Gateway
	GatewayID, err := uuid.Parse(stateData.GatewayID)
	if err != nil {
		return "", "", fmt.Errorf("gateway parsing: %w", err)
	}

	//Get the GatewayURL
	gateway, err := r.gatewayRepo.GetGatewayByID(ctx, GatewayID)
	if err != nil {
		return "", "", fmt.Errorf("gateway error: %w", err)
	}

	idpConfig, err := r.repo.GetIdentityConfigByID(ctx, providerID)
	if err != nil {
		return "", "", fmt.Errorf("get provider: %w", err)
	}
	identityProvider := &oidc.IdentityProvider{
		ID:          idpConfig.ID.String(),
		TenantID:    idpConfig.OrgID.String(),
		Type:        idpConfig.ProviderType,
		DisplayName: idpConfig.Name,
		// OIDC specific fields from provider
		IssuerURL:       idpConfig.IssuerUrl,
		ClientID:        idpConfig.ClientID,     // Use app-specific client ID
		ClientSecretEnc: idpConfig.ClientSecret, // Use app-specific client secret
		Scopes:          idpConfig.Scopes,
		EmailClaim:      idpConfig.EmailClaim,
		NameClaim:       idpConfig.NameClaim,
		GroupsClaim:     idpConfig.GroupClaim,
		ExtraConfig:     idpConfig.ExtraConfig,
	}

	adapter, err := r.getOrCreateAdapter(ctx, identityProvider)

	identity, err := adapter.Exchange(ctx, code, stateData.Nonce, stateData.PKCEVerifier)
	if err != nil {
		return "", "", fmt.Errorf("exchange: %w", err)
	}

	token, err := r.idpSession.CreateSession(ctx, gateway.ID.String(), gateway.Name, identity)
	if err != nil {
		return "", "", fmt.Errorf("get state: %w", err)
	}

	tenantUUID, err := uuid.Parse(stateData.TenantID)
	userUUID, err := uuid.Parse(identity.UserID)
	gatewayUUID, err := uuid.Parse(stateData.TenantID)

	if err != nil {
		return "", "", fmt.Errorf("Parse Error: %w", err)
	}

	//Create Session in the db for the gateway
	_, err = r.repo.CreateUserSessionForGateway(ctx, store.CreateUserSessionForGatewayParams{
		OrgID:     tenantUUID,
		UserID:    userUUID,
		GatewayID: gatewayUUID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(8 * time.Hour), Valid: true},
	})

	if err != nil {
		return "", "", fmt.Errorf("Session Creation Error: %w", err)
	}

	return gateway.PublicUrl, token, nil
}

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
