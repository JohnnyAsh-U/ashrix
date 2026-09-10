package session

import (
	"sync"
	"time"
)

type RevocationStore struct {
	mu      sync.RWMutex
	revoked map[string]time.Time
}

func NewRevocationStore() *RevocationStore {
	return &RevocationStore{revoked: make(map[string]time.Time)}
}

func (s *RevocationStore) Revoke(sessionID string, expiresAt time.Time) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[sessionID] = expiresAt
}

func (s *RevocationStore) IsRevoked(sessionID string) bool {
	s.mu.RLock()
	expiresAt, ok := s.revoked[sessionID]
	s.mu.RUnlock()
	return ok && time.Now().Before(expiresAt)
}

func (s *RevocationStore) RemoveExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for sessionID, expiresAt := range s.revoked {
		if !now.Before(expiresAt) {
			delete(s.revoked, sessionID)
		}
	}
}
