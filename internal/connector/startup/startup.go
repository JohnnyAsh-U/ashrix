package startup

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/connector/pki"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)

type Result struct {
	PKI    *PKIInitialiser
	Status *gen.ConnectorStatusResponse
}

// Run executes the full connector startup workflow.
//
// Workflow:
//
//	cert + key exist?
//	  YES → renew cert with CP
//	          fail → delete cert + key → EXIT
//	  NO  → register (generate keys, prompt token, send CSR)
//	          fail → EXIT
//
//	check connector status with CP
//	  fail → delete cert + key → EXIT
//
//	save cert + key to disk
//	init PKIInitialiser
//	return Result
func Run(ctx context.Context, cpurl, certDir, secret, context string, log *zap.Logger, token string) (*Result, error) {
	certPath := filepath.Join(certDir, "connector.crt")
	trustPath := filepath.Join(certDir, "bundle.crt")
	keyPath := filepath.Join(certDir, "connector.key.enc")

	certExists, err := filehelper.FileExists(certDir)
	trustExist, err := filehelper.FileExists(trustPath)
	keyExist, err := filehelper.FileExists(keyPath)

	if err != nil {
		return nil, fmt.Errorf("Error Checking files")
	}

	if certExists && trustExist && keyExist {
		log.Info("credentials found on disk — loading...")
		key, cert, err := pki_utils.LoadKeyAndCert(keyPath, certPath, secret, context)
		if err != nil {
			return nil, fmt.Errorf("failed to load gateway identity: %w", err)
		}

		// Load Trust Bundle
		_, err = os.ReadFile(trustPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read trust bundle: %w", err)
		}

		log.Info("Credentials Loaded, Renewing...")

		ConnectorId := cert.Subject.CommonName

		result, newkey, err := Renew(ctx, cpurl, ConnectorId, log, key)
		if err != nil {
			// Renewal failed — wipe credentials, tell user to restart
			log.Error("renewal failed — deleting credentials",
				zap.Error(err),
				zap.String("action", "restart connector to re-register"),
			)
			os.RemoveAll(certDir)
			return nil, fmt.Errorf(
				"cert renewal failed: %w\n"+
					"Credentials deleted. Restart connector to re-register.",
				err,
			)
		}

		// ── Check status ──────────────────────────────────────────
		status, err := FetchStatus(ctx, cpurl, newkey, ConnectorId, log)
		if err != nil {
			log.Error("status check failed after renewal — deleting credentials",
				zap.Error(err),
			)
			os.RemoveAll(certDir)
			return nil, fmt.Errorf(
				"status check failed: %w\n"+
					"Credentials deleted. Restart connector to re-register.",
				err,
			)
		}

		// ── Save renewed cert + key ────────────────────────────────

		if err := SaveCred(result, certPath, keyPath, trustPath, newkey, context, secret); err != nil {
			return nil, err
		}

		log.Info("credentials renewed and saved")

		// ── Build PKIInitialiser ───────────────────────────────────
		pkiInit, err := NewPKIInitialiser(
			cert,
			key,
			result.TrustBundle,
			status.ConnectorId,
			cpurl,
			certPath,
		keyPath,
		trustPath,
			log,
		)
		if err != nil {
			return nil, fmt.Errorf("init PKI: %w", err)
		}

		return &Result{PKI: pkiInit, Status: status}, nil
	}

	// ── BRANCH: no credentials — register ────────────────────────
	log.Info("no credentials found — starting registration")

	regResult, newKey, err := Register(ctx, cpurl, log, token)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}

	// ── Check status ──────────────────────────────────────────
	status, err := FetchStatus(ctx, cpurl, newKey, regResult.ConnectorId, log)
	if err != nil {
		log.Error("status check failed after renewal — deleting credentials",
			zap.Error(err),
		)
		os.RemoveAll(certDir)
		return nil, fmt.Errorf(
			"status check failed: %w\n"+
				"Credentials deleted. Restart connector to re-register.",
			err,
		)
	}

	// ── Save renewed cert + key ────────────────────────────────

	if err := SaveCred(regResult, certPath, keyPath, trustPath, newKey, context, secret); err != nil {
		return nil, err
	}

	log.Info("credentials saved to disk",
		zap.String("cert", certPath),
		zap.String("key", keyPath),
	)

	_, cert, err := pki_utils.LoadKeyAndCert(keyPath, certPath, secret, context)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway identity: %w", err)
	}

	// ── Build PKIInitialiser ──────────────────────────────────────
	pkiInit, err := NewPKIInitialiser(
		cert,
		newKey,
		regResult.TrustBundle,
		status.ConnectorId,
		cpurl,
		certPath,
		keyPath,
		trustPath,
		log,
	)
	if err != nil {
		return nil, fmt.Errorf("init PKI: %w", err)
	}

	return &Result{PKI: pkiInit, Status: status}, nil
}

func SaveCred(regResult *gen.ConnectorEnrollResponse, certPath, keyPath, trustPath string, newKey *ecdsa.PrivateKey, context, secret string) error {
	// ── Save renewed cert + key ────────────────────────────────
	//Key
	if err := pki_utils.WriteKey(keyPath, newKey, secret, context); err != nil {
		return err
	}

	//Cert
	newCert := regResult.Certificate
	block, _ := pem.Decode([]byte(newCert))
	if block == nil {
		return fmt.Errorf("Failed to decode PEM Certificate")
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("Expected Certificate pem block got %q", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	if err := pki_utils.WriteCert(certPath, cert); err != nil {
		return fmt.Errorf("save renewed cert credentials: %w", err)
	}

	//Bundle
	newBundle := regResult.TrustBundle
	block, _ = pem.Decode([]byte(newBundle))
	if block == nil {
		return fmt.Errorf("Failed to decode PEM Certificate")
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("Expected Certificate pem block got %q", block.Type)
	}

	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	if err := pki_utils.WriteCert(trustPath, cert); err != nil {
		return fmt.Errorf("save renewed bundle credentials: %w", err)
	}
	return nil
}
