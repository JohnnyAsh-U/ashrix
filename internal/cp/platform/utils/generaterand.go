package utils
	
import (
	"crypto/rand"
	"math/big"
)

// GenerateRandomString generates a random string of the specified length using alphanumeric characters.
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback or panic if crypto/rand fails
			panic(err)
		}
		result[i] = charset[num.Int64()]
	}
	return string(result)
}