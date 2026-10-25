package security

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
)

type Claims struct {
	UserID       uuid.UUID       `json:"user_id"`
	Email        string          `json:"email"`
	Role         domain.UserRole `json:"role"`
	HomeBranchID *uuid.UUID      `json:"home_branch_id,omitempty"`
	TokenType    string          `json:"token_type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	AccessExpiry time.Time `json:"access_expiry"`
}

type JWTManager struct {
	secretKey     []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{
		secretKey:     []byte(secretKey),
		accessExpiry:  15 * time.Minute,
		refreshExpiry: 7 * 24 * time.Hour,
	}
}

func (m *JWTManager) GenerateTokenPair(user *domain.User) (*TokenPair, error) {
	now := time.Now().UTC()
	accessExpiryTime := now.Add(m.accessExpiry)
	refreshExpiryTime := now.Add(m.refreshExpiry)

	// Access Token Claims
	accessClaims := &Claims{
		UserID:       user.ID,
		Email:        user.Email,
		Role:         user.Role,
		HomeBranchID: user.HomeBranchID,
		TokenType:    "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiryTime),
			Issuer:    "fitgiris",
		},
	}

	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err := accessTokenObj.SignedString(m.secretKey)
	if err != nil {
		return nil, fmt.Errorf("access token imzalanamadi: %w", err)
	}

	// Refresh Token Claims
	refreshClaims := &Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExpiryTime),
			Issuer:    "fitgiris",
		},
	}

	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refreshTokenObj.SignedString(m.secretKey)
	if err != nil {
		return nil, fmt.Errorf("refresh token imzalanamadi: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		AccessExpiry: accessExpiryTime,
	}, nil
}

func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("beklenmeyen imza yontemi: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("gecersiz token")
	}

	return claims, nil
}

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
