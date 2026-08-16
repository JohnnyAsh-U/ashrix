package pki

import (
	// "github.com/JohnnyAsh-U/ashrix-api/internal/pki"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

func NewKMSCASigner(VaultToken, VaultUrl, baseDir, rootUnlockSecret string, pkica pkica.Repository) (*KMSCASigner, error) {
	println("Initializing PKI...")
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}

	if baseDir == "" {
		baseDir = filepath.Join(home, ".ashrix", "pki")
		println("→ Using default PKI base directory:", baseDir)
	} else {
		baseDir = filepath.Join(home, baseDir)
	}
	println("Connecting to Vault...")
	config := api.DefaultConfig()
	config.Address = VaultUrl
	client, err := api.NewClient(config)

	if err != nil {
		return nil, err
	}
	client.SetToken(VaultToken)

	_, err = client.Logical().Read("auth/token/lookup-self")
	if err != nil {
		return nil, err
	}

	fmt.Println("Connected to Vault")

	//Check if intermediate exist
	exists, err := checkIntermediate(client, "pki")
	if err != nil {
		return nil, err
	}

	rootKeyPath := filepath.Join(baseDir, "root-ca.key.enc")
	rootCertPath := filepath.Join(baseDir, "root-ca.crt")

	if !exists {
		println("Generating Intermediate CA...")
		println("Checking for Root CA...")

		rootcaexists, err := filehelper.FileExists(rootKeyPath)
		if err != nil {
			return nil, err
		}
		rootcertexists, err := filehelper.FileExists(rootCertPath)
		if err != nil {
			return nil, err
		}

		if !rootcertexists && !rootcaexists {
			fmt.Println("Root Key and Cert Not Found...")
			fmt.Println("Generating RootCA")

			rootKey, rootCert, err := generateRootCA(rootKeyPath, rootCertPath, rootUnlockSecret, "root")
			if err != nil {
				return nil, err
			}

			fmt.Println("Generating a New Intermediate CA and CSR...")

			interCSR, err := generateIntermediateCAInVault(client, "pki")
			if err != nil {
				return nil, err
			}
			isSigned, err := SignWithRootCA(client, "pki", interCSR, rootCert, rootKey)
			if err != nil {
				return nil, err
			}
			if !isSigned {
				return nil, fmt.Errorf("Unable to Signed Intermediate")
			}
			fmt.Println("Generated a New Intermediate CA")
		} else if rootcaexists && rootcertexists {
			fmt.Println("Root Key and Cert Found...")
			fmt.Println("Generating a New Intermediate CA...")
			rootKey, rootCert, err := pki.LoadKeyAndCert(rootKeyPath, rootCertPath, rootUnlockSecret, "root")
			if err != nil {
				return nil, err
			}
			interCSR, err := generateIntermediateCAInVault(client, "pki")
			if err != nil {
				return nil, err
			}
			isSigned, err := SignWithRootCA(client, "pki", interCSR, rootCert, rootKey)
			if err != nil {
				return nil, err
			}
			if !isSigned {
				return nil, fmt.Errorf("Unable to Signed Intermediate")
			}
			fmt.Println("Generated a New Intermediate CA")
		} else if !rootcaexists {
			panic("Root CA KEY NOT FOUND")
		} else if !rootcertexists {
			panic("Root CA CERT NOT FOUND")
		} else {
			panic("UNKNOWN ERROR")
		}
	}

	var intermediateCert *x509.Certificate
	secret, err := client.Logical().Read("pki/cert/ca")
	if err == nil && secret != nil && secret.Data["certificate"] != nil {
		certPEM := secret.Data["certificate"].(string)
		certBlock, _ := pem.Decode([]byte(certPEM))
		if certBlock != nil {
			intermediateCert, _ = x509.ParseCertificate(certBlock.Bytes)
		}
	}

	rootCert, err := pki.LoadCert(rootCertPath)
	if err != nil {
		return nil, err
	}

	fmt.Println("PKI Intializing Done...")
	return &KMSCASigner{
		VaultClient:      client,
		RootCAKeyEncPath: rootKeyPath,
		RootCACertPath:   rootCertPath,
		MountPath:        "pki",
		rootCACert:       rootCert,
		intermediateCert: intermediateCert,
	}, nil
}

func (s *KMSCASigner) IssueCert(csr *x509.CertificateRequest, validity time.Duration, CN,DNSName string) (*x509.Certificate, error) {
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
	return nil,nil
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
