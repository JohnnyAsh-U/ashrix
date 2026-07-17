package pki

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"time"
)

// validateCSR checks the CSR is well-formed and matches the expected node identity.
func ValidateCSR(csr *x509.CertificateRequest, nodeID string) error {
	if err := csr.CheckSignature(); err != nil {
		return fmt.Errorf("CSR self-signature invalid: %w", err)
	}
	expectedCN := nodeID + ".ashrix.internal"
	if csr.Subject.CommonName != expectedCN {
		return fmt.Errorf("CN %q does not match expected %q", csr.Subject.CommonName, expectedCN)
	}
	for _, dns := range csr.DNSNames {
		if dns == expectedCN {
			return nil
		}
	}
	return fmt.Errorf("CSR missing SAN DNS:%s", expectedCN)
}

// buildCertTemplate builds the x509 template for a node leaf cert.
// Same template regardless of which backend signs it.
func BuildCertTemplate(csr *x509.CertificateRequest, validity time.Duration, CN string) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating serial: %w", err)
	}
	now := time.Now().UTC()
	return &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   CN,
			Organization: []string{"Ashrix"},
			OrganizationalUnit: csr.Subject.OrganizationalUnit,
		},
		DNSNames:  []string{"localhost"},
		IPAddresses: []net.IP{
			net.ParseIP("10.18.74.9"),
		},
		NotBefore: now.Add(-30 * time.Second), // small backdating for clock skew
		NotAfter:  now.Add(validity),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, // inbound — node as server
			x509.ExtKeyUsageClientAuth, // outbound — node as client
		},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}, nil
}
