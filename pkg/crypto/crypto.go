package crypto


import (
	"crypto/rand"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
)



// hkdfDeriveKey derives a 32-byte key using HKDF-SHA256 (stdlib only).
// HKDF-Extract: PRK = HMAC-SHA256(salt=nil, ikm=secret)
// HKDF-Expand:  key = HMAC-SHA256(PRK, info || 0x01)
func hkdfDeriveKey(secret, info []byte) []byte {
	// Extract
	mac := hmac.New(sha256.New, nil) // salt = zero
	mac.Write(secret)
	prk := mac.Sum(nil)

	// Expand — one block (32 bytes) is enough for AES-256
	mac2 := hmac.New(sha256.New, prk)
	mac2.Write(info)
	counter := make([]byte, 4)
	binary.BigEndian.PutUint32(counter, 1)
	mac2.Write(counter)
	return mac2.Sum(nil)
}


// aesgcmEncrypt encrypts plaintext with AES-256-GCM.
// Key derived from secret+context using HKDF (stdlib hmac+sha256).
// Format: [12-byte nonce][ciphertext+16-byte tag]
func AesgcmEncrypt(plaintext []byte, secret, context string) ([]byte, error) {
	key := hkdfDeriveKey([]byte(secret), []byte("ashrix-pki:"+context))

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}


// aesgcmDecrypt decrypts AES-256-GCM ciphertext.
func AesgcmDecrypt(ciphertext []byte, secret, context string) ([]byte, error) {
	key := hkdfDeriveKey([]byte(secret), []byte("ashrix-pki:"+context))

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("ciphertext too short")
	}

	plaintext, err := gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
	if err != nil {
		return nil, errors.New("decryption failed — wrong secret or corrupted file")
	}
	return plaintext, nil
}


