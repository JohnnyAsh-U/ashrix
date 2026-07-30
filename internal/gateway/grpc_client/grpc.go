package gateway_grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func GRPCConn(
	ctx context.Context,
	cpURL string,
	tlsConfig *tls.Config,
	pki *crypto.GatewayPKI,
	log *zap.Logger,
) (*grpc.ClientConn, error) {

	conn, err := grpc.NewClient(cpURL,
		// grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)

	if err != nil {
		return nil, &FatalError{
			UserMessage: fmt.Sprintf("Invalide CP target or TLS config: %v", err),
		}
	}

	// if err == nil {
	return conn, nil
	// }

	// log.Warn("Failed to open control stream",
	// 	zap.Int("attempt", attempt),
	// 	zap.Int("max", maxAttempts),
	// 	zap.Error(err),
	// )
	// if attempt == maxAttempts {
	// 	break
	// }

	// errorType := classifyDialError(err)

	// if errorType == errClassUnauthenticated || errorType == errClassNetwork {
	// 	return nil, &FatalError{
	// 		UserMessage: fmt.Sprintf("control stream failed with non recoverable error: %v", err),
	// 	}
	// }

	// log.Warn("TLS/auth error detected - attempt cert renewal", zap.Int("attempt", attempt))
	// renewCtx, renewCancel := context.WithTimeout(ctx, 30*time.Second)

	// renewErr := pki.PreflightRenew(renewCtx)
	// renewCancel()

	// //Renewwal failed - log and retry stream
	// if renewErr != nil {
	// 	log.Warn("Cert renewal failed - retrying stream open with existing cert", zap.Error(renewErr))
	// 	log.Warn("Retrying Again...")
	// 	time.Sleep(10 * time.Second)
	// 	continue
	// }

	//if cert renewal is successful we dial with new tls
	// conn.Close()
}
