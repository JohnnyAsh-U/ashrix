package connector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/tunnel"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
	// pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

func RunConnector(ctx context.Context, appConfig *config.Config, log *slog.Logger) error {
	// credential, err := appStorage.LoadCredential()
	key, err := identity.LoadPrivateKey(appConfig.Connector.PrivateKey)
	if err != nil {
		return fmt.Errorf("load local connector credentials: %w", err)
	}

	cert, err := identity.LoadCert(appConfig.Connector.Certificate)

	if err != nil {
		return fmt.Errorf("load local connector credentials: %w", err)
	}

	bundle, err := identity.LoadCert(appConfig.Connector.CA)

	if err != nil {
		return fmt.Errorf("load local connector credentials: %w", err)
	}

	now := time.Now()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return fmt.Errorf("connector certificate is not currently valid; provision a valid certificate")
	}
	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("connector certificate expired; provision a renewed certificate and trust bundle")
	}

	if err := identity.VerifyLocalKeyPair(cert, key); err != nil {
		return fmt.Errorf("verify connector certificate/private key: %w", err)
	}
	if err := pki.VerifyConnectorCertificate(cert, bundle, pki.VerifyOptions{
		ConnectorID: appConfig.Connector.ID,
	}); err != nil {
		return fmt.Errorf("verify connector certificate trust and identity: %w", err)
	}

	localPKI, err := startup.NewLocalPKIInitialiser(cert, key, string(bundle.Raw), appConfig.Gateway.ServerName)

	if err != nil {
		return fmt.Errorf("initialize local connector identity: %w", err)
	}

	client := tunnel.NewTunnel(appConfig, localPKI.TLSConfig(), log)
	if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("client exited with error", "err", err)
		return err
	}
	return nil
}
