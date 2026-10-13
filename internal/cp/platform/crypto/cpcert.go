package crypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgtype"
)

// =============================================================================
// THE INTERFACE — the only thing conrolplanesigner code sees
// =============================================================================

type ControlPlaneCrypto interface {
	// Signs with private key
	CPCert() *x509.Certificate
	TLSCertificate() tls.Certificate
	CACertPool() *x509.CertPool
}

type ControlPlanePKI struct {
	ControlPlaneKeyPath  string
	ControlPlaneCertPath string
	ControlPlaneKey      *ecdsa.PrivateKey
	ControlPlaneCert     *x509.Certificate
	CAPool               *x509.CertPool
	CARepo               pkica.Repository
}

func ControlPlanePKIIntializer(baseDir string, secret string, signer pki.CASigner, pki_ca pkica.Repository, log *slog.Logger) (ControlPlaneCrypto, error) {
	return LoadControlPlanePKI(baseDir, secret, signer, pki_ca, log)
}

func ProvisionControlPlanePKI(baseDir, secret string, signer pki.CASigner, pkiCA pkica.Repository, log *slog.Logger) (ControlPlaneCrypto, error) {
	log.Info("Provisioning Control Plane PKI...")
	cpPKIDir := filepath.Join(baseDir, "cp")
	if err := os.MkdirAll(cpPKIDir, 0700); err != nil {
		return nil, fmt.Errorf("creating PKI directory: %w", err)
	}

	cpKeyPath := filepath.Join(cpPKIDir, "cp.key.enc")
	cpCertPath := filepath.Join(cpPKIDir, "cp.crt")
	keyExists, err := filehelper.FileExists(cpKeyPath)
	if err != nil {
		return nil, err
	}
	certExists, err := filehelper.FileExists(cpCertPath)
	if err != nil {
		return nil, err
	}
	if keyExists != certExists {
		return nil, fmt.Errorf("incomplete Control Plane certificate files at %s; refusing to replace them", cpPKIDir)
	}

	ctx := context.Background()
	dbCert, dbErr := pkiCA.GetActiveComponentCertByType(ctx, "cp")
	if dbErr != nil && !errors.Is(dbErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get active CP component certificate: %w", dbErr)
	}
	if !keyExists {
		if dbErr == nil {
			return nil, fmt.Errorf("active CP certificate exists in the database but key files are missing; restore the files or perform an explicit rotation")
		}
		cert, err := generateControlPlaneCERT(cpKeyPath, cpCertPath, secret, signer, log)
		if err != nil {
			return nil, err
		}
		if err := registerControlPlaneCert(ctx, pkiCA, cert); err != nil {
			return nil, err
		}
	} else if errors.Is(dbErr, sql.ErrNoRows) {
		key, cert, err := pki_utils.LoadKeyAndCert(cpKeyPath, cpCertPath, secret, "cp")
		if err != nil {
			return nil, err
		}
		if err := pki_utils.VerifyKeyAndCert(cert, key); err != nil {
			return nil, err
		}
		if err := verifyCertificateChain(cert, signer); err != nil {
			return nil, err
		}
		if err := registerControlPlaneCert(ctx, pkiCA, cert); err != nil {
			return nil, err
		}
	} else if dbCert.SerialNumber != "" {
		_, cert, err := pki_utils.LoadKeyAndCert(cpKeyPath, cpCertPath, secret, "cp")
		if err != nil {
			return nil, err
		}
		if dbCert.SerialNumber != cert.SerialNumber.String() {
			return nil, fmt.Errorf("active CP certificate in database does not match the certificate on disk")
		}
	}

	return LoadControlPlanePKI(baseDir, secret, signer, pkiCA, log)
}

func LoadControlPlanePKI(baseDir, secret string, signer pki.CASigner, pkiCA pkica.Repository, log *slog.Logger) (ControlPlaneCrypto, error) {
	log.Info("Initializing Control Plane PKI...")
	CPPKIDir := filepath.Join(baseDir, "cp")

	if info, err := os.Stat(CPPKIDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Control Plane PKI is not provisioned; run 'ashrix-cp setup cp-cert' first: %w", err)
		}
		return nil, fmt.Errorf("checking Control Plane PKI directory: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("Control Plane PKI path %s is not a directory", CPPKIDir)
	}

	cpKeyPath := filepath.Join(CPPKIDir, "cp.key.enc")
	cpCertPath := filepath.Join(CPPKIDir, "cp.crt")

	keyExists, err := filehelper.FileExists(cpKeyPath)
	if err != nil {
		return nil, err
	}
	certExists, err := filehelper.FileExists(cpCertPath)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	dbCert, dbErr := pkiCA.GetActiveComponentCertByType(ctx, "cp")
	if dbErr != nil && !errors.Is(dbErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get active CP component certificate: %w", dbErr)
	}

	if keyExists != certExists {
		return nil, fmt.Errorf("incomplete Control Plane certificate files at %s; refusing to replace them", CPPKIDir)
	}
	if !keyExists {
		if dbErr == nil {
			return nil, fmt.Errorf("active CP certificate exists in the database but key files are missing; restore the files or perform an explicit rotation")
		}
		return nil, fmt.Errorf("Control Plane certificate is not provisioned; run 'ashrix-cp setup cp-cert'")
	}
	if errors.Is(dbErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("Control Plane certificate is missing from the database; run 'ashrix-cp setup cp-cert' to reconcile it")
	}

	cpKey, cpCert, err := pki_utils.LoadKeyAndCert(cpKeyPath, cpCertPath, secret, "cp")
	if err != nil {
		return nil, err
	}
	if err := pki_utils.VerifyKeyAndCert(cpCert, cpKey); err != nil {
		return nil, err
	}
	if dbErr == nil && dbCert.SerialNumber != cpCert.SerialNumber.String() {
		return nil, fmt.Errorf("active CP certificate in database does not match the certificate on disk")
	}
	if err := verifyCertificateChain(cpCert, signer); err != nil {
		return nil, err
	}

	// Build the CA pool for mTLS verification of nodes
	pool := x509.NewCertPool()
	pool.AddCert(signer.RootCert())
	pool.AddCert(signer.IntermediateCert())

	log.Info("Control Plane PKI Intializing Done...")

	return &ControlPlanePKI{
		ControlPlaneKeyPath:  cpKeyPath,
		ControlPlaneCertPath: cpCertPath,
		ControlPlaneKey:      cpKey,
		ControlPlaneCert:     cpCert,
		CAPool:               pool,
	}, nil
}

func registerControlPlaneCert(ctx context.Context, pkiCA pkica.Repository, cert *x509.Certificate) error {
	intermediate, err := pkiCA.GetActiveCACert(ctx, store.GetActiveCACertParams{Name: "Ashrix Intermediate CA", Type: "intermediate"})
	if err != nil {
		return fmt.Errorf("failed to get active intermediate CA record: %w", err)
	}
	_, err = pkiCA.CreateComponentCert(ctx, store.RegisterCompCertParams{
		OrgID: pgtype.UUID{}, ComponentType: "cp", ComponentID: pgtype.UUID{}, CaID: intermediate.ID,
		CertPem: string(pki_utils.MarshalCert(cert)), SerialNumber: cert.SerialNumber.String(),
		Subject: cert.Subject.CommonName, San: cert.DNSNames, IssuedAt: cert.NotBefore,
		ExpiresAt: cert.NotAfter, RotationOf: pgtype.UUID{},
	})
	if err != nil {
		return fmt.Errorf("register Control Plane certificate: %w", err)
	}
	return nil
}

func verifyCertificateChain(cert *x509.Certificate, signer pki.CASigner) error {
	roots := x509.NewCertPool()
	roots.AddCert(signer.RootCert())
	intermediates := x509.NewCertPool()
	intermediates.AddCert(signer.IntermediateCert())
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates}); err != nil {
		return fmt.Errorf("Control Plane certificate chain verification failed: %w", err)
	}
	return nil
}

func (b *ControlPlanePKI) CPCert() *x509.Certificate {
	return b.ControlPlaneCert
}

// TLSCertificate returns the tls.Certificate for use in TLS configs.
func (b *ControlPlanePKI) TLSCertificate() tls.Certificate {
	return tls.Certificate{
		Certificate: [][]byte{b.ControlPlaneCert.Raw},
		PrivateKey:  b.ControlPlaneKey,
		Leaf:        b.ControlPlaneCert,
	}
}

// CACertPool returns the pool containing the Root and Intermediate CAs.
func (b *ControlPlanePKI) CACertPool() *x509.CertPool {
	return b.CAPool
}

func generateControlPlaneCERT(keyPath, certPath, secret string, signer pki.CASigner, log *slog.Logger) (*x509.Certificate, error) {

	key, _, err := pki_utils.GenerateCertificateSigningKey(keyPath, certPath, secret, "cp")

	if err != nil {
		return nil, err
	}
	fmt.Printf("Control Plane CSR Request Done ...")

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         "cp-main",
			Organization:       []string{"Ashrix"},
			OrganizationalUnit: []string{"ControlPlane"},
		},
		URIs: []*url.URL{
			{
				Scheme: "spiffe",
				Host:   "ashrix",
				Path:   "/cp/cp-main",
			},
		},
		DNSNames: []string{"localhost"},
	}

	certDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return nil, fmt.Errorf("Signing Control Plane Cert: %w", err)
	}

	certReq, err := x509.ParseCertificateRequest(certDER)
	if err != nil {
		return nil, err
	}
	log.Info("Control Plane CSR Request Done ...")
	log.Info("CA Sigining Control Plane CSR...")

	Cert, err := signer.IssueCert(certReq, 356*24*time.Hour, certReq.Subject.CommonName, "*.ashrix.io")
	if err != nil {
		return nil, fmt.Errorf("issue Control Plane certificate: %w", err)
	}

	if err := pki_utils.WriteCert(certPath, Cert); err != nil {
		return nil, err
	}
	fmt.Printf("  ✓ Control Plane Cert generated. Valid until: %s\n", Cert.NotAfter.Format("2006-01-02"))
	return Cert, nil
}

type NullUUID struct {
	UUID  uuid.UUID
	Valid bool
}
