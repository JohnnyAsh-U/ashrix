package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"
)

var ErrLocalCertExpired = fmt.Errorf("gateway certificate expired")

const ALPN = "ashrix/1"

type LocalGatewayPKI struct {
	certificate *tls.Certificate
	leaf        *x509.Certificate
	certPool    *x509.CertPool
}

func NewLocalGatewayPKI(certPath, keyPath string, CACert *x509.Certificate, serverName string) (*LocalGatewayPKI, error) {
	if CACert == nil || !CACert.IsCA || CACert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, errors.New("configured certificate is not a signing CA")
	}
	if serverName == "" {
		return nil, errors.New("gateway server name is required")
	}

	keyPem, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", keyPath, err)
	}
	block, _ := pem.Decode(keyPem)
	if block == nil {
		return nil, errors.New("failed to load key PEM")
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)

	if err != nil {
		return nil, fmt.Errorf("Failed to load cert : %w", err)
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", certPath, err)
	}
	block, rest := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("invalid cert PEM: %s", keyPath)
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		return nil, errors.New("Trailing byte found after parsing cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		return nil, fmt.Errorf("Failed to load key : %w", err)
	}

	if cert.IsCA || cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, errors.New("gateway certificate must be a non-CA signing certificate")
	}

	// Verify the key and cert
	//Verify public key
	certPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Public Key Invalid")
	}

	//Verify if key and public matches
	if !certPub.Equal(&key.PublicKey) {
		return nil, fmt.Errorf("Public Key and Cert Doesn't match")
	}

	pool := x509.NewCertPool()
	pool.AddCert(CACert)

	now := time.Now()
	if now.Before(cert.NotBefore) {
		return nil, fmt.Errorf("gateway certificate is not valid before %s", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return nil, fmt.Errorf("gateway certificate expired %s", cert.NotBefore)

	}

	// Check the serverauth ext key usage
	if len(cert.ExtKeyUsage) == 0 {
		return nil, fmt.Errorf("No key Usage found")
	}

	var isValid = true
	for _, usage := range cert.ExtKeyUsage {
		if usage != x509.ExtKeyUsageServerAuth {
			isValid = false
		}
	}

	if !isValid {
		return nil, fmt.Errorf("Cert has other ExtKeyUsage instead of only ServerAuth")
	}

	_, err = cert.Verify(x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: now,
		DNSName:     serverName,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})

	if err != nil {
		return nil, fmt.Errorf("e: %w", err)
	}

	return &LocalGatewayPKI{
		certificate: &tls.Certificate{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  key,
			Leaf:        cert,
		},
		leaf:     cert,
		certPool: pool,
	}, nil
}

func (g *LocalGatewayPKI) Cert() *x509.Certificate { return g.leaf }

func (g *LocalGatewayPKI) GetTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return g.certificate, nil
		},
		RootCAs:    g.certPool,
		ClientCAs:  g.certPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{ALPN},
	}
}
