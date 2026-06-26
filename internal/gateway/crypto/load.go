package crypto

import (
	"errors"
	"fmt"
	"time"
)

func (g *GatewayPKI) LoadAndVerify() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	fmt.Println(g.cert.Leaf.NotBefore)

	if time.Now().After(g.leaf.NotAfter){
		return ErrCertExpired
	}
	return nil
}

var ErrCertExpired = errors.New("Certificate Expired")