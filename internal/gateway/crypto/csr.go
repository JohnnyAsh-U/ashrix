package crypto

import (
	"crypto/ecdsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
)

func GenerateCSR(
	priv *ecdsa.PrivateKey,
) ([]byte, error) {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			Organization: []string{"Ashrix"},
			OrganizationalUnit: []string{"Gateway"},
		},
	}

	csrDER, err := x509.CreateCertificateRequest(
		nil, template, priv,
	)

	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(
		&pem.Block{
			Type: "CERTIFICATE REQUEST",
			Bytes: csrDER,
		},
	), nil
}