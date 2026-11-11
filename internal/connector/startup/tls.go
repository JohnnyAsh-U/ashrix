package startup

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

type PKIInitialiser struct {
	certificate tls.Certificate
	trustRoots  *x509.CertPool
}

func NewLocalPKIInitialiser(cert *x509.Certificate, privateKey *ecdsa.PrivateKey, trustBundle string) (*PKIInitialiser, error) {
	if cert == nil || privateKey == nil {
		return nil, fmt.Errorf("connector certificate and private key are required")
	}
	trustRoots := x509.NewCertPool()
	if !trustRoots.AppendCertsFromPEM([]byte(trustBundle)) {
		return nil, fmt.Errorf("connector trust bundle contains no certificates")
	}
	return &PKIInitialiser{
		certificate: tls.Certificate{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  privateKey,
			Leaf:        cert,
		},
		trustRoots: trustRoots,
	}, nil
}

func (p *PKIInitialiser) TLSConfig() *tls.Config {
	return &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &p.certificate, nil
		},
		RootCAs:    p.trustRoots,
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{"ashrix-mtls", "h2", "ashrix-quic-v1"},
	}
}
