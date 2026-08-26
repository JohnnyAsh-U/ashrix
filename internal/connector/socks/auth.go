package socks

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
)

const (
	MaxAppIDLength  = 255
	MaxPasswordSize = 255
)

type Credential struct {
	AppID        string
	PasswordHash string
}

type CredentialStore struct {
	mu          sync.RWMutex
	credentials map[string]string

	// Used to make unknown-user authentication less distinguishable
	// from known-user authentication.
	dummyHash []byte
}

func NewCredentialStore() (*CredentialStore, error) {
	// Generate this once. It is never used as a real credential.
	dummy, err := bcrypt.GenerateFromPassword(
		[]byte("ashrix-dummy-password"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, err
	}

	return &CredentialStore{
		credentials: make(map[string]string),
		dummyHash:   dummy,
	}, nil
}

// Replace atomically replaces the complete credential snapshot.
//
// This is what your CP -> Connector synchronization should call.
func (s *CredentialStore) Replace(
	credentials []Credential,
) error {
	next := make(map[string]string, len(credentials))

	for _, credential := range credentials {
		if credential.AppID == "" ||
			len(credential.AppID) > MaxAppIDLength {
			return errors.New("invalid app id")
		}

		if credential.PasswordHash == "" {
			return errors.New("empty password hash")
		}

		next[credential.AppID] = credential.PasswordHash
	}

	s.mu.Lock()
	s.credentials = next
	s.mu.Unlock()

	return nil
}



func (s *CredentialStore) Authenticate(
	ctx context.Context,
	appID string,
	password string,
) error {
	if appID == "" ||
		len(appID) > MaxAppIDLength ||
		len(password) > MaxPasswordSize {
		return ErrInvalidCredentials
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.RLock()
	hash, exists := s.credentials[appID]
	s.mu.RUnlock()

	if !exists {
		// Perform bcrypt anyway so that "unknown app" and
		// "wrong password" don't have radically different timing.
		_ = bcrypt.CompareHashAndPassword(
			s.dummyHash,
			[]byte(password),
		)

		return ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	); err != nil {
		return ErrInvalidCredentials
	}

	return nil
}