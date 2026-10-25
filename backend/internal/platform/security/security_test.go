package security_test

import (
	"testing"

	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/security"
)

func TestPasswordHashing(t *testing.T) {
	password := "SecretPass123!"

	hash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("Parola hashlenemedi: %v", err)
	}

	if hash == password {
		t.Error("Hash düz metin ile aynı olamaz")
	}

	if !security.CheckPassword(password, hash) {
		t.Error("Doğru parola hash ile eşleşmeliydi")
	}

	if security.CheckPassword("WrongPassword", hash) {
		t.Error("Yanlış parola eşleşmemeliydi")
	}
}

func TestJWTTokenGenerationAndValidation(t *testing.T) {
	secret := "test-secret-key-32-characters-minimum!"
	jwtManager := security.NewJWTManager(secret)

	userID := uuid.New()
	user := &domain.User{
		ID:        userID,
		Email:     "trainer@fitgiris.com",
		Role:      domain.RoleTrainer,
		FirstName: "Ahmet",
		LastName:  "Yılmaz",
		IsActive:  true,
	}

	tokens, err := jwtManager.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("Token çifti üretilemedi: %v", err)
	}

	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("Tokenlar boş olamaz")
	}

	// Validate Access Token
	claims, err := jwtManager.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("Access token doğrulanamadı: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Beklenen UserID %s, alınan %s", userID, claims.UserID)
	}

	if claims.Role != domain.RoleTrainer {
		t.Errorf("Beklenen Rol %s, alınan %s", domain.RoleTrainer, claims.Role)
	}

	if claims.TokenType != "access" {
		t.Errorf("Beklenen TokenType 'access', alınan '%s'", claims.TokenType)
	}

	// Validate Refresh Token
	refreshClaims, err := jwtManager.ValidateToken(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh token doğrulanamadı: %v", err)
	}

	if refreshClaims.TokenType != "refresh" {
		t.Errorf("Beklenen TokenType 'refresh', alınan '%s'", refreshClaims.TokenType)
	}
}

func TestJWTInvalidSignature(t *testing.T) {
	manager1 := security.NewJWTManager("secret-key-number-1111111111111111")
	manager2 := security.NewJWTManager("secret-key-number-2222222222222222")

	user := &domain.User{
		ID:    uuid.New(),
		Email: "test@fitgiris.com",
		Role:  domain.RoleMember,
	}

	tokens, _ := manager1.GenerateTokenPair(user)

	// Validate with different key
	_, err := manager2.ValidateToken(tokens.AccessToken)
	if err == nil {
		t.Error("Farklı secret key ile imzalanan token reddedilmeliydi")
	}
}

func TestTokenHashDeterminism(t *testing.T) {
	token := "some-random-token-string"
	hash1 := security.HashToken(token)
	hash2 := security.HashToken(token)

	if hash1 != hash2 {
		t.Error("Aynı token için hash değerleri aynı olmalıdır")
	}
}
