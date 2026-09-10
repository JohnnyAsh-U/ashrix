package startup

import (
	"context"
	"time"

	// "crypto/ecdsa"
	// "crypto/x509"
	// "encoding/pem"
	"fmt"
	"log/slog"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"

	// "os"
	// pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

type Result struct {
	PKI    *PKIInitialiser
	Status *gen.ConnectorStatusResponse
}

// Run executes the full connector startup workflow.
//
// Workflow:
//
//		cert + key exist?
//	   YES
//	     Expiry cert date less than 30 days
//		      YES → renew cert with CP
//		          fail → delete cert + key → EXIT
//		      NO  → continue
//		  NO => register (generate keys, prompt token, send CSR)
//		check connector status with CP
//		  fail → delete cert + key → EXIT
//
//		save cert + key to disk
//		init PKIInitialiser
//		return Result
func Run(ctx context.Context, cpurl string, log *slog.Logger, token string, appStorage storage.Storage) (*Result, error) {

	if appStorage.CredentialExists() {
		log.Info("credentials found on disk — loading...")
		cred, err := appStorage.LoadCredential()
		if err != nil {
			return nil, fmt.Errorf("failed to load gateway identity: %w", err)
		}

		ConnectorId := cred.Cert.Subject.CommonName
		remaining := time.Until(cred.Cert.NotAfter)

		if remaining < 30*24*time.Hour {
			log.Info("Credentials Loaded, Renewing...")
			result, newkey, err := Renew(ctx, cpurl, ConnectorId, log, cred.PrivKey)
			if err != nil {
				// Renewal failed — wipe credentials, tell user to restart
				log.Error("renewal failed — deleting credentials",
					"error", err,
					"action", "restart connector to re-register",
				)
				appStorage.ClearCredential()
				return nil, fmt.Errorf(
					"cert renewal failed: %w\n"+
						"Credentials deleted. Restart connector to re-register.",
					err,
				)
			}
			if err := appStorage.SaveCredential(result, newkey); err != nil {
				return nil, err
			}
		}

		// ── Reload cert + key ────────────────────────────────
		cred, err = appStorage.LoadCredential()

		if err != nil {
			return nil, fmt.Errorf("failed to load gateway identity: %w", err)
		}

		log.Info("credentials loaded")

		// ── Check status ──────────────────────────────────────────
		status, err := FetchStatus(ctx, cpurl, cred.PrivKey, ConnectorId, log)
		if err != nil {
			log.Error("status check failed after renewal — deleting credentials",
				"error", err,
			)
			appStorage.ClearCredential()
			return nil, fmt.Errorf(
				"status check failed: %w\n"+
					"Credentials deleted. Restart connector to re-register.",
				err,
			)
		}
		// ── Build PKIInitialiser ───────────────────────────────────
		pkiInit, err := NewPKIInitialiser(
			cred.Cert,
			cred.PrivKey,
			cred.Bundle,
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
			"error", err,
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
