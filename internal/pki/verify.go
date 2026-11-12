package pki

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type VerifyOptions struct {
	ConnectorID       string
	ExpectedSerial    string
	CurrentTime       time.Time
}


func NormalizeSerial(serial string) (string, error) {
	serial = strings.ReplaceAll(serial, ":", "")
	serial = strings.TrimSpace(serial)
	serial = strings.ToUpper(serial)

	if serial == "" {
		return "", errors.New("certificate serial is required")
	}

	for _, r := range serial {
		if !strings.ContainsRune("0123456789ABCDEF", r) {
			return "", errors.New("certificate serial must be hexadecimal")
		}
	}

	if len(serial)%2 != 0 {
		serial = "0" + serial
	}

	return serial, nil
}

func SerialString(serial *big.Int) string {
	if serial == nil {
		return ""
	}

	return strings.ToUpper(serial.Text(16))
}

func VerifyConnectorCertificate(
	cert *x509.Certificate,
	ca *x509.Certificate,
	opts VerifyOptions,
) error {
	if cert == nil || ca == nil {
		return errors.New("certificate and CA are required")
	}

	if opts.ConnectorID == "" {
		return errors.New("connector ID is required")
	}

	now := opts.CurrentTime
	if now.IsZero() {
		now = time.Now()
	}

	if now.Before(ca.NotBefore) || now.After(ca.NotAfter) {
		return errors.New("CA certificate is not currently valid")
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca)

	intermediates := x509.NewCertPool()
	for _, issuer := range cert.IssuingCertificateURL {
		_ = issuer
	}

	_, err := cert.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}

	if cert.IsCA {
		return errors.New("connector certificate must not be a CA")
	}

	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("connector certificate lacks digital signature usage")
	}

	if len(cert.URIs) != 1 {
		return errors.New("connector certificate must contain exactly one URI SAN")
	}


	if opts.ExpectedSerial != "" {
		expectedSerial, err := NormalizeSerial(opts.ExpectedSerial)
		if err != nil {
			return err
		}

		actualSerial, err := NormalizeSerial(SerialString(cert.SerialNumber))
		if err != nil {
			return err
		}

		if actualSerial != expectedSerial {
			return errors.New("certificate serial is not currently authorized")
		}
	}

	// Ensure the certificate was directly issued by this CA.
	if cert.CheckSignatureFrom(ca) != nil {
		return errors.New("certificate was not signed by configured CA")
	}

	return nil
}



func ParseCertificatePEM(data []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("invalid certificate PEM")
	}

	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("unexpected data after certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return cert, nil
}
