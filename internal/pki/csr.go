package pki

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
)

func GenerateCSR(privKey *ecdsa.PrivateKey, ID, Component string) ([]byte, error) {
	connectorURI, _ := url.Parse(fmt.Sprintf("ashrix://%s/%s",Component, ID))

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         ID,
			Organization:       []string{"Ashrix"},
			OrganizationalUnit: []string{Component},
		},
		URIs: []*url.URL{connectorURI},
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, privKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	}), nil
}



func ParseCSR(data []byte) (*x509.CertificateRequest, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, errors.New("invalid CSR PEM")
	}

	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("unexpected data after CSR")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CSR: %w", err)
	}

	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("invalid CSR signature: %w", err)
	}

	pub, ok := csr.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("CSR public key must be ECDSA")
	}

	if pub.Curve != elliptic.P256() {
		return nil, errors.New("CSR public key must use P-256")
	}

	return csr, nil
}

