package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/config"
	"github.com/zalando/go-keyring"
)

const (
	serviceName = "ashrix-cli"
	tokenKey    = "session_token"
)

type SessionCredentials struct {
	SessionToken string    `json:"session_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserEmail    string    `json:"user_email,omitempty"`
	UserID       string    `json:"user_id,omitempty"`
	TenantID     string    `json:"tenant_id,omitempty"`
}

type Store interface {
	Save(creds *SessionCredentials) error
	Load() (*SessionCredentials, error)
	Clear() error
}

type SecureStore struct{}

func NewSecureStore() *SecureStore {
	return &SecureStore{}
}

func (s *SecureStore) Save(creds *SessionCredentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to serialize session credentials: %w", err)
	}

	// Try OS keyring first
	if keyringErr := keyring.Set(serviceName, tokenKey, string(data)); keyringErr == nil {
		return nil
	}

	// Fallback to secure file storage (mode 0600)
	return s.saveToFile(data)
}

func (s *SecureStore) Load() (*SessionCredentials, error) {
	// Try OS keyring first
	if secret, keyringErr := keyring.Get(serviceName, tokenKey); keyringErr == nil && secret != "" {
		var creds SessionCredentials
		if err := json.Unmarshal([]byte(secret), &creds); err == nil {
			return &creds, nil
		}
	}

	// Fallback to secure file
	return s.loadFromFile()
}

func (s *SecureStore) Clear() error {
	_ = keyring.Delete(serviceName, tokenKey)

	dir, err := config.GetConfigDir()
	if err != nil {
		return nil
	}
	sessionFile := filepath.Join(dir, "session.json")
	if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session file: %w", err)
	}
	return nil
}

func (s *SecureStore) saveToFile(data []byte) error {
	dir, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	sessionFile := filepath.Join(dir, "session.json")
	if err := os.WriteFile(sessionFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write secure session file: %w", err)
	}
	return nil
}

func (s *SecureStore) loadFromFile() (*SessionCredentials, error) {
	dir, err := config.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("config directory error: %w", err)
	}
	sessionFile := filepath.Join(dir, "session.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no active Ashrix session found. Please run 'ashrix login'")
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var creds SessionCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("corrupt session file: %w", err)
	}

	return &creds, nil
}
