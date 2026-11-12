package crypto

import (
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

type LocalGatewayPKI struct {
	certificate *tls.Certificate
	leaf        *x509.Certificate
	certPool    *x509.CertPool
}

func NewLocalGatewayPKI(certPath, keyPath string, CACert *x509.Certificate) (*LocalGatewayPKI, error) {
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
	block, _ = pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("invalid cert PEM: %s", keyPath)
	}
	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		return nil, fmt.Errorf("Failed to load key : %w", err)
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
	_, err = cert.Verify(x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
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
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return g.certificate, nil
		},
		RootCAs:    g.certPool,
		ClientCAs:  g.certPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
		MinVersion: tls.VersionTLS13,
	}
}
