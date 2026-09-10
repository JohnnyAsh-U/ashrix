package session

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/publicsuffix"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	sessionCookie = "__Ashrix-session"

	sessionKeyPrefix     = "session:"
	cpSessionIndexPrefix = "index:cp-session:"
)

var (
	ErrNoSession       = errors.New("no session")
	ErrSessionNotFound = errors.New("session not found")
	ErrCorruptSession  = errors.New("corrupt session")
)

type SessionManager struct {
	redis          *redis.Client
	ttl            time.Duration
	streamRegistry *registry.ActiveStreamRegistry
	secure         bool
	revocations    *RevocationStore
	publicKey      ed25519.PublicKey
	gatewayID      string
	tenantID       string
}

func NewSessionManager(
	redis *redis.Client, 
	ttl time.Duration, 
	streamRegistry *registry.ActiveStreamRegistry, 
	secure bool, gatewayID, tenantID string,
	) (*SessionManager, error) {
	rootKey, err := store.RootPublicKey()
	if err != nil {
		return nil, err
	}
	return &SessionManager{
		redis:          redis,
		ttl:            ttl,
		streamRegistry: streamRegistry,
		secure:         secure,
		revocations:    NewRevocationStore(),
		publicKey:      rootKey.PublicKey,
		gatewayID:      gatewayID,
		tenantID:       tenantID,
	}, nil
}

func sessionKey(sessionID string) string {
	return sessionKeyPrefix + sessionID
}

func cpSessionIndexKey(cpSessionID string) string {
	return cpSessionIndexPrefix + cpSessionID
}

func (sm *SessionManager) Get(r *http.Request) (*proto.NormalizedIdentity, string, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, "", fmt.Errorf("no session cookie")
	}
	data, err := sm.redis.Get(
		r.Context(),
		sessionKey(cookie.Value),
	).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, "", ErrSessionNotFound
		}

		return nil, "", fmt.Errorf("redis get session: %w", err)
	}
	token, err := sm.verifyToken(data)
	if err != nil {
		_ = sm.redis.Del(r.Context(), sessionKey(cookie.Value)).Err()
		return nil, "", err
	}
	if sm.revocations.IsRevoked(token.SessionID) {
		return nil, "", fmt.Errorf("session revoked")
	}

	
	id := proto.NormalizedIdentity{
		UserId: token.Subject, 
		TenantId: token.TenantID, 
		CpSessionId: token.SessionID, 
		Groups: token.Groups,
		Email: token.Email,
		Name: token.Name,
		ProviderId: token.ProviderID,
		Provider: token.Provider,
	}


	return &id, sessionKey(cookie.Value), nil
}

func (sm *SessionManager) Create(w http.ResponseWriter, r *http.Request, sessionToken string) error {

	sid, err := crypto.GenerateSessionID()
	if err != nil {
		return err
	}
	if sessionToken == "" {
		return fmt.Errorf("missing CP session token")
	}
	claims, err := sm.verifyToken(sessionToken)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sessionKey := sessionKey(sid)
	cpIndexKey := cpSessionIndexKey(claims.SessionID)

	// Create both:
	//
	// session:<gateway_session_id>
	// index:cp-session:<cp_session_id> -> SET(gateway_session_id)
	//
	// The pipeline reduces round trips.
	pipe := sm.redis.TxPipeline()

	pipe.Set(
		ctx,
		sessionKey,
		sessionToken,
		sm.ttl,
	)

	pipe.SAdd(
		ctx,
		cpIndexKey,
		sid,
	)

	_, err = pipe.Exec(ctx)

	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	domain := CookieDomain(r.Host)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sid,
		Path:     "/",
		Domain:   domain,
		MaxAge:   int(sm.ttl.Seconds()),
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

type SessionClaims struct {
	TenantID  string   `json:"tid"`
	GatewayID string   `json:"gid"`
	SessionID string   `json:"sid"`
	ProviderID string `json:"pid"`
	Provider string `json:"prov"`
	Email string `json:"email"`
	Name string `json:"name"`
	Groups    []string `json:"groups"`
	jwt.RegisteredClaims
}

func (sm *SessionManager) verifyToken(raw string) (*SessionClaims, error) {
	claims := &SessionClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
			return nil, fmt.Errorf("unexpected session signing algorithm")
		}
		if token.Header["kid"] != "cp-2026-01" {
			return nil, fmt.Errorf("unknown session signing key")
		}
		return sm.publicKey, nil
	},
		jwt.WithIssuer("ashrix-cp"),
		jwt.WithAudience("ashrix-gateway"),
		jwt.WithValidMethods([]string{"EdDSA"}))
		
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid CP session token")
	}
	if claims.IssuedAt == nil || claims.IssuedAt.After(time.Now().Add(30*time.Second)) {
		return nil, fmt.Errorf("invalid CP session issue time")
	}
	if claims.Subject == "" || claims.SessionID == "" || claims.TenantID != sm.tenantID || claims.GatewayID != sm.gatewayID {
		return nil, fmt.Errorf("session claims do not match gateway")
	}
	return claims, nil
}

func (sm *SessionManager) Destroy(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(sessionCookie)

	if err == nil && cookie.Value != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		sid := cookie.Value

		// Retrieve the session first so we know which CP index
		// contains this gateway session.
		data, err := sm.redis.Get(
			ctx,
			sessionKey(sid),
		).Result()

		if err == nil {
			if claims, verifyErr := sm.verifyToken(data); verifyErr == nil {

				pipe := sm.redis.TxPipeline()
				pipe.Del(
					ctx,
					sessionKey(sid),
				)

				pipe.SRem(
					ctx,
					cpSessionIndexKey(claims.SessionID),
					sid,
				)

				_, _ = pipe.Exec(ctx)
			} else {
				_ = sm.redis.Del(ctx, sessionKey(sid)).Err()
			}
		} else if errors.Is(err, redis.Nil) {
			// Session already gone.
		}

	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// RevokeByCPSession revokes every gateway session associated
// with a CP session.
func (sm *SessionManager) RevokeByCPSession(
	ctx context.Context,
	cpSessionID string,
	SessionExpiresAt *timestamppb.Timestamp,
) error {

	if cpSessionID == "" {
		return fmt.Errorf("missing cp session id")
	}

	sm.revocations.Revoke(cpSessionID, SessionExpiresAt.AsTime())

	indexKey := cpSessionIndexKey(cpSessionID)

	sessionIDs, err := sm.redis.SMembers(
		ctx,
		indexKey,
	).Result()

	if err != nil {
		return fmt.Errorf("get cp session index: %w", err)
	}
	if len(sessionIDs) == 0 {
		// Nothing to revoke.
		return nil
	}

	pipe := sm.redis.TxPipeline()

	for _, sid := range sessionIDs {
		sm.streamRegistry.RevokeSession(sid)
		pipe.Del(
			ctx,
			sessionKey(sid),
		)
	}

	// Remove the index itself after revoking all sessions.
	pipe.Del(ctx, indexKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("revoke gateway sessions: %w", err)
	}

	return nil
}


func CookieDomain(host string) string {
	host = strings.TrimSuffix(host, ".")

	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return ""
	}

	return domain
}
