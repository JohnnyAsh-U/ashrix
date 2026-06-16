// Package pki provides a swappable CA signing abstraction for Ashrix.
//
// The pki interface is the single boundary between cert issuance logic
// and key storage. Swap the backend (disk → KMS ) by changing one
// line in your wire-up code. All cert issuance logic stays unchanged.
//
// Backends:
//   - DiskCASigner   → dev / early prod (key on disk, plaintext or encrypted)
//   - KMSCASigner  → HashiCorp Vault PKI engine
package pki

import (
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"io"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
)

// keep io in scope — used by KMSCASigner stub
var _ io.Reader = rand.Reader

// =============================================================================
// THE INTERFACE — the only thing cert issuance code sees
// =============================================================================

// CASigner abstracts Intermediate CA key storage and cert signing.
// Swap the backend by changing one line in New(). Everything else stays.
type CASigner interface {
	// Issue signs a CSR and returns a signed x509 certificate.
	IssueCert(csr *x509.CertificateRequest, validity time.Duration) (*x509.Certificate, error)
	RootCert() *x509.Certificate
	IntermediateCert() *x509.Certificate
	// GetCertPool returns a cached x509.CertPool containing all active CA certificates.
	// It refreshes the pool from the database if it's older than 5 minutes.
	GetCertPool() (*x509.CertPool, error)
}

// New constructs the CASigner for the configured backend.
// This is the only line that changes when you switch backends.
func NewSigner(cfg *config.PKIConfig, dbQueries *store.Queries) (CASigner, error) {
	switch cfg.Backend {
	case config.BackendDisk:
		return NewDiskCASigner(cfg.BasePath, cfg.PKIUnlockSecret, dbQueries)
	case config.BackendKMS:
		return NewKMSCASigner(cfg.KMSToken, cfg.KMSUrl, cfg.BasePath, cfg.PKIUnlockSecret, dbQueries) // KMSCASigner also needs dbQueries
	default:
		return nil, fmt.Errorf("unknown backend: %q", cfg.Backend)
	}
}

// Compile-time interface checks.
// If a backend is missing a method, this fails the build immediately.
var (
	_ CASigner = (*DiskCASigner)(nil)
	_ CASigner = (*KMSCASigner)(nil)
)
