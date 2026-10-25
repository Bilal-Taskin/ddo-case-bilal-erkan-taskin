package security

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("parola ozetlenemedi: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies whether the plain password matches the stored bcrypt hash.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
