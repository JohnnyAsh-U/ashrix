package crypto

import (
	"errors"
	"time"
)

func (g *GatewayPKI) LoadAndVerify() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if time.Now().After(g.leaf.NotAfter){
		return ErrCertExpired
	}
	return nil
}

var ErrCertExpired = errors.New("Certificate Expired")