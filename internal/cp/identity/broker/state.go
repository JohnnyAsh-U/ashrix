// broker/internal/broker/state.go
package broker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// StateStore persists login-flow state in Redis, not in-memory —
// required the moment you run more than one broker instance,
// which any real production deployment behind a load balancer
// will need for availability.
type StateStore struct {
	rdb    *redis.Client
	prefix string
	ttl    time.Duration
}

type StateEntry struct {
	Nonce        string `json:"nonce"`
	TenantID     string `json:"tenant_id"`
	ProviderID   string `json:"provider_id"`
	PKCEVerifier string `json:"pkce_verifier"`
	RedirectURI  string `json:"redirect_uri"`
}

func NewIDPStateStore(rdb *redis.Client, prefix string) *StateStore {
	return &StateStore{rdb: rdb, prefix: prefix, ttl: 10 * time.Minute}
}

// Create Nonce and state
func (s *StateStore) Create(ctx context.Context, tenantID, providerID, redirectURI string) (state, code_challenge, nonce string, err error) {
	state, err = randomToken(32)
	if err != nil {
		return "", "", "", err
	}
	nonce, err = randomToken(32)
	if err != nil {
		return "", "", "", err
	}

	pkceVerifier, pkceChallenge, err := generatePKCE()
	if err != nil {
		return "", "", "", err
	}

	entry := StateEntry{
		Nonce:        nonce,
		TenantID:     tenantID,
		ProviderID:   providerID,
		RedirectURI:  redirectURI,
		PKCEVerifier: pkceVerifier,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", "", "", err
	}

	// SET with TTL, NX (only set if not already present — belt and
	// suspenders against a random-generation collision, which is
	// astronomically unlikely with 32 bytes of entropy but costs
	// nothing to guard against explicitly)
	ok, err := s.rdb.SetNX(ctx, stateKey(state), data, s.ttl).Result()
	if err != nil {
		return "", "", "", fmt.Errorf("redis setnx: %w", err)
	}
	if !ok {
		return "", "", "", fmt.Errorf("state collision — retry")
	}

	return state, pkceChallenge, nonce, nil
}

// func (s *StateStore)

// Consume validates and atomically deletes a state entry.
// GETDEL is atomic in Redis 6.2+ — this is what makes it genuinely
// single-use even under concurrent requests, which the earlier
// in-memory mutex-based version also achieved, but this needs to
// hold across multiple broker processes now.
func (s *StateStore) Consume(ctx context.Context, state string) (*StateEntry, error) {
	data, err := s.rdb.GetDel(ctx, stateKey(state)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("unknown, expired, or already-used state")
	}
	if err != nil {
		return nil, fmt.Errorf("redis getdel: %w", err)
	}

	var entry StateEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, fmt.Errorf("unmarshal state entry: %w", err)
	}
	return &entry, nil
}

func stateKey(state string) string {
	return "broker:oidc_state:" + state
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func generatePKCE() (verifier string, challenge string, err error) {
	verifier, err = randomToken(43)
	if err != nil {
		return "", "", err
	}

	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.URLEncoding.EncodeToString(hash[:])
	return verifier, challenge, nil
}
