package bootstrap

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

func VerifyRegisterResponse(resp *APIRegisterResponse) error {

	if resp.Data.GatewayId == "" {
		return fmt.Errorf("Missing gateway_id in CP Response")
	}

	if resp.Data.Certificate == "" {
		return fmt.Errorf("Missing certficate in CP Response")
	}

	//Parse Cert
	block, _ := pem.Decode([]byte(resp.Data.Certificate))
	if block == nil {
		return fmt.Errorf("Invalide Certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("Invalide Certificate PEM, %s", err)
	}

	if cert.NotAfter.Before(cert.NotBefore) {
		return fmt.Errorf("Invalid Certificate Validity Window")
	}

	return nil
}


func VerifyRenewResponse(resp *APIRenewResponse) error {

	if resp.Data.GatewayId == "" {
		return fmt.Errorf("Missing gateway_id in CP Response")
	}

	if resp.Data.Certificate == "" {
		return fmt.Errorf("Missing certficate in CP Response")
	}

	//Parse Cert
	block, _ := pem.Decode([]byte(resp.Data.Certificate))
	if block == nil {
		return fmt.Errorf("Invalide Certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("Invalide Certificate PEM, %s", err)
	}

	if cert.NotAfter.Before(cert.NotBefore) {
		return fmt.Errorf("Invalid Certificate Validity Window")
	}

	return nil
}
