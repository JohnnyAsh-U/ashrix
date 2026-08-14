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
	CARepo pkica.Repository
}

func ControlPlanePKIIntializer(baseDir string, secret string, signer pki.CASigner, pki_ca pkica.Repository) (ControlPlaneCrypto, error) {
	fmt.Println("Initializing Control Plane PKI...")
	CPPKIDir := filepath.Join(baseDir, "pki", "cp")

	if err := os.MkdirAll(CPPKIDir, 0700); err != nil {
		return nil, fmt.Errorf("creating PKI directory: %w", err)
	}

	cpKeyPath := filepath.Join(CPPKIDir, "cp.key.enc")
	cpCertPath := filepath.Join(CPPKIDir, "cp.crt")

	// Check if intermediateCAKey and Cert Exists on the filepath
	keyexists, err := filehelper.FileExists(cpKeyPath)
	if err != nil {
		return nil, err
	}
	certexists, err := filehelper.FileExists(cpCertPath)
	if err != nil {
		return nil, err
	}

	if !keyexists || !certexists {
		if keyexists {
			os.Remove(cpKeyPath)
		}
		if certexists {
			os.Remove(cpCertPath)
		}
		fmt.Println("→ Control Plane Key and Cert not found")
		fmt.Println("Requesting one...")
		cert, err := generateControlPlaneCERT(cpKeyPath, cpCertPath, secret, signer)
		if err != nil {
			return nil, err
		}

		// Get the active CA cert
		context := context.Background()
		dbRootCertRecord, err := pki_ca.GetActiveCACert(context, store.GetActiveCACertParams{Name: "Ashrix Intermediate CA", Type: "intermediate"})
		if err != nil {
			return nil, fmt.Errorf("failed to get active CA cert: %w", err)
		}

		// Get existing cp component cert id
		dbCPComponentCertRecords, err := pki_ca.GetActiveComponentCertByType(context, "cp")

		if errors.Is(err, sql.ErrNoRows) {

		}

		// ✅ FIX: Check if record exists properly
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to get active component cert: %w", err)
		}

		if err == nil {
			// Record found - revoke it
			_, err = pki_ca.RevokeComponentCert(context, store.RevokeCompCertParams{
				ComponentID:           dbCPComponentCertRecords.ComponentID,
				RevokeReason: pgtype.Text{String: "superseded", Valid: true},
			})
			if err != nil {
				return nil, fmt.Errorf("failed to revoke existing component cert: %w", err)
			}
		}

		// Register component cert
		_, err = pki_ca.CreateComponentCert(context, store.RegisterCompCertParams{
			OrgID:         pgtype.UUID{Valid: false},
			ComponentType: "cp",
			ComponentID:   pgtype.UUID{Valid: false},
			CaID:          dbRootCertRecord.ID,
			CertPem:       string(pki_utils.MarshalCert(cert)),
			SerialNumber:  cert.SerialNumber.String(),
			Subject:       cert.Subject.CommonName,
			San:           cert.DNSNames,
			IssuedAt:      cert.NotBefore,
			ExpiresAt:     cert.NotAfter,
			RotationOf:    pgtype.UUID{Valid: false},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create component certificate: %w", err)
		}
		fmt.Println("CPLANE certificate registered successfully")
	}

	cpKey, cpCert, err := pki_utils.LoadKeyAndCert(cpKeyPath, cpCertPath, secret, "cp")
	if err != nil {
		return nil, err
	}

	// Verify Key and Cert
	if err := pki_utils.VerifyKeyAndCert(cpCert, cpKey); err != nil {
		return nil, err
	}

	// Build the CA pool for mTLS verification of nodes
	pool := x509.NewCertPool()
	pool.AddCert(signer.RootCert())
	pool.AddCert(signer.IntermediateCert())

	fmt.Println("Control Plane PKI Intializing Done...")

	return &ControlPlanePKI{
		ControlPlaneKeyPath:  cpKeyPath,
		ControlPlaneCertPath: cpCertPath,
		ControlPlaneKey:      cpKey,
		ControlPlaneCert:     cpCert,
		CAPool:               pool,
	}, nil
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

func generateControlPlaneCERT(keyPath, certPath, secret string, signer pki.CASigner) (*x509.Certificate, error) {

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
	fmt.Println("Control Plane CSR Request Done ...")
	fmt.Println("CA Sigining Control Plane CSR...")

	Cert, err := signer.IssueCert(certReq, 356*24*time.Hour, certReq.Subject.CommonName)

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
