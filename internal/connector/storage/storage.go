package storage

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"runtime"
	"time"
)

// For key encryption
var (
	secret  = "SECRET"
	context = "CONNECTOR"
)

type Cred struct {
	PrivKey *ecdsa.PrivateKey
	Cert    *x509.Certificate
	Bundle  string
}

type Storage interface {
	LoadCredential() (*Cred, error)
	ClearCredential() error
	CredentialExists() bool
}

func NewStorage() (Storage, error) {
	switch runtime.GOOS {
	case "windows":
		return NewWindowsStorage()
	case "linux":
		return NewLinuxStorage()
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func (c *Cred) IsExpired() bool {
	return time.Now().After(c.Cert.NotAfter)
}
