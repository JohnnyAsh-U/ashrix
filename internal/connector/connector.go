package connector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	// "math/rand"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/tunnel"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
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

	if cert.Subject.CommonName != appConfig.Connector.ID {
		return fmt.Errorf("configured connector ID %q does not match certificate identity %q", appConfig.Connector.ID, cert.Subject.CommonName)
	}
	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("connector certificate expired; provision a renewed certificate and trust bundle")
	}

	pki, err := startup.NewLocalPKIInitialiser(cert, key, string(bundle.Raw))
	if err != nil {
		return fmt.Errorf("initialize local connector identity: %w", err)
	}
	apps := make([]*pb.ConnectorApps, 0, len(appConfig.Applications))
	for _, app := range appConfig.Applications {
		apps = append(apps, &pb.ConnectorApps{
			Id:       app.Name,
			Name:     app.Name,
			Upstream: app.Target,
		})
	}

	//--------------------Mangement Receiver, HeartBeat, and Tunnel Loop ---------------------------//
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	// errCh := make(chan error, 1)
	defer sessionCancel()

	client := tunnel.NewTunnel(sessionCtx, appConfig, pki.TLSConfig(), log, apps)
	if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("client exited with error", "err", err)
		return err
	}
	return nil
}
