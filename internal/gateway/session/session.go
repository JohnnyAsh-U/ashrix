package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/redis/go-redis/v9"
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
	redis  *redis.Client
	ttl    time.Duration
	secure bool
}

func NewSessionManager(redis *redis.Client, ttl time.Duration, secure bool) *SessionManager {
	return &SessionManager{redis: redis, ttl: ttl, secure: secure}
}

func sessionKey(sessionID string) string {
	return sessionKeyPrefix + sessionID
}

func cpSessionIndexKey(cpSessionID string) string {
	return cpSessionIndexPrefix + cpSessionID
}

func (sm *SessionManager) Get(r *http.Request) (*proto.NormalizedIdentity, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("no session cookie")
	}
	data, err := sm.redis.Get(
		r.Context(),
		sessionKey(cookie.Value),
	).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}

		return nil, fmt.Errorf("redis get session: %w", err)
	}
	var id proto.NormalizedIdentity
	if err := json.Unmarshal([]byte(data), &id); err != nil {
		return nil, fmt.Errorf("corrupt session")
	}
	return &id, nil
}

func (sm *SessionManager) Create(w http.ResponseWriter, r *http.Request, identity *proto.NormalizedIdentity) error {

	if identity == nil {
		return fmt.Errorf("identity is nil")
	}

	if identity.CpSessionId == "" {
		return fmt.Errorf("missing cp session id")
	}

	sid, err := crypto.GenerateSessionID()
	if err != nil {
		return err
	}
	identity.AuthTime = timestamppb.Now()
	data, err := json.Marshal(identity)
	if err != nil {
		return fmt.Errorf("marshal identity: %w", err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sessionKey := sessionKey(sid)
	cpIndexKey := cpSessionIndexKey(identity.CpSessionId)

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
		data,
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

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sid,
		Path:     "/",
		Domain:   ".ashrix.io",
		MaxAge:   int(sm.ttl.Seconds()),
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
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
			var identity proto.NormalizedIdentity

			if json.Unmarshal([]byte(data), &identity) == nil &&
				identity.CpSessionId != "" {

				pipe := sm.redis.TxPipeline()
				pipe.Del(
					ctx,
					sessionKey(sid),
				)

				pipe.SRem(
					ctx,
					cpSessionIndexKey(identity.CpSessionId),
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
) error {

	if cpSessionID == "" {
		return fmt.Errorf("missing cp session id")
	}

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

// RevokeGatewaySession revokes one specific gateway session.
func (sm *SessionManager) RevokeGatewaySession(
	ctx context.Context,
	sessionID string,
) error {

	if sessionID == "" {
		return fmt.Errorf("missing session id")
	}

	data, err := sm.redis.Get(
		ctx,
		sessionKey(sessionID),
	).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return fmt.Errorf("get gateway session: %w", err)
	}

	var identity proto.NormalizedIdentity

	if err := json.Unmarshal([]byte(data), &identity); err != nil {
		// Session is corrupt, but we can still delete it.
		if err := sm.redis.Del(ctx, sessionKey(sessionID)).Err(); err != nil {
			return fmt.Errorf("delete corrupt session: %w", err)
		}

		return ErrCorruptSession
	}

	pipe := sm.redis.TxPipeline()

	pipe.Del(
		ctx,
		sessionKey(sessionID),
	)
	if identity.CpSessionId != "" {
		pipe.SRem(
			ctx,
			cpSessionIndexKey(identity.CpSessionId),
			sessionID,
		)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("revoke gateway session: %w", err)
	}

	return nil
}
