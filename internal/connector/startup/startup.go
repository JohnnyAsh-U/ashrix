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
			var result *gen.ConnectorEnrollResponse
			var newkey = cred.PrivKey
			err := retryWithBackoff(ctx, log, "certificate renewal", func() error {
				var err error
				result, newkey, err = Renew(ctx, cpurl, ConnectorId, log, cred.PrivKey)
				return err
			})
			if err != nil {
				log.Error("certificate renewal interrupted",
					"error", err,
				)
				return nil, fmt.Errorf("cert renewal interrupted: %w", err)
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
		var status *gen.ConnectorStatusResponse
		err = retryWithBackoff(ctx, log, "connector status check", func() error {
			var err error
			status, err = FetchStatus(ctx, cpurl, cred.PrivKey, ConnectorId, log)
			return err
		})
		if err != nil {
			log.Error("connector status check interrupted",
				"error", err,
			)
			return nil, fmt.Errorf("status check interrupted: %w", err)
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
	var status *gen.ConnectorStatusResponse
	err = retryWithBackoff(ctx, log, "connector status check", func() error {
		var err error
		status, err = FetchStatus(ctx, cpurl, newKey, regResult.ConnectorId, log)
		return err
	})
	if err != nil {
		log.Error("connector status check interrupted",
			"error", err,
		)
		return nil, fmt.Errorf("status check interrupted: %w", err)
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
