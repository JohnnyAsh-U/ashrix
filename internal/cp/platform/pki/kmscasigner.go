package pki

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	"github.com/hashicorp/vault/api"
	// "time"
)

// Layout holds all resolved paths for the PKI directory.
type KMSCASigner struct {
	RootCAKeyEncPath string
	RootCACertPath   string

	MountPath   string
	VaultClient *api.Client

	intermediateCert *x509.Certificate
	rootCACert       *x509.Certificate
}

func NewKMSCASigner(VaultToken, VaultUrl, baseDir, rootUnlockSecret string, pkica pkica.Repository, log *slog.Logger) (*KMSCASigner, error) {
	resolvedBaseDir, err := resolveKMSBaseDir(baseDir)
	if err != nil {
		return nil, err
	}
	client, err := connectVault(VaultToken, VaultUrl)
	if err != nil {
		return nil, err
	}
	exists, err := checkIntermediate(client, "pki")
	if err != nil {
		return nil, fmt.Errorf("check Vault intermediate CA: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("Vault intermediate CA is not provisioned; run 'ashrix-cp setup pki'")
	}

	rootCertPath := filepath.Join(resolvedBaseDir, "root-ca.crt")
	rootCert, err := pki.LoadCert(rootCertPath)
	if err != nil {
		return nil, fmt.Errorf("load root CA certificate; run 'ashrix-cp setup pki' or restore %s: %w", rootCertPath, err)
	}
	intermediateCert, err := loadVaultIntermediateCert(client, "pki")
	if err != nil {
		return nil, err
	}
	if err := verifyChain(intermediateCert, rootCert); err != nil {
		return nil, fmt.Errorf("Vault intermediate CA is not signed by the configured root: %w", err)
	}
	ctx := context.Background()
	if err := verifyCACertRecord(ctx, pkica, "Ashrix Root CA", "root", rootCert); err != nil {
		return nil, err
	}
	if err := verifyCACertRecord(ctx, pkica, "Ashrix Intermediate CA", "intermediate", intermediateCert); err != nil {
		return nil, err
	}

	return &KMSCASigner{
		VaultClient:      client,
		RootCAKeyEncPath: filepath.Join(resolvedBaseDir, "root-ca.key.enc"),
		RootCACertPath:   rootCertPath,
		MountPath:        "pki",
		rootCACert:       rootCert,
		intermediateCert: intermediateCert,
	}, nil
}

func ProvisionKMSCASigner(VaultToken, VaultUrl, baseDir, rootUnlockSecret string, pkica pkica.Repository, log *slog.Logger) (*KMSCASigner, error) {
	resolvedBaseDir, err := resolveKMSBaseDir(baseDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(resolvedBaseDir, 0700); err != nil {
		return nil, fmt.Errorf("creating PKI directory: %w", err)
	}
	client, err := connectVault(VaultToken, VaultUrl)
	if err != nil {
		return nil, err
	}
	exists, err := checkIntermediate(client, "pki")
	if err != nil {
		return nil, fmt.Errorf("check Vault intermediate CA: %w", err)
	}
	rootKeyPath := filepath.Join(resolvedBaseDir, "root-ca.key.enc")
	rootCertPath := filepath.Join(resolvedBaseDir, "root-ca.crt")

	if !exists {
		rootKeyExists, err := filehelper.FileExists(rootKeyPath)
		if err != nil {
			return nil, fmt.Errorf("check root CA key: %w", err)
		}
		rootCertExists, err := filehelper.FileExists(rootCertPath)
		if err != nil {
			return nil, fmt.Errorf("check root CA certificate: %w", err)
		}
		var rootKey *ecdsa.PrivateKey
		var rootCert *x509.Certificate
		switch {
		case !rootKeyExists && !rootCertExists:
			rootKey, rootCert, err = generateRootCA(rootKeyPath, rootCertPath, rootUnlockSecret, "root", log)
		case rootKeyExists && rootCertExists:
			rootKey, rootCert, err = pki.LoadKeyAndCert(rootKeyPath, rootCertPath, rootUnlockSecret, "root")
			if err == nil {
				err = pki.VerifyKeyAndCert(rootCert, rootKey)
			}
		default:
			return nil, fmt.Errorf("incomplete root CA files at %s; refusing to recreate them", resolvedBaseDir)
		}
		if err != nil {
			return nil, fmt.Errorf("load or create root CA: %w", err)
		}

		csr, err := generateIntermediateCAInVault(client, "pki")
		if err != nil {
			return nil, fmt.Errorf("generate Vault intermediate CSR: %w", err)
		}
		signed, err := SignWithRootCA(client, "pki", csr, rootCert, rootKey)
		if err != nil {
			return nil, fmt.Errorf("sign Vault intermediate CA: %w", err)
		}
		if !signed {
			return nil, fmt.Errorf("failed to sign Vault intermediate CA")
		}
	}

	rootCert, err := pki.LoadCert(rootCertPath)
	if err != nil {
		return nil, fmt.Errorf("load provisioned root CA: %w", err)
	}
	intermediateCert, err := loadVaultIntermediateCert(client, "pki")
	if err != nil {
		return nil, err
	}
	if err := verifyChain(intermediateCert, rootCert); err != nil {
		return nil, fmt.Errorf("Vault intermediate CA is not signed by the configured root: %w", err)
	}
	ctx := context.Background()
	if err := ensureCACertRecord(ctx, pkica, "Ashrix Root CA", "root", rootCert); err != nil {
		return nil, err
	}
	if err := ensureCACertRecord(ctx, pkica, "Ashrix Intermediate CA", "intermediate", intermediateCert); err != nil {
		return nil, err
	}
	return NewKMSCASigner(VaultToken, VaultUrl, resolvedBaseDir, rootUnlockSecret, pkica, log)
}

func resolveKMSBaseDir(baseDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	if baseDir == "" {
		baseDir = filepath.Join(home, ".ashrix", "pki")
	} else if !filepath.IsAbs(baseDir) {
		baseDir = filepath.Join(home, baseDir)
	}
	return baseDir, nil
}

func connectVault(token, vaultURL string) (*api.Client, error) {
	config := api.DefaultConfig()
	config.Address = vaultURL
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create Vault client: %w", err)
	}
	client.SetToken(token)
	_, err = client.Logical().Read("auth/token/lookup-self")
	if err != nil {
		return nil, fmt.Errorf("authenticate to Vault: %w", err)
	}
	return client, nil
}

func (s *KMSCASigner) IssueCert(csr *x509.CertificateRequest, validity time.Duration, CN, DNSName string) (*x509.Certificate, error) {
	//Issue with the vault intermediate ca
	csrPEM := pki.MarshalCsr(csr)
	resp, err := s.VaultClient.Logical().Write(s.MountPath+"/sign/ashrix",
		map[string]interface{}{
			"csr": csrPEM,
			"ttl": "720h",
		},
	)
	if err != nil {
		return nil, err
	}

	certPEM := resp.Data["certificate"].(string)
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil {
		return nil, errors.New("Vault returned certificate is not a valid PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing issued certificate: %w", err)
	}
	return cert, nil
}

func (d *KMSCASigner) RootCert() *x509.Certificate {
	return d.rootCACert
}

func (d *KMSCASigner) IntermediateCert() *x509.Certificate {
	return d.intermediateCert
}

func (d *KMSCASigner) TrustBundle() string {
	if d.intermediateCert == nil || d.rootCACert == nil {
		return ""
	}
	interPEM := pki.MarshalCert(d.intermediateCert)
	rootPEM := pki.MarshalCert(d.rootCACert)
	return string(interPEM) + string(rootPEM)
}

// GetCertPool returns a cached x509.CertPool containing all active CA certificates.
// It refreshes the pool from the database if it's older than 5 minutes.
func (d *KMSCASigner) GetCertPool() (*x509.CertPool, error) {
	if d.rootCACert == nil || d.intermediateCert == nil {
		return nil, fmt.Errorf("KMS CA certificates are not loaded")
	}
	pool := x509.NewCertPool()
	pool.AddCert(d.rootCACert)
	pool.AddCert(d.intermediateCert)
	return pool, nil
}

func ensureCACertRecord(ctx context.Context, repo pkica.Repository, name, certType string, cert *x509.Certificate) error {
	if cert == nil {
		return fmt.Errorf("%s certificate is missing", name)
	}
	active, err := repo.GetActiveCACert(ctx, store.GetActiveCACertParams{Name: name, Type: certType})
	if err == nil {
		if active.SerialNumber != cert.SerialNumber.String() {
			return fmt.Errorf("active %s in database does not match configured CA", name)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load active %s record: %w", name, err)
	}
	_, err = repo.CreateCACert(ctx, store.InsertCACertParams{
		Name: name, Type: certType, CertPem: string(pki.MarshalCert(cert)),
		SerialNumber: cert.SerialNumber.String(), Subject: cert.Subject.CommonName,
		IssuedAt: cert.NotBefore, ExpiresAt: cert.NotAfter,
	})
	if err != nil {
		return fmt.Errorf("register %s in database: %w", name, err)
	}
	return nil
}

func verifyCACertRecord(ctx context.Context, repo pkica.Repository, name, certType string, cert *x509.Certificate) error {
	if cert == nil {
		return fmt.Errorf("%s certificate is missing", name)
	}
	active, err := repo.GetActiveCACert(ctx, store.GetActiveCACertParams{Name: name, Type: certType})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("active %s is missing from the database; run 'ashrix-cp setup pki'", name)
	}
	if err != nil {
		return fmt.Errorf("load active %s record: %w", name, err)
	}
	if active.SerialNumber != cert.SerialNumber.String() {
		return fmt.Errorf("active %s in database does not match configured CA", name)
	}
	return nil
}

func loadVaultIntermediateCert(client *api.Client, mountPath string) (*x509.Certificate, error) {
	secret, err := client.Logical().Read(mountPath + "/cert/ca")
	if err != nil {
		return nil, fmt.Errorf("read Vault intermediate CA certificate: %w", err)
	}
	if secret == nil || secret.Data["certificate"] == nil {
		return nil, fmt.Errorf("Vault intermediate CA certificate is missing; run 'ashrix-cp setup pki'")
	}
	certPEM, ok := secret.Data["certificate"].(string)
	if !ok {
		return nil, fmt.Errorf("Vault intermediate CA certificate has an unexpected format")
	}
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("Vault intermediate CA certificate is not valid PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse Vault intermediate CA certificate: %w", err)
	}
	return cert, nil
}

func checkIntermediate(client *api.Client, mountPath string) (bool, error) {
	// Check if CA Cert exists
	secret, err := client.Logical().Read(mountPath + "/cert/ca")

	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	if secret == nil {
		fmt.Println("No CA Configured")
		return false, nil
	} else {
		fmt.Println("CA Already configured")
		return true, nil
	}
}

func generateIntermediateCAInVault(client *api.Client, mountPath string) (string, error) {
	resp, err := client.Logical().Write(
		mountPath+"/intermediate/generate/internal",
		map[string]interface{}{
			"common_name":  "Ashrix Intermediate CA",
			"organization": "Ashrix",
			"key_type":     "ec",
			"key_bits":     256,
		},
	)
	if err != nil {
		return "", err
	}
	fmt.Println("Generating CSR with Intermediate Key")
	csr := resp.Data["csr"].(string)

	return csr, nil
}

func SignWithRootCA(client *api.Client, mountPath string, csr string, rootCert *x509.Certificate, rootKey *ecdsa.PrivateKey) (bool, error) {
	//Parse csr from vault
	csrBlock, _ := pem.Decode([]byte(csr))
	if csrBlock == nil {
		return false, errors.New("Vault CSR is not a valid PEM")
	}
	ParseCSR, err := x509.ParseCertificateRequest(csrBlock.Bytes)

	if err != nil {
		return false, err
	}

	if err := ParseCSR.CheckSignature(); err != nil {
		return false, fmt.Errorf("CSR signature invalid")
	}

	serial, err := pki.RandomSerial()

	if err != nil {
		return false, err
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
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{".ashrix.internal", "ashrix.internal"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, rootCert, ParseCSR.PublicKey, rootKey)
	if err != nil {
		return false, fmt.Errorf("signing intermediate CA: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return false, err
	}

	// Verify chain before writing
	if err := verifyChain(cert, rootCert); err != nil {
		return false, fmt.Errorf("chain verification failed: %w", err)
	}

	intermediatePEM := pki.MarshalCert(cert)

	_, err = client.Logical().Write(mountPath+"/intermediate/set-signed",
		map[string]interface{}{
			"certificate": string(intermediatePEM),
		},
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "400") || strings.Contains(msg, "no handler") || strings.Contains(msg, "unsupported paths")
}
