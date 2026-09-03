package tlsconfig

import (
	"crypto/tls"
	"strings"
)

const (
	// GRPCALPN is NOT the protocol negotiated by grpc-go.
	// It is only a signal in ClientHello that tells the Gateway
	// to select the mTLS configuration.
	GRPCALPN = "ashrix-mtls"

	// grpc-go ultimately negotiates HTTP/2.
	HTTP2ALPN = "h2"

	HTTP11ALPN = "http/1.1"

	QUICALPN = "ashrix-quic-v1"
)

// GatewayTLS contains the TLS configurations used by the Gateway.
type GatewayTLS struct {
	// Shared is the TLS configuration for TCP :443 when HTTPS is enabled.
	//
	// It supports:
	//   - normal HTTPS
	//   - gRPC over mTLS
	//
	// It is nil when HTTPS is disabled.
	Shared *tls.Config

	// GRPC is the dedicated gRPC mTLS configuration.
	//
	// It is always created because gRPC management traffic must
	// always use mTLS.
	GRPC *tls.Config

	// QUIC is the mTLS configuration for UDP :443.
	QUIC *tls.Config
}

// NewGatewayTLSConfig creates all Gateway TLS configurations.
//
// mtlsTLS:
//   - Ashrix CA/client trust configuration
//   - Ashrix server certificate when available
//
// publicTLS:
//   - Public CA certificate used for browser-facing HTTPS.
//   - May be nil when HTTPS is disabled.
//
// httpsEnabled:
//   - true  => shared HTTPS + gRPC listener
//   - false => plaintext HTTP listener + dedicated gRPC TLS listener
func NewGatewayTLSConfig(
	mtlsTLS *tls.Config,
	publicTLS *tls.Config,
	httpsEnabled bool,
) *GatewayTLS {

	grpcTLS := newGRPCTLSConfig(mtlsTLS)

	quicTLS := newQUICTLSConfig(mtlsTLS)

	var sharedTLS *tls.Config

	if httpsEnabled {
		if publicTLS == nil {
			panic("public TLS configuration is required when HTTPS is enabled")
		}

		sharedTLS = newSharedTLSConfig(
			publicTLS,
			grpcTLS,
		)
	}

	return &GatewayTLS{
		Shared: sharedTLS,
		GRPC:   grpcTLS,
		QUIC:   quicTLS,
	}
}

func newSharedTLSConfig(
	publicTLS *tls.Config,
	grpcTLS *tls.Config,
) *tls.Config {

	cfg := publicTLS.Clone()

	// Normal HTTPS must NOT request client certificates.
	cfg.ClientAuth = tls.NoClientCert
	cfg.ClientCAs = nil

	// Browser HTTPS.
	//
	// h2 is required for HTTP/2.
	// http/1.1 keeps compatibility with normal HTTP clients.
	cfg.NextProtos = []string{
		HTTP2ALPN,
		HTTP11ALPN,
	}

	// Important:
	// returned configs must not call GetConfigForClient again.
	cfg.GetConfigForClient = func(
		hello *tls.ClientHelloInfo,
	) (*tls.Config, error) {

		if supportsALPN(hello.SupportedProtos, GRPCALPN) {
			return grpcTLS, nil
		}

		// Normal browser HTTPS.
		result := cfg.Clone()
		result.GetConfigForClient = nil
		return result, nil
	}

	return cfg
}

func newGRPCTLSConfig(
	mtlsTLS *tls.Config,
) *tls.Config {

	cfg := mtlsTLS.Clone()

	// gRPC clients MUST authenticate with an Ashrix client certificate.
	// cfg.ClientAuth = tls.RequireAndVerifyClientCert

	// The actual gRPC protocol is HTTP/2.
	cfg.NextProtos = []string{
		HTTP2ALPN,
	}

	cfg.GetConfigForClient = nil
	return cfg
}


func newQUICTLSConfig(mtlsTLS *tls.Config) *tls.Config {
	cfg := mtlsTLS.Clone()

	cfg.ClientAuth = tls.RequireAndVerifyClientCert

	cfg.NextProtos = []string{
		QUICALPN,
	}

	cfg.GetConfigForClient = nil

	return cfg
}


func supportsALPN(
	protocols []string,
	target string,
) bool {

	for _, protocol := range protocols {
		if strings.EqualFold(protocol, target) {
			return true
		}
	}

	return false
}