package crypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
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
	SignBundle(version string, payload []byte) (*SignedBundle, error)
	VerifyBundle(sb *SignedBundle) error
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
	db                   *store.Queries // Database queries for CA certificates

}

type Bundle struct {
	Version  string    `json:"version"`
	Payload  []byte    `json:"payload"`
	IssuedAt time.Time `json:"issued_at"`
	Nonce    string    `json:"nonce"`
}

type SignedBundle struct {
	Bundle    Bundle `json:"bundle"`
	Signature string `json:"signature"`
}

func ControlPlanePKIIntializer(baseDir string, secret string, signer pki.CASigner, dbQueries *store.Queries) (ControlPlaneCrypto, error) {
	fmt.Println("Initializing Control Plane PKI...")
	CPPKIDir := filepath.Join(baseDir, "pki", "cp")

	if err := os.MkdirAll(CPPKIDir, 0700); err != nil {
		return nil, fmt.Errorf("creating PKI directory: %w", err)
	}

	cpKeyPath := filepath.Join(CPPKIDir, "cp.key.enc")
	cpCertPath := filepath.Join(CPPKIDir, "cp.crt")

	//Check if intermediateCAKey and Cert Exists on the filepath
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
		//Get the activate
		context := context.Background()
		dbRootCertRecord, err := dbQueries.GetActiveCACert(context, store.GetActiveCACertParams{Name: "Ashrix Intermediate CA", Type: "intermediate"})

		//Get existing cp component cert id
		dbCPComponentCertRecords, err := dbQueries.GetActiveComponentCertByType(context, "cp")
		//Revoke any Existing cp component cert
		_, err = dbQueries.RevokeComponentCertificate(context, store.RevokeComponentCertificateParams{
			ID:           dbCPComponentCertRecords.ID,
			RevokeReason: pgtype.Text{String: "superseded", Valid: true},
		})

		if err != nil {
			return nil, fmt.Errorf("%v", err)
		}

		// Register component cert
		_, err = dbQueries.CreateComponentCertificate(context, store.CreateComponentCertificateParams{
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
		fmt.Println(err)
		if err != nil {
			return nil, fmt.Errorf("%v", err)
		}
	}
	cpKey, cpCert, err := pki_utils.LoadKeyAndCert(cpKeyPath, cpCertPath, secret, "cp")
	if err != nil {
		return nil, err
	}

	//Verify Key and Cert
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

// Sign signs a bundle. Returns ASN.1 DER-encoded ECDSA signature.
// version and payload must match exactly what the node will verify.
func (b *ControlPlanePKI) SignBundle(version string, payload []byte) (*SignedBundle, error) {
	if len(payload) == 0 {
		return nil, errors.New("SignedBundle: payload is empty")
	}

	//Generate nonce to prevent replay even if timestamp matches
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("Signed: failed to generate nonce: %w", err)
	}

	bundle := Bundle{
		Version:  version,
		Payload:  payload,
		IssuedAt: time.Now().UTC(),
		Nonce:    fmt.Sprintf("%x", nonce),
	}

	digest, err := bundleDigest(bundle)
	if err != nil {
		return nil, fmt.Errorf("signing bundle failed to compute digest: %w", err)
	}
	sig, err := ecdsa.SignASN1(rand.Reader, b.ControlPlaneKey, digest)
	if err != nil {
		return nil, fmt.Errorf("signing bundle: %w", err)
	}
	return &SignedBundle{
		Bundle:    bundle,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}, nil
}

// VerifyBundle verifies a CP-signed bundle on the node side.
// cpPublicKeyPEM is the embedded CP public key from the binary (go:embed).
func (b *ControlPlanePKI) VerifyBundle(sb *SignedBundle) error {
	if sb == nil {
		return errors.New("VerifyBundle: signed bundle is nil")
	}
	if len(sb.Bundle.Payload) == 0 {
		return errors.New("Bundle Payload is empty")
	}

	if sb.Bundle.Nonce == "" {
		return errors.New("Bundle Nonce is empty")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sb.Signature)
	if err != nil {
		return fmt.Errorf("parsing CP public key: %w", err)
	}
	digest, err := bundleDigest(sb.Bundle)
	if !ecdsa.VerifyASN1(b.ControlPlaneCert.PublicKey.(*ecdsa.PublicKey), digest, sigBytes) {
		return errors.New("bundle signature invalid")
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

// bundleDigest computes SHA256(version_big_endian || payload).
// Identical on CP (Sign) and node (VerifyBundle).
func bundleDigest(b Bundle) ([]byte, error) {
	canonical, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("bundleDigest: json.Marshal: %x", err)
	}
	digest := sha256.Sum256(canonical)
	return digest[:], nil
}

func generateControlPlaneCERT(keyPath, certPath, secret string, signer pki.CASigner) (*x509.Certificate, error) {

	key, _, err := pki_utils.GenerateSigningKey(keyPath, certPath, secret, "cp")

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
