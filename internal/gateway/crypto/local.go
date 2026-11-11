package crypto

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pkiutils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
)

var ErrLocalCertExpired = fmt.Errorf("gateway certificate expired")

type LocalGatewayPKI struct {
	certificate *tls.Certificate
	leaf        *x509.Certificate
	certPool    *x509.CertPool
}

func NewLocalGatewayPKI(dataDir, secret, encryptionContext string) (*LocalGatewayPKI, error) {
	keyPath := filepath.Join(dataDir, "gateway.key.enc")
	certPath := filepath.Join(dataDir, "gateway.crt")
	trustPath := filepath.Join(dataDir, "bundle.crt")
	privateKey, cert, err := pkiutils.LoadKeyAndCert(keyPath, certPath, secret, encryptionContext)
	if err != nil {
		return nil, fmt.Errorf("load local gateway identity: %w", err)
	}
	trustBundle, err := os.ReadFile(trustPath)
	if err != nil {
		return nil, fmt.Errorf("read local gateway trust bundle: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(trustBundle) {
		return nil, fmt.Errorf("parse local gateway trust bundle")
	}
	return &LocalGatewayPKI{
		certificate: &tls.Certificate{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  privateKey,
			Leaf:        cert,
		},
		leaf:     cert,
		certPool: pool,
	}, nil
}

func (g *LocalGatewayPKI) Cert() *x509.Certificate { return g.leaf }

func (g *LocalGatewayPKI) LoadAndVerify() error {
	now := time.Now()
	if now.Before(g.leaf.NotBefore) {
		return fmt.Errorf("gateway certificate is not valid before %s", g.leaf.NotBefore)
	}
	if now.After(g.leaf.NotAfter) {
		return ErrLocalCertExpired
	}
	_, err := g.leaf.Verify(x509.VerifyOptions{
		Roots:       g.certPool,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err != nil {
		return fmt.Errorf("verify local gateway certificate: %w", err)
	}
	return nil
}

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
