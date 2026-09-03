package startup

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"sync"
	"time"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"
	// pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
)

// PKIInitialiser holds the connector's active PKI state.
// Built after startup completes — used for all subsequent connections.
//
// Holds:
//   - Current cert and private key (in memory)
//   - TLSConfig for mTLS connections to gateway
//   - Renew() for forced renewal
//   - StartRotator() for background rotation
type PKIInitialiser struct {
	mu      sync.RWMutex
	tlscert *tls.Certificate  // ready for TLS stack
	leaf    *x509.Certificate // parsed, for expiry checks
	pool    *x509.CertPool    //Trustbundle for tls
	privKey *ecdsa.PrivateKey // current key — needed for possession proof

	connectorID string
	cpURL       string
	log         *slog.Logger
	appStorage  storage.Storage

	certPath  string
	keyPath   string
	trustPath string

	context string
	secret  string

	stopCh chan struct{}
	once   sync.Once
}

// New builds a PKIInitialiser from already-loaded cert and key.
// Called after the startup workflow completes.
func NewPKIInitialiser(
	cert *x509.Certificate,
	privKey *ecdsa.PrivateKey,
	pool string,
	connectorID string,
	cpURL string,
	log *slog.Logger,
	appStorage storage.Storage,
) (*PKIInitialiser, error) {

	CertPool := x509.NewCertPool()

	if !CertPool.AppendCertsFromPEM([]byte(pool)) {
		return nil, fmt.Errorf("Failed to append certificate to pool")
	}

	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  privKey,
		Leaf:        cert,
	}

	return &PKIInitialiser{
		tlscert:     &tlsCert,
		leaf:        cert,
		pool:        CertPool,
		appStorage:  appStorage,
		privKey:     privKey,
		connectorID: connectorID,
		cpURL:       cpURL,
		log:         log,
		stopCh:      make(chan struct{}),
	}, nil
}

// Cert returns the current parsed certificate.
func (p *PKIInitialiser) Cert() *x509.Certificate {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.leaf
}

// TLSConfig returns a *tls.Config that always serves the current cert.
func (p *PKIInitialiser) TLSConfig() *tls.Config {
	return &tls.Config{
		// Outbound: connector dials gateway
		GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			p.mu.RLock()
			defer p.mu.RUnlock()
			return p.tlscert, nil
		},

		RootCAs:    p.pool,
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{"ashrix-mtls", "h2", "ashrix-quic-v1"},
	}
}

// ── Renewal ───────────────────────────────────────────────────────────────────

// Renew performs an immediate cert renewal with the CP.
// Generates new keypair, sends possession proof, hot-swaps on success.
// Called by: StartRotator() when cert is within 30 days of expiry.
// Called by: gateway rotation command.
func (p *PKIInitialiser) Renew(ctx context.Context, force bool) error {

	// ── Read current state (read lock — fast) ─────────────────────
	p.mu.RLock()
	remaining := time.Until(p.leaf.NotAfter)
	currentKey := p.privKey
	p.mu.RUnlock()

	// Another goroutine may have already renewed
	if !force && remaining > 30*24*time.Hour {
		p.log.Debug("renewal skipped — cert still fresh",
			"remaining", remaining,
		)
		return nil
	}

	p.log.Info("renewing certificate",
		"remaining", remaining,
	)

	// ── Call CP for new cert (no lock held — network I/O) ─────────
	result, newKey, err := Renew(ctx, p.cpURL, p.connectorID, p.log, currentKey)
	if err != nil {
		return fmt.Errorf("renew: %w", err)
	}


	if err := p.appStorage.SaveCredential(result, newKey); err != nil {
		return err
	}

	p.log.Info("credentials saved to disk",
		"cert", p.certPath,
		"key", p.keyPath,
	)

	cred, err := p.appStorage.LoadCredential()
	if err != nil {
		return fmt.Errorf("failed to load credential: %w", err)
	}

	// Sanity check — CP must have extended the expiry
	if !cred.Cert.NotAfter.After(p.leaf.NotAfter) {
		return fmt.Errorf("renew: CP returned cert with same or earlier expiry")
	}

	CertPool := x509.NewCertPool()

	if !CertPool.AppendCertsFromPEM([]byte(result.TrustBundle)) {
		return fmt.Errorf("Failed to append certificate to pool")
	}

	tlsCert := tls.Certificate{
		Certificate: [][]byte{cred.Cert.Raw},
		PrivateKey:  cred.PrivKey,
		Leaf:        cred.Cert,
	}

	// ── Hot-swap under write lock (fast — pointer swaps only) ─────
	// TLS handshakes are never blocked — lock held for microseconds
	p.mu.Lock()
	p.tlscert = &tlsCert
	p.leaf = cred.Cert
	p.privKey = cred.PrivKey
	p.pool = CertPool
	p.mu.Unlock()

	p.log.Info("certificate renewed and hot-swapped",
		"new_expiry", cred.Cert.NotAfter,
		"valid_for", time.Until(cred.Cert.NotAfter),
	)
	return nil
}

// ── Rotator ───────────────────────────────────────────────────────────────────

// StartRotator begins the background cert rotation loop.
// Checks expiry every 5 minutes.
// Renews when less than 30 days remain.
// Call this after the management stream is established.
func (p *PKIInitialiser) StartRotator() {
	go p.rotator()
}

// Stop shuts down the rotator. Safe to call multiple times.
func (p *PKIInitialiser) Stop() {
	p.once.Do(func() { close(p.stopCh) })
}

func (p *PKIInitialiser) rotator() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	p.log.Debug("cert rotator started")

	for {
		select {
		case <-ticker.C:
			p.mu.RLock()
			remaining := time.Until(p.leaf.NotAfter)
			p.mu.RUnlock()

			p.log.Debug("cert expiry check",
				"remaining", remaining,
				"expires_at", p.leaf.NotAfter,
			)

			if remaining < 30*24*time.Hour {
				p.log.Info("cert approaching expiry — renewing",
					"remaining", remaining,
				)
				if err := p.Renew(context.Background(), true); err != nil {
					p.log.Error("background renewal failed",
						"error", err,
					)
					// Rotator will retry on next tick (5 min)
				}
			}

		case <-p.stopCh:
			p.log.Debug("cert rotator stopped")
			return
		}
	}
}
