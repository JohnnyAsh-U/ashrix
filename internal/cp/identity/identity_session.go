// broker/internal/broker/session.go
package identity

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/oidc"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"os"
	"path/filepath"
	"time"
)

type BrokerClaims struct {
	jwt.RegisteredClaims
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Groups   []string `json:"groups"`
	Provider string   `json:"provider"`
}

type StateEntry struct {
	Nonce        string `json:"nonce"`
	AppID        string `json:"app_id"`
	TenantID     string `json:"tenant_id"`
	ProviderID   string `json:"provider_id"`
	PKCEVerifier string `json:"pkce_verifier"`
	RedirectURI  string `json:"redirect_uri"`
}

type IDPSession struct {
	signingKey *ecdsa.PrivateKey
	issuer     string
	tokenTTL   time.Duration

	rdbClient *redis.Client
	stateTTL  time.Duration
}

func NewIDPSession(baseDir, encryptionSecret, issuer string, rdb *redis.Client) (*IDPSession, error) {
	fmt.Println("Initializing JWT PKI...")

	jwtDir := filepath.Join(baseDir, "jwt")

	// Ensure the directory exists
	if err := os.MkdirAll(jwtDir, 0700); err != nil {
		return nil, fmt.Errorf("creating JWT directory: %w", err)
	}

	jwtPrivateKeyPath := filepath.Join(jwtDir, "jwt.key.enc")
	context := "jwt-signing-key"

	var signingKey *ecdsa.PrivateKey

	// Check if key exists
	keyExists, err := filehelper.FileExists(jwtPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("checking key existence: %w", err)
	}

	if keyExists {
		// Load existing key
		fmt.Println("→ Loading existing JWT signing key...")
		signingKey, err = decryptAndLoadKey(jwtPrivateKeyPath, encryptionSecret, context)
		if err != nil {
			return nil, fmt.Errorf("loading JWT signing key: %w", err)
		}
		fmt.Println("✓ JWT signing key loaded successfully")
	} else {
		// Generate new key pair
		fmt.Println("→ Generating new JWT signing key...")
		signingKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generating JWT signing key: %w", err)
		}

		// Encrypt and write the key
		if err := encryptAndWriteKey(jwtPrivateKeyPath, signingKey, encryptionSecret, context); err != nil {
			return nil, fmt.Errorf("writing JWT signing key: %w", err)
		}
		fmt.Println("✓ JWT signing key generated and saved successfully")
	}

	return &IDPSession{
		signingKey: signingKey,
		issuer:     issuer,
		tokenTTL:   15 * time.Minute,
		stateTTL:   10 * time.Minute,
		rdbClient:  rdb,
	}, nil
}

func (t *IDPSession) CreateSession(identity *oidc.NormalizedIdentity) (string, error) {
	claims := BrokerClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   identity.UserID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(t.tokenTTL)),
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
		return "", fmt.Errorf("sign broker token: %w", err)
	}
	return signed, nil
}

// For Gateway to verify; to be moved to gateway
func (t *IDPSession) VerifySession(tokenString string) (*BrokerClaims, error) {
	claims := &BrokerClaims{}
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

// Create Nonce and state
func (s *IDPSession) CreateState(ctx context.Context, tenantID, providerID, AppID, redirectURI string) (state, code_challenge, nonce string, err error) {
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
		AppID:        AppID,
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
	ok, err := s.rdbClient.SetNX(ctx, stateKey(state), data, s.stateTTL).Result()
	if err != nil {
		return "", "", "", fmt.Errorf("redis setnx: %w", err)
	}
	if !ok {
		return "", "", "", fmt.Errorf("state collision — retry")
	}

	fmt.Println(stateKey(state), string(data), AppID)

	return state, pkceChallenge, nonce, nil
}

// Consume validates and atomically deletes a state entry.
// GETDEL is atomic in Redis 6.2+ — this is what makes it genuinely
// single-use even under concurrent requests, which the earlier
// in-memory mutex-based version also achieved, but this needs to
// hold across multiple broker processes now.
func (s *IDPSession) ConsumeState(ctx context.Context, state string) (*StateEntry, error) {
	data, err := s.rdbClient.GetDel(ctx, stateKey(state)).Result()
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

// PublicKeyPEM should be exposed via a JWKS-style endpoint so
// Ashrix CP (and anything else trusting broker tokens) can verify
// signatures without a shared secret — same pattern as your CP's
// own embedded-public-key model from the PKI architecture.

// encryptAndWriteKey encrypts and writes an ECDSA private key to disk
func encryptAndWriteKey(path string, key *ecdsa.PrivateKey, secret, context string) error {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshaling key: %w", err)
	}
	defer wipeBytes(keyDER)

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	defer wipeBytes(keyPEM)

	encrypted, err := crypto.AesgcmEncrypt(keyPEM, secret, context)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encrypted, 0600)
}

// decryptAndLoadKey decrypts and loads an ECDSA private key from disk
func decryptAndLoadKey(path, secret, context string) (*ecdsa.PrivateKey, error) {
	encrypted, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	keyPEM, err := crypto.AesgcmDecrypt(encrypted, secret, context)
	if err != nil {
		return nil, err
	}
	defer wipeBytes(keyPEM)

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, errors.New("failed to decode decrypted key PEM")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// wipeBytes securely wipes a byte slice by overwriting with zeros
func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func stateKey(state string) string {
	return "broker:oidc_state:" + state
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generatePKCE() (verifier string, challenge string, err error) {
	verifier, err = randomToken(32)
	if err != nil {
		return "", "", err
	}

	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge, nil
}
