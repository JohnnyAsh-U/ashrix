package socks

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

const (
	SOCKS5 = 0x05

	MethodNoAuth       = 0x00
	MethodUserPass     = 0x02
	MethodNoAcceptable = 0xff

	UserPassVersion = 0x01

	CmdConnect      = 0x01
	CmdBind         = 0x02
	CmdUDPAssociate = 0x03

	AddrIPv4   = 0x01
	AddrDomain = 0x03
	AddrIPv6   = 0x04
)

const (
	ReplySucceeded          = 0x00
	ReplyGeneralFailure     = 0x01
	ReplyNotAllowed         = 0x02
	ReplyNetworkUnreachable = 0x03
	ReplyHostUnreachable    = 0x04
	ReplyConnectionRefused  = 0x05
	ReplyTTLExpired         = 0x06
	ReplyCommandUnsupported = 0x07
	ReplyAddressUnsupported = 0x08
)

const (
	MaxMethods = 255
	MaxDomain  = 255
)

type ConnectRequest struct {
	Host string
	Port uint16
}

func (r ConnectRequest) Address() string {
	return net.JoinHostPort(
		r.Host,
		fmt.Sprintf("%d", r.Port),
	)
}

// negotiate performs:
//
// Client:
//
//	VER NMETHODS METHODS...
//
// Server:
//
//	VER METHOD
func negotiate(r io.Reader, w io.Writer) error {
	header := make([]byte, 2)

	if _, err := io.ReadFull(r, header); err != nil {
		return fmt.Errorf("read SOCKS greeting: %w", err)
	}

	if header[0] != SOCKS5 {
		return errors.New("unsupported SOCKS version")
	}

	nmethods := int(header[1])

	if nmethods == 0 {
		return errors.New("client supplied zero authentication methods")
	}

	methods := make([]byte, nmethods)

	if _, err := io.ReadFull(r, methods); err != nil {
		return fmt.Errorf("read SOCKS methods: %w", err)
	}

	for _, method := range methods {
		if method == MethodUserPass {
			_, err := w.Write([]byte{
				SOCKS5,
				MethodUserPass,
			})
			return err
		}
	}

	_, err := w.Write([]byte{
		SOCKS5,
		MethodNoAcceptable,
	})

	return err
}

// RFC 1929:
//
// VER ULEN UNAME PLEN PASSWD
func readUserPass(r io.Reader) (string, string, error) {
	header := make([]byte, 2)

	if _, err := io.ReadFull(r, header); err != nil {
		return "", "", fmt.Errorf(
			"read username/password header: %w",
			err,
		)
	}

	if header[0] != UserPassVersion {
		return "", "", errors.New(
			"unsupported username/password version",
		)
	}

	usernameLen := int(header[1])

	if usernameLen == 0 {
		return "", "", errors.New(
			"empty username",
		)
	}

	username := make([]byte, usernameLen)

	if _, err := io.ReadFull(r, username); err != nil {
		return "", "", fmt.Errorf(
			"read username: %w",
			err,
		)
	}

	if _, err := io.ReadFull(r, header[:1]); err != nil {
		return "", "", fmt.Errorf(
			"read password length: %w",
			err,
		)
	}

	passwordLen := int(header[0])

	if passwordLen == 0 {
		return "", "", errors.New(
			"empty password",
		)
	}

	password := make([]byte, passwordLen)

	if _, err := io.ReadFull(r, password); err != nil {
		return "", "", fmt.Errorf(
			"read password: %w",
			err,
		)
	}

	return string(username), string(password), nil
}

func writeAuthReply(w io.Writer, success bool) error {
	status := byte(0x01)
	if success {
		status = 0x00
	}
	_, err := w.Write([]byte{UserPassVersion, status})
	return err
}

func readConnectRequest(r io.Reader) (ConnectRequest, error) {
	header := make([]byte, 4)

	if _, err := io.ReadFull(r, header); err != nil {
		return ConnectRequest{}, fmt.Errorf(
			"read SOCKS request: %w",
			err,
		)
	}

	if header[0] != SOCKS5 {
		return ConnectRequest{}, errors.New(
			"invalid SOCKS request version",
		)
	}

	if header[1] != CmdConnect {
		return ConnectRequest{}, fmt.Errorf(
			"unsupported SOCKS command: 0x%02x",
			header[1],
		)
	}

	if header[2] != 0x00 {
		return ConnectRequest{}, errors.New(
			"invalid SOCKS reserved byte",
		)
	}

	req := ConnectRequest{}

	switch header[3] {

	case AddrIPv4:
		buf := make([]byte, 4)

		if _, err := io.ReadFull(r, buf); err != nil {
			return req, err
		}

		req.Host = net.IP(buf).String()

	case AddrDomain:
		lengthBuf := []byte{0}

		if _, err := io.ReadFull(r, lengthBuf); err != nil {
			return req, err
		}

		length := int(lengthBuf[0])

		if length == 0 || length > MaxDomain {
			return req, errors.New(
				"invalid domain length",
			)
		}

		buf := make([]byte, length)

		if _, err := io.ReadFull(r, buf); err != nil {
			return req, err
		}

		req.Host = string(buf)

	case AddrIPv6:
		buf := make([]byte, 16)

		if _, err := io.ReadFull(r, buf); err != nil {
			return req, err
		}

		req.Host = net.IP(buf).String()

	default:
		return req, errors.New(
			"unsupported SOCKS address type",
		)
	}

	portBuf := make([]byte, 2)

	if _, err := io.ReadFull(r, portBuf); err != nil {
		return req, err
	}

	req.Port = binary.BigEndian.Uint16(portBuf)

	return req, nil
}

// We return 0.0.0.0:0 because the gateway owns the actual
// destination connection.
func writeConnectReply(
	w io.Writer,
	reply byte,
) error {
	packet := []byte{
		SOCKS5,
		reply,
		0x00,
		AddrIPv4,
		0x00,
		0x00,
		0x00,
		0x00,
		0x00,
		0x00,
	}

	_, err := w.Write(packet)

	return err
}
