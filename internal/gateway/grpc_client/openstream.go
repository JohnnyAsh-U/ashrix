package gateway_grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func OpenStream(
	ctx context.Context,
	cpURL string,
	tlsConfig *tls.Config,
	pki *crypto.GatewayPKI,
	log *zap.Logger,
) (*grpc.ClientConn, proto.ControlPlaneService_ConnectClient, error) {

	const maxAttempts = 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := grpc.NewClient(cpURL,
			// grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)

		

		if err != nil {
			return nil, nil, &FatalError{
				UserMessage: fmt.Sprintf("Invalide CP target or TLS config: %v", err),
			}
		}

		client := proto.NewControlPlaneServiceClient(conn)

		stream, err := client.Connect(ctx)


		if err == nil {
			return conn,stream, nil
		}

		log.Warn("Failed to open control stream",
			zap.Int("attempt", attempt),
			zap.Int("max", maxAttempts),
			zap.Error(err),
		)
		if attempt == maxAttempts {
			break
		}

		// errorType := classifyDialError(err)

		// if errorType == errClassUnauthenticated || errorType == errClassNetwork {
		// 	return nil, &FatalError{
		// 		UserMessage: fmt.Sprintf("control stream failed with non recoverable error: %v", err),
		// 	}
		// }

		log.Warn("TLS/auth error detected - attempt cert renewal", zap.Int("attempt", attempt))
		renewCtx, renewCancel := context.WithTimeout(ctx, 30*time.Second)

		renewErr := pki.PreflightRenew(renewCtx)
		renewCancel()

		//Renewwal failed - log and retry stream
		if renewErr != nil {
			log.Warn("Cert renewal failed - retrying stream open with existing cert", zap.Error(renewErr))
			log.Warn("Retrying Again...")
			time.Sleep(10 * time.Second)
			continue
		}

		//if cert renewal is successful we dial with new tls
		conn.Close()

		log.Info("Cert renewed and redialed - retry stream open", zap.Int("next attempt", attempt+1))
	}

	return nil,nil, &FatalError{
		UserMessage: fmt.Sprintf("failed to open control stream after %d attempts -"+
			"check CP reachability and certificate validity. "+
			"Run ashrix-gateway resiter --cp-url=xxx --token=xxxx",
			maxAttempts,
		),
	}
}
