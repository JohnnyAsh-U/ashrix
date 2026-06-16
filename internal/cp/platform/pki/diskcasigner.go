package pki

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
}

func NewDiskCASigner(baseDir, secret string) (*DiskCASigner, error) {
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

	rootKeyPath := filepath.Join(baseDir, "root-ca.key.enc")
	rootCertPath := filepath.Join(baseDir, "root-ca.crt")
	interKeyPath := filepath.Join(baseDir, "intermediate-ca.key.enc")
	interCertPath := filepath.Join(baseDir, "intermediate-ca.crt")
	var intermediateKey *ecdsa.PrivateKey
	var intermediateCert *x509.Certificate
	var rootCert *x509.Certificate

	//Check if intermediateCAKey and Cert Exists on the filepath
	keyexists, err := filehelper.FileExists(interKeyPath)
	if err != nil {
		return nil, err
	}
	certexists, err := filehelper.FileExists(interCertPath)
	if err != nil {
		return nil, err
	}

	if !keyexists || !certexists {
		fmt.Println("→ Intermediate CA not found. Checking Root CA...")
		// Generate Root CA if not exists
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

			rootKey, rootCert, err := generateRootCA(rootKeyPath, rootCertPath, secret, "root")
			if err != nil {
				return nil, err
			}

			fmt.Println("Generating a New Intermediate CA...")

			intermediateKey, intermediateCert, err = generateIntermediateCA(interKeyPath, interCertPath, secret, rootKey, rootCert)
			if err != nil {
				return nil, err
			}

		} else if rootcaexists && rootcertexists {
			fmt.Println("Root Key and Cert Found...")
			fmt.Println("Generating a New Intermediate CA...")
			rootKey, rootCert, err := pki.LoadKeyAndCert(rootKeyPath, rootCertPath, secret, "root")
			if err != nil {
				return nil, err
			}
			intermediateKey, intermediateCert, err = generateIntermediateCA(interKeyPath, interCertPath, secret, rootKey, rootCert)
			if err != nil {
				return nil, err
			}
		} else if !rootcaexists {
			panic("Root CA KEY NOT FOUND")
		} else if !rootcertexists {
			panic("Root CA CERT NOT FOUND")
		} else {
			panic("UNKNOWN ERROR")
		}
		fmt.Println("Intermediate CA Generated...")
	} else {
		fmt.Println("Loading RootCA")
		rootCert, err = pki.LoadCert(rootCertPath)
		if err != nil {
			return nil, err
		}
		fmt.Println("Loading Intermediate")

		intermediateKey, intermediateCert, err = pki.LoadKeyAndCert(interKeyPath, interCertPath, secret, "intermediate")
		if err != nil {
			return nil, err
		}
	}

	//Verify Key and Cert
	if err := pki.VerifyKeyAndCert(intermediateCert, intermediateKey); err != nil {
		return nil, err
	}

	//Load the root Cert
	rootCert, err = pki.LoadCert(rootCertPath)
	if err != nil {
		return nil, err
	}

	fmt.Println("PKI Intializing Done...")
	return &DiskCASigner{
		RootCAKeyEncPath:       rootKeyPath,
		RootCACertPath:         rootCertPath,
		IntermediateKeyEncPath: interKeyPath,
		IntermediateCACertPath: interCertPath,
		intermediateKey:        intermediateKey,
		intermediateCert:       intermediateCert,
		RootCACert:             rootCert,
	}, nil
}

func (d *DiskCASigner) IssueCert(csr *x509.CertificateRequest, validity time.Duration) (*x509.Certificate, error) {
	template, err := pki.BuildCertTemplate(csr, validity)
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

// func (s *DiskCASigner) Close() error

func generateRootCA(rootCAKeyPath, rootCACertPath, secret, context string) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	key, _, err := pki.GenerateSigningKey(rootCAKeyPath, rootCACertPath, secret, context)
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

	fmt.Printf("  ✓ Root CA generated. Valid until: %s\n", cert.NotAfter.Format("2006-01-02"))
	return key, cert, nil
}

func generateIntermediateCA(intermediateKeyPath, intermediateCertPath, secret string, rootKey *ecdsa.PrivateKey, rootCert *x509.Certificate) (*ecdsa.PrivateKey, *x509.Certificate, error) {

	key, pub, err := pki.GenerateSigningKey(intermediateKeyPath, intermediateCertPath, secret, "intermediate")

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
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{".ashrix.internal", "ashrix.internal"},
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

	fmt.Printf("  ✓ Intermediate CA generated. Valid until: %s\n", cert.NotAfter.Format("2006-01-02"))
	fmt.Printf("  ✓ Chain verified: Intermediate CA → Root CA\n")
	return key, cert, nil
}

func verifyChain(cert, rootCert *x509.Certificate) error {
	pool := x509.NewCertPool()
	pool.AddCert(rootCert)
	_, err := cert.Verify(x509.VerifyOptions{Roots: pool})
	return err
}
