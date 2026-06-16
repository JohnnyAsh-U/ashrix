package crypto

import (
	"crypto/tls"
)

// NewServerTLSConfig creates a standard TLS configuration for CP servers
// using the loaded Control Plane identity.
func NewServerTLSConfig(cp ControlPlaneCrypto) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cp.TLSCertificate()},
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    cp.CACertPool(),
	}
}
