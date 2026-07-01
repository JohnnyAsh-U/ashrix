package gateway_grpc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── Error classification ──────────────────────────────────────────────────────

type errClass int

const (
    errClassNetwork         errClass = iota
    errClassTLSRejected
    errClassUnauthenticated
)

func classifyDialError(err error) errClass {
    // TLS / certificate errors — cert rejected at handshake
	fmt.Println(err)
    if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
        return errClassTLSRejected
    }

    // x509 errors — cert invalid, expired, or untrusted
    var x509Err x509.CertificateInvalidError
    var unknownAuthErr x509.UnknownAuthorityError
    if errors.As(err, &x509Err) || errors.As(err, &unknownAuthErr) {
        return errClassTLSRejected
    }

    // gRPC status errors
    if s, ok := status.FromError(err); ok {
        switch s.Code() {
        case codes.Unauthenticated, codes.PermissionDenied:
            return errClassUnauthenticated
        }
    }

    // Everything else — network, timeout, connection refused
    return errClassNetwork
}

// ── Error types ───────────────────────────────────────────────────────────────

// FatalError is an error that cannot be recovered by retrying.
// The UserMessage is shown directly to the operator.
type FatalError struct {
    Err         error
    UserMessage string
}

func (e *FatalError) Error() string { return e.UserMessage }
func (e *FatalError) Unwrap() error { return e.Err }