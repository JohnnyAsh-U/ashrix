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
	serverName  string
}

const ALPN = "ashrix/1"

func NewLocalPKIInitialiser(cert *x509.Certificate, privateKey *ecdsa.PrivateKey, trustBundle, serverName string) (*PKIInitialiser, error) {
	if cert == nil || privateKey == nil {
		return nil, fmt.Errorf("connector certificate and private key are required")
	}

	if serverName == "" {
		return nil, fmt.Errorf("gateway server name is required")
	}

	// Parse the cert
	ca, err := x509.ParseCertificate([]byte(trustBundle))
	if err != nil {
		return nil, fmt.Errorf("connector trust bundle contains no certificates")
	}

	trustRoots := x509.NewCertPool()

	trustRoots.AddCert(ca)

	return &PKIInitialiser{
		certificate: tls.Certificate{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  privateKey,
			Leaf:        cert,
		},
		trustRoots: trustRoots,
		serverName: serverName,
	}, nil
}

func (p *PKIInitialiser) TLSConfig() *tls.Config {
	return &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &p.certificate, nil
		},
		RootCAs:    p.trustRoots,
		ServerName: p.serverName,
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{ALPN},
	}
}
