package crypto

import (
	"context"
	"go.uber.org/zap"
)

//This is only called when cert is expired at startup.
// this is the break-glass path, no existing grpc connection needed

func (g *GatewayPKI) PreflightRenew(ctx context.Context) error {
	g.log.Warn("Certificate expired - attempting pre-flight renewal", zap.Time("Expired at", g.leaf.NotAfter))

	if err := g.renew(); err != nil {
		return err
	}
	g.log.Info("Preflight renewal successful", zap.Time("new_expiry", g.leaf.NotAfter))
	return nil
}