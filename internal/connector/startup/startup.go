package startup

import (
	"context"
	// "crypto/ecdsa"
	// "crypto/x509"
	// "encoding/pem"
	"fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"
	// "os"
	// pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
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
func Run(ctx context.Context, cpurl string, log *zap.Logger, token string, appStorage storage.Storage) (*Result, error) {

	if appStorage.CredentialExists() {
		log.Info("credentials found on disk — loading...")
		cred, err := appStorage.LoadCredential()
		if err != nil {
			return nil, fmt.Errorf("failed to load gateway identity: %w", err)
		}

		log.Info("Credentials Loaded, Renewing...")

		ConnectorId := cred.Cert.Subject.CommonName

		result, newkey, err := Renew(ctx, cpurl, ConnectorId, log, cred.PrivKey)
		if err != nil {
			// Renewal failed — wipe credentials, tell user to restart
			log.Error("renewal failed — deleting credentials",
				zap.Error(err),
				zap.String("action", "restart connector to re-register"),
			)
			appStorage.ClearCredential()
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
			appStorage.ClearCredential()
			return nil, fmt.Errorf(
				"status check failed: %w\n"+
					"Credentials deleted. Restart connector to re-register.",
				err,
			)
		}

		// ── Save renewed cert + key ────────────────────────────────

		if err := appStorage.SaveCredential(result, newkey); err != nil {
			return nil, err
		}

		cred, err = appStorage.LoadCredential()

		if err != nil {
			return nil, fmt.Errorf("failed to load gateway identity: %w", err)
		}

		log.Info("credentials renewed and saved")

		// ── Build PKIInitialiser ───────────────────────────────────
		pkiInit, err := NewPKIInitialiser(
			cred.Cert,
			newkey,
			result.TrustBundle,
			status.ConnectorId,
			cpurl,
			log,
			appStorage,
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
		appStorage.ClearCredential()
		return nil, fmt.Errorf(
			"status check failed: %w\n"+
				"Credentials deleted. Restart connector to re-register.",
			err,
		)
	}

	// ── Save renewed cert + key ────────────────────────────────

	if err := appStorage.SaveCredential(regResult, newKey); err != nil {
		return nil, err
	}

	log.Info("credentials saved to disk")

	cred, err := appStorage.LoadCredential()

	if err != nil {
		return nil, fmt.Errorf("failed to load gateway identity: %w", err)
	}

	// ── Build PKIInitialiser ──────────────────────────────────────
	pkiInit, err := NewPKIInitialiser(
		cred.Cert,
		newKey,
		regResult.TrustBundle,
		status.ConnectorId,
		cpurl,
		log,
		appStorage,
	)
	if err != nil {
		return nil, fmt.Errorf("init PKI: %w", err)
	}

	return &Result{PKI: pkiInit, Status: status}, nil
}

// func SaveCred(regResult *gen.ConnectorEnrollResponse, certPath, keyPath, trustPath string, newKey *ecdsa.PrivateKey, context, secret string) error {
// 	// ── Save renewed cert + key ────────────────────────────────
// 	//Key
// 	if err := pki_utils.WriteKey(keyPath, newKey, secret, context); err != nil {
// 		return err
// 	}

// 	//Cert
// 	newCert := regResult.Certificate
// 	block, _ := pem.Decode([]byte(newCert))
// 	if block == nil {
// 		return fmt.Errorf("Failed to decode PEM Certificate")
// 	}
// 	if block.Type != "CERTIFICATE" {
// 		return fmt.Errorf("Expected Certificate pem block got %q", block.Type)
// 	}

// 	cert, err := x509.ParseCertificate(block.Bytes)
// 	if err != nil {
// 		return err
// 	}
// 	if err := pki_utils.WriteCert(certPath, cert); err != nil {
// 		return fmt.Errorf("save renewed cert credentials: %w", err)
// 	}

// 	//Bundle
// 	newBundle := regResult.TrustBundle
// 	block, _ = pem.Decode([]byte(newBundle))
// 	if block == nil {
// 		return fmt.Errorf("Failed to decode PEM Certificate")
// 	}
// 	if block.Type != "CERTIFICATE" {
// 		return fmt.Errorf("Expected Certificate pem block got %q", block.Type)
// 	}

// 	cert, err = x509.ParseCertificate(block.Bytes)
// 	if err != nil {
// 		return err
// 	}
// 	if err := pki_utils.WriteCert(trustPath, cert); err != nil {
// 		return fmt.Errorf("save renewed bundle credentials: %w", err)
// 	}
// 	return nil
// }
