package pki

import (
	"context" // Added for database operations
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql" // For sql.ErrNoRows
	"encoding/pem" // For PEM encoding/decoding
	"errors"       // For errors.Is
	"fmt"
	"log/slog"
	"os"

	// "os"
	"path/filepath"
	"sync" // For mutex
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store" // Added for database queries
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
)

// Layout holds all resolved paths for the PKI directory.
type DiskCASigner struct {
	RootCAKeyEncPath string
	RootCACertPath   string
	RootCACert       *x509.Certificate

	intermediateKey        *ecdsa.PrivateKey
	intermediateCert       *x509.Certificate
	IntermediateKeyEncPath string
	IntermediateCACertPath string

	pkicaRepo pkica.Repository

	trustBundleVersion int64

	certPool            *x509.CertPool
	certPoolMutex       sync.RWMutex
	certPoolLastRefresh time.Time

	log *slog.Logger
}

func NewDiskCASigner(baseDir, secret string, pkicaRepo pkica.Repository, log *slog.Logger) (*DiskCASigner, error) {
	log.Info("Initializing PKI...")
	PkiDir := filepath.Join(baseDir, "pki")
	if err := os.MkdirAll(PkiDir, 0700); err != nil {
		return nil, fmt.Errorf("creating PKI directory: %w", err)
	}

	rootKeyPath := filepath.Join(PkiDir, "root-ca.key.enc")
	rootCertPath := filepath.Join(PkiDir, "root-ca.crt")
	interKeyPath := filepath.Join(PkiDir, "intermediate-ca.key.enc")
	interCertPath := filepath.Join(PkiDir, "intermediate-ca.crt")
	var intermediateKey *ecdsa.PrivateKey
	var intermediateCert *x509.Certificate
	var rootCert *x509.Certificate
	var rootKey *ecdsa.PrivateKey

	ctx := context.Background() // Use a background context for startup operations

	// --- Root CA Loading/Generation ---
	log.Info("Attempting to load Root CA...")
	dbRootCertRecord, err := pkicaRepo.GetActiveCACert(ctx, store.GetActiveCACertParams{Name: "Ashrix Root CA", Type: "root"})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query active root CA from DB: %w", err)
	}

	if dbRootCertRecord.CertPem != "" { // Root CA found in DB
		log.Info("→ Loading Root CA from database.")
		rootCert, err = parseCertPEM([]byte(dbRootCertRecord.CertPem))
		if err != nil {
			return nil, fmt.Errorf("failed to parse root CA certificate from DB: %w", err)
		}
	} else { // Root CA not in DB, check disk or generate
		log.Info("→ Root CA not found in database. Checking disk for existing files...")
		rootCertExists, err := filehelper.FileExists(rootCertPath)
		if err != nil {
			return nil, fmt.Errorf("checking root cert existence: %w", err)
		}

		if rootCertExists {
			log.Info("→ Loading Root CA from disk.")
			rootCert, err = pki.LoadCert(rootCertPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load root CA cert from disk: %w", err)
			}
		} else {
			log.Info("→ Root CA not found on disk. Generating new Root CA...")
			rootKey, rootCert, err = generateRootCA(rootKeyPath, rootCertPath, secret, "root", log)
			if err != nil {
				return nil, fmt.Errorf("failed to generate root CA: %w", err)
			}
		}

		// Insert the loaded/generated Root CA into the database
		_, err = pkicaRepo.CreateCACert(ctx, store.InsertCACertParams{
			Name:         "Ashrix Root CA",
			Type:         "root",
			CertPem:      string(encodeCertPEM(rootCert)),
			SerialNumber: rootCert.SerialNumber.String(),
			Subject:      rootCert.Subject.CommonName,
			IssuedAt:     rootCert.NotBefore,
			ExpiresAt:    rootCert.NotAfter,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to insert generated root CA into DB: %w", err)
		}
		log.Info("→ Root CA successfully stored in database.")
	}

	// --- Intermediate CA Loading/Generation ---
	log.Info("Attempting to load Intermediate CA...")
	dbIntermediateCertRecord, err := pkicaRepo.GetActiveCACert(ctx, store.GetActiveCACertParams{Name: "Ashrix Intermediate CA", Type: "intermediate"})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query active intermediate CA from DB: %w", err)
	}

	if dbIntermediateCertRecord.CertPem != "" { // Intermediate CA found in DB
		log.Info("→ Loading Intermediate CA from database.")
		intermediateCert, err = parseCertPEM([]byte(dbIntermediateCertRecord.CertPem))
		if err != nil {
			return nil, fmt.Errorf("failed to parse intermediate CA certificate from DB: %w", err)
		}
		// Key is never stored in DB, must be on disk
		intermediateKey, err = pki.LoadKey(interKeyPath, secret, "intermediate") // Assumes pki.LoadKey exists
		if err != nil {
			return nil, fmt.Errorf("failed to load intermediate CA key from disk: %w", err)
		}
	} else { // Intermediate CA not in DB, check disk or generate
		log.Info("→ Intermediate CA not found in database. Checking disk for existing files...")
		interKeyExists, err := filehelper.FileExists(interKeyPath)
		if err != nil {
			return nil, fmt.Errorf("checking intermediate key existence: %w", err)
		}
		interCertExists, err := filehelper.FileExists(interCertPath)
		if err != nil {
			return nil, fmt.Errorf("checking intermediate cert existence: %w", err)
		}

		if interKeyExists && interCertExists {
			log.Info("→ Loading Intermediate CA from disk.")
			intermediateKey, intermediateCert, err = pki.LoadKeyAndCert(interKeyPath, interCertPath, secret, "intermediate")
			if err != nil {
				return nil, fmt.Errorf("failed to load intermediate CA from disk: %w", err)
			}
		} else {
			log.Info("→ Intermediate CA not found on disk. Generating new Intermediate CA...")

			// Root key is strictly required for signing a new intermediate.
			// If it wasn't just generated, we must load it from disk now.
			if rootKey == nil {
				rootKey, err = pki.LoadKey(rootKeyPath, secret, "root")
				if err != nil {
					return nil, fmt.Errorf("intermediate CA generation required, but root key is missing or inaccessible: %w", err)
				}
			}

			intermediateKey, intermediateCert, err = generateIntermediateCA(interKeyPath, interCertPath, secret, rootKey, rootCert, log)
			if err != nil {
				return nil, fmt.Errorf("failed to generate intermediate CA: %w", err)
			}
		}

		// Insert the loaded/generated Intermediate CA into the database
		_, err = pkicaRepo.CreateCACert(ctx, store.InsertCACertParams{
			Name:         "Ashrix Intermediate CA",
			Type:         "intermediate",
			CertPem:      string(encodeCertPEM(intermediateCert)),
			SerialNumber: intermediateCert.SerialNumber.String(),
			Subject:      intermediateCert.Subject.CommonName,
			IssuedAt:     intermediateCert.NotBefore,
			ExpiresAt:    intermediateCert.NotAfter,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to insert generated intermediate CA into DB: %w", err)
		}
		log.Info("→ Intermediate CA successfully stored in database.")
	}

	// --- Verification ---
	log.Info("Performing PKI verification checks...")
	// Verify intermediate key matches its certificate
	if err := pki.VerifyKeyAndCert(intermediateCert, intermediateKey); err != nil {
		return nil, fmt.Errorf("intermediate CA key-certificate mismatch: %w", err)
	}
	log.Info("  ✓ Intermediate CA key matches certificate.")

	// Verify intermediate certificate was signed by the root certificate
	if err := verifyChain(intermediateCert, rootCert); err != nil {
		return nil, fmt.Errorf("intermediate CA not signed by Root CA: %w", err)
	}
	log.Info("  ✓ Intermediate CA signed by Root CA.")

	log.Info("PKI Initializing Done...")
	d := &DiskCASigner{
		RootCAKeyEncPath:       rootKeyPath,
		RootCACertPath:         rootCertPath,
		IntermediateKeyEncPath: interKeyPath,
		IntermediateCACertPath: interCertPath,
		intermediateKey:        intermediateKey,
		intermediateCert:       intermediateCert,
		RootCACert:             rootCert,
		pkicaRepo:              pkicaRepo,
		certPoolMutex:          sync.RWMutex{},
		log: log,
	}

	// Initial refresh of the cert pool
	if err := d.refreshCertPool(ctx); err != nil {
		return nil, fmt.Errorf("failed to initially refresh cert pool: %w", err)
	}

	return d, nil
}

func (d *DiskCASigner) IssueCert(csr *x509.CertificateRequest, validity time.Duration, CN, DNSName string) (*x509.Certificate, error) {
	template, err := pki.BuildCertTemplate(csr, validity, CN, DNSName)
	if err != nil {
		return nil, err
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, d.intermediateCert, csr.PublicKey, d.intermediateKey)
	if err != nil {
		return nil, fmt.Errorf("signing certificate: %w", err)
	}
	return x509.ParseCertificate(certDER)
}

func (d *DiskCASigner) RootCert() *x509.Certificate {
	return d.RootCACert
}

func (d *DiskCASigner) IntermediateCert() *x509.Certificate {
	return d.intermediateCert
}

func (d *DiskCASigner) TrustBundle() string {
	if d.intermediateCert == nil || d.RootCACert == nil {
		return ""
	}
	interPEM := pki.MarshalCert(d.intermediateCert)
	rootPEM := pki.MarshalCert(d.RootCACert)
	return string(interPEM) + string(rootPEM)
}

// GetCertPool returns a cached x509.CertPool containing all active CA certificates.
// It refreshes the pool from the database if it's older than 5 minutes.
func (d *DiskCASigner) GetCertPool() (*x509.CertPool, error) {
	d.certPoolMutex.RLock()
	// Check if the cached pool is still fresh
	if d.certPool != nil && time.Since(d.certPoolLastRefresh) < 5*time.Minute {
		defer d.certPoolMutex.RUnlock()
		return d.certPool, nil
	}
	d.certPoolMutex.RUnlock() // Release read lock before acquiring write lock

	// Pool is stale or not initialized, acquire write lock to refresh
	d.certPoolMutex.Lock()
	defer d.certPoolMutex.Unlock()

	// Double-check condition after acquiring write lock (another goroutine might have refreshed)
	if d.certPool != nil && time.Since(d.certPoolLastRefresh) < 5*time.Minute {
		return d.certPool, nil
	}

	// Refresh the cert pool from the database
	if err := d.refreshCertPool(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to refresh cert pool: %w", err)
	}

	return d.certPool, nil
}

// refreshCertPool loads all active CA certificates from the database and rebuilds the certPool.
// This method should be called with the write lock held on certPoolMutex.
func (d *DiskCASigner) refreshCertPool(ctx context.Context) error {
	d.log.Info("Refreshing CA CertPool from database...")

	newPool := x509.NewCertPool()

	//Intermediate CA
	intermediateCA, err := d.pkicaRepo.ListActiveCACerts(ctx, store.ListActiveCACertsParams{
		Name: "Ashrix Intermediate CA",
		Type: "intermediate",
	})
	if err != nil {
		return fmt.Errorf("failed to list active CA certs from DB: %w", err)
	}

	for _, ca := range intermediateCA {
		intermediateCert, err := parseCertPEM([]byte(ca.CertPem))
		if err != nil {
			// Log the error but continue with other certificates
			d.log.Info("Warning: failed to parse certificate %s (ID: %s) from DB: %v\n", ca.Name, ca.ID.String())
			return fmt.Errorf("failed to parse intermediate CA cert: %w", err)
		}
		newPool.AddCert(intermediateCert)
	}

	//Root CA
	rootCA, err := d.pkicaRepo.GetActiveCACert(ctx, store.GetActiveCACertParams{
		Name: "Ashrix Root CA",
		Type: "root",
	})
	if err != nil {
		return fmt.Errorf("failed to list active CA certs from DB: %w", err)
	}
	rootCert, err := parseCertPEM([]byte(rootCA.CertPem))
	if err != nil {
		// Log the error but continue with other certificates
		d.log.Info("Warning: failed to parse certificate %s (ID: %s) from DB: %v\n", rootCA.Name, rootCA.ID.String())
		return fmt.Errorf("failed to parse root CA cert: %w", err)
	}
	newPool.AddCert(rootCert)

	d.certPool = newPool
	d.certPoolLastRefresh = time.Now()
	d.log.Info("CertPool refreshed with active certificates.\n")
	return nil
}

// parseCertPEM decodes a PEM-encoded certificate.
func parseCertPEM(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("failed to decode PEM block containing certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

// encodeCertPEM encodes an x509.Certificate to PEM format.
func encodeCertPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func generateRootCA(rootCAKeyPath, rootCACertPath, secret, context string, log *slog.Logger) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	key, _, err := pki.GenerateCertificateSigningKey(rootCAKeyPath, rootCACertPath, secret, context)
	if err != nil { // Added error check for GenerateSigningKey
		return nil, nil, fmt.Errorf("failed to generate signing key for root CA: %w", err)
	}
	serial, err := pki.RandomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Ashrix Root CA", Organization: []string{"Ashrix"}},
		NotBefore:             now.Add(-30 * time.Second),
		NotAfter:              now.Add(20 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
		MaxPathLenZero:        false,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("self-signing root CA: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	if err := pki.WriteCert(rootCACertPath, cert); err != nil {
		return nil, nil, err
	}

	log.Info("✓ Root CA generated.")
	return key, cert, nil
}

func generateIntermediateCA(intermediateKeyPath, intermediateCertPath, secret string, rootKey *ecdsa.PrivateKey, rootCert *x509.Certificate, log *slog.Logger) (*ecdsa.PrivateKey, *x509.Certificate, error) {

	key, pub, err := pki.GenerateCertificateSigningKey(intermediateKeyPath, intermediateCertPath, secret, "intermediate")
	if err != nil { // Added error check for GenerateSigningKey
		return nil, nil, fmt.Errorf("failed to generate signing key for intermediate CA: %w", err)
	}

	serial, err := pki.RandomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber:                serial,
		Subject:                     pkix.Name{CommonName: "Ashrix Intermediate CA", Organization: []string{"Ashrix"}},
		NotBefore:                   now.Add(-30 * time.Second),
		NotAfter:                    now.Add(365 * 24 * time.Hour),
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid:       true,
		IsCA:                        true,
		MaxPathLen:                  0,
		MaxPathLenZero:              true,
		// PermittedDNSDomainsCritical: true,
		// PermittedDNSDomains:         []string{".ashrix.internal", "ashrix.internal", "localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, rootCert, pub, rootKey)
	if err != nil {
		return nil, nil, fmt.Errorf("signing intermediate CA: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	// Verify chain before writing
	if err := verifyChain(cert, rootCert); err != nil {
		return nil, nil, fmt.Errorf("chain verification failed: %w", err)
	}

	if err := pki.WriteCert(intermediateCertPath, cert); err != nil {
		return nil, nil, err
	}

	log.Info("  ✓ Intermediate CA generated.")
	log.Info("  ✓ Chain verified: Intermediate CA → Root CA\n")
	return key, cert, nil
}

func verifyChain(cert, rootCert *x509.Certificate) error {
	pool := x509.NewCertPool()
	pool.AddCert(rootCert)
	_, err := cert.Verify(x509.VerifyOptions{Roots: pool})
	return err
}
