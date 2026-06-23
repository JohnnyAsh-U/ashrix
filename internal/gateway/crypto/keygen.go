package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

func GenerateECDSAP256()(*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(
		elliptic.P256(),
		rand.Reader,
	)
}