package ashrixerrors

import (
	"errors"
	"fmt"

	"github.com/quic-go/quic-go"
)

type Code quic.ApplicationErrorCode

const (
	CodeOK               Code = 0x00
	CodeNoCert           Code = 0x100
	CodeIDMismatch       Code = 0x101
	CodeSerialMismatch   Code = 0x102
	CodeUnknownConnector Code = 0x103
	CodeBadCertificate   Code = 0x104
)

type Error struct {
	Code  Code
	Msg   string
	Cause error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Cause)
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) IsPermanent() bool {
	switch e.Code {
	case CodeNoCert, CodeIDMismatch, CodeSerialMismatch,
		CodeUnknownConnector, CodeBadCertificate:
		return true
	default:
		return false
	}
}

func IsPermanent(err error) bool {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.IsPermanent()
	}
	return false
}

func CodeOf(err error) Code {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return CodeOK
}

func NoCert(cause error) *Error {
	return &Error{Code: CodeNoCert, Msg: "no client certificate", Cause: cause}
}

func IDMismatch(got, want string) *Error {
	return &Error{Code: CodeIDMismatch, Msg: fmt.Sprintf("ConnectorID Mismatch: got %q want %q", got, want)}
}

func SerialMismatch(got, want string) *Error {
	return &Error{Code: CodeSerialMismatch, Msg: fmt.Sprintf("Connector Serial Number Mismatch: got %s want %s", got, want)}
}

func UnknownConnector(id string) *Error {
	return &Error{Code: CodeUnknownConnector, Msg: fmt.Sprintf("unknown connector %q", id)}
}

func BadCertificate(cause error) *Error {
	return &Error{Code: CodeBadCertificate, Msg: "bad certificate", Cause: cause}
}
