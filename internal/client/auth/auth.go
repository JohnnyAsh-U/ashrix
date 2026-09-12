package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/storage"
	"github.com/pkg/browser"
)

type AuthService struct {
	cfg   *config.Config
	store storage.Store
}

func NewAuthService(cfg *config.Config, store storage.Store) *AuthService {
	return &AuthService{
		cfg:   cfg,
		store: store,
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Data struct {
		SessionToken string `json:"session_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			ID       string `json:"id"`
			Email    string `json:"email"`
			TenantID string `json:"tenant_id"`
		} `json:"user"`
	} `json:"data"`
	Error string `json:"error"`
}

// Login handles OIDC browser authentication flow or direct CP session authentication
func (a *AuthService) Login(ctx context.Context, email, password string) (*storage.SessionCredentials, error) {
	// If credentials are supplied directly via CLI flags/env or interactive prompt
	if email != "" && password != "" {
		return a.loginWithPassword(ctx, email, password)
	}

	// Interactive browser authentication callback flow
	return a.loginWithBrowser(ctx)
}

func (a *AuthService) loginWithPassword(ctx context.Context, email, password string) (*storage.SessionCredentials, error) {
	reqBody, _ := json.Marshal(LoginRequest{Email: email, Password: password})
	url := fmt.Sprintf("%s/v1/auth/login", a.cfg.ControlPlaneURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Control Plane at %s: %w", a.cfg.ControlPlaneURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	var res LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to parse login response: %w", err)
	}

	token := res.Data.SessionToken
	if token == "" {
		token = res.Data.AccessToken
	}
	if token == "" {
		return nil, fmt.Errorf("control plane returned empty session token")
	}

	creds := &storage.SessionCredentials{
		SessionToken: token,
		RefreshToken: res.Data.RefreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		UserEmail:    res.Data.User.Email,
		UserID:       res.Data.User.ID,
		TenantID:     res.Data.User.TenantID,
	}

	if err := a.store.Save(creds); err != nil {
		return nil, fmt.Errorf("failed to store credentials: %w", err)
	}

	return creds, nil
}

func (a *AuthService) loginWithBrowser(ctx context.Context) (*storage.SessionCredentials, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to open local callback port: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			if token == "" {
				token = r.URL.Query().Get("session_token")
			}
			if token != "" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("<html><body><h2>Ashrix CLI Login Successful! You may close this browser tab.</h2></body></html>"))
				codeChan <- token
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("<html><body><h2>Login failed: session token missing in callback.</h2></body></html>"))
			errChan <- fmt.Errorf("missing token in callback")
		}),
	}

	go func() {
		_ = server.Serve(listener)
	}()
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	authURL := fmt.Sprintf("%s/v1/auth/browser-login?redirect_uri=http://127.0.0.1:%d/callback", a.cfg.ControlPlaneURL, port)
	fmt.Fprintf(os.Stderr, "Opening browser for Ashrix authentication...\nIf browser does not open automatically, visit:\n  %s\n\n", authURL)
	_ = browser.OpenURL(authURL)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, err
	case token := <-codeChan:
		creds := &storage.SessionCredentials{
			SessionToken: token,
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}
		if err := a.store.Save(creds); err != nil {
			return nil, fmt.Errorf("failed to save session token: %w", err)
		}
		return creds, nil
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("login timed out waiting for browser callback")
	}
}

func (a *AuthService) Logout(ctx context.Context) error {
	return a.store.Clear()
}

func (a *AuthService) GetCurrentSession() (*storage.SessionCredentials, error) {
	creds, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	if time.Now().After(creds.ExpiresAt) {
		_ = a.store.Clear()
		return nil, fmt.Errorf("session has expired. Please run 'ashrix login'")
	}
	return creds, nil
}
