package common

import (
	"crypto/rand"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

const keyChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateKey returns a cryptographically random string of length n
// drawn from [a-zA-Z0-9], used for API keys and session secrets.
func GenerateKey(n int) string {
	b := make([]byte, n)
	max := big.NewInt(int64(len(keyChars)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err) // crypto/rand failure is unrecoverable
		}
		b[i] = keyChars[idx.Int64()]
	}
	return string(b)
}

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword reports whether the plaintext password matches the bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
