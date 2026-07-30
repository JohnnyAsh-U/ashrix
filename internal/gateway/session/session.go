package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const sessionCookie = "__Host-session"

type SessionManager struct {
	redis  *redis.Client
	ttl    time.Duration
	secure bool
}

func NewSessionManager(redis *redis.Client, ttl time.Duration, secure bool) *SessionManager {
	return &SessionManager{redis: redis, ttl: ttl, secure: secure}
}

func (sm *SessionManager) Get(r *http.Request) (*proto.NormalizedIdentity, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("no session cookie")
	}
	data, err := sm.redis.Get(r.Context(), fmt.Sprintf("session:%s", cookie.Value)).Result()
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}
	var id proto.NormalizedIdentity
	if err := json.Unmarshal([]byte(data), &id); err != nil {
		return nil, fmt.Errorf("corrupt session")
	}
	return &id, nil
}

func (sm *SessionManager) Create(w http.ResponseWriter, r *http.Request, identity *proto.NormalizedIdentity) error {
	sid, err := crypto.GenerateSessionID()
	if err != nil {
		return err
	}
	identity.AuthTime = timestamppb.Now()
	data, _ := json.Marshal(identity)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := sm.redis.Set(ctx, fmt.Sprintf("session:%s", sid), data, sm.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sid,
		Path:     "/",
		MaxAge:   int(sm.ttl.Seconds()),
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (sm *SessionManager) Destroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err == nil && cookie.Value != "" {
		sm.redis.Del(r.Context(), fmt.Sprintf("session:%s", cookie.Value))
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
}