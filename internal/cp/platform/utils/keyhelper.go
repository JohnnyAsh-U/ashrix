package utils

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

func VerifySignature(pub *ecdsa.PublicKey, canonical, signature string) error {
	signBytes, err := base64.StdEncoding.DecodeString(signature)

	if err != nil {
		return err
	}

	hash := sha256.Sum256([]byte(canonical))

	if !ecdsa.VerifyASN1(pub, hash[:], signBytes){
		return errors.New("Invalid Signature")
	}
	return nil
}