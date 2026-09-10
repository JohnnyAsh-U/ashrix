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
	"log/slog"
	"os"
	"path/filepath"
	"time"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity/oidc"
	cpcrypto "github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
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
	Nonce string `json:"nonce"`
	// AppID        string `json:"app_id"`
	TenantID     string `json:"tenant_id"`
	ProviderID   string `json:"provider_id"`
	GatewayID    string `json:"gateway_id"`
	PKCEVerifier string `json:"pkce_verifier"`
	// RedirectURI  string `json:"redirect_uri"`
}

type IDPSession struct {
	signingKey *ecdsa.PrivateKey
	issuer     string
	tokenTTL   time.Duration

	jwtTokenTTL time.Duration

	rdbClient *redis.Client
	stateTTL  time.Duration
	signer    cpcrypto.BundleSigning
}

func NewIDPSession(baseDir, encryptionSecret, issuer string, rdb *redis.Client, log *slog.Logger, signer cpcrypto.BundleSigning) (*IDPSession, error) {
	log.Info("Initializing JWT PKI...")

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
		log.Info("→ Loading existing JWT signing key...")
		signingKey, err = decryptAndLoadKey(jwtPrivateKeyPath, encryptionSecret, context)
		if err != nil {
			return nil, fmt.Errorf("loading JWT signing key: %w", err)
		}
		log.Info("✓ JWT signing key loaded successfully")
	} else {
		// Generate new key pair
		log.Info("→ Generating new JWT signing key...")
		signingKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generating JWT signing key: %w", err)
		}

		// Encrypt and write the key
		if err := encryptAndWriteKey(jwtPrivateKeyPath, signingKey, encryptionSecret, context); err != nil {
			return nil, fmt.Errorf("writing JWT signing key: %w", err)
		}
		log.Info("✓ JWT signing key generated and saved successfully")
	}

	return &IDPSession{
		signingKey: signingKey,
		issuer:     issuer,
		tokenTTL:   15 * time.Minute,
		stateTTL:   10 * time.Minute,
		jwtTokenTTL: 8 *time.Hour,
		rdbClient:  rdb,
		signer:     signer,
	}, nil
}

func (t *IDPSession) CreateUserSession(ctx context.Context, gatewayID, gatewayName string, identity *proto.NormalizedIdentity) (string, error) {

	token, tokenHash, err := crypto.GenerateToken(32)
	if err != nil {
		return "", fmt.Errorf("Generation failed: %w", err)
	}

	if t.signer == nil {
		return "", fmt.Errorf("session signer is not configured")
	}
	data, err := t.issueGatewaySessionToken(identity, gatewayID)
	if err != nil {
		return "", err
	}

	// SET with TTL, NX (only set if not already present — belt and
	// suspenders against a random-generation collision, which is
	// astronomically unlikely with 32 bytes of entropy but costs
	// nothing to guard against explicitly)
	ok, err := t.rdbClient.SetNX(ctx, sessionKey(gatewayName, tokenHash), data, t.tokenTTL).Result()
	if err != nil {
		return "", fmt.Errorf("redis setnx: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("state collision — retry")
	}
	return token, nil
}

func (t *IDPSession) issueGatewaySessionToken(identity *proto.NormalizedIdentity, gatewayID string) (string, error) {
	now := time.Now()
	header, err := json.Marshal(map[string]string{"alg": "EdDSA", "kid": "cp-2026-01", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{
		"iss": "ashrix-cp", 
		"aud": []string{"ashrix-gateway"}, 
		"sub": identity.UserId,  //UserId
		"sid": identity.CpSessionId, //CPSessionId
		"tid": identity.TenantId, //TenantId
		"gid": gatewayID,  //GatewayId
		"email": identity.Email, //Email
		"name": identity.Name, //Name
		"groups": identity.Groups, //Groups
		"prov": identity.Provider,
		"iat": now.Unix(), //AuthTime
		"pid": identity.ProviderId, //ProviderId
		"nbf": now.Unix(), 
		"exp": now.Add(t.jwtTokenTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	encode := base64.RawURLEncoding.EncodeToString
	signingInput := encode(header) + "." + encode(claims)
	return signingInput + "." + encode(t.signer.SignJWT([]byte(signingInput))), nil
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
func (s *IDPSession) CreateState(ctx context.Context, tenantID, providerID, gatewayID string) (state, code_challenge, nonce string, err error) {
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
		GatewayID:    gatewayID,
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

func sessionKey(gatewayName, state string) string {
	return fmt.Sprintf("session:%s:%s", gatewayName, state)
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
