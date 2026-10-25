package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/security"
	"fitgiris/internal/repository"
)

type RegisterInput struct {
	Email        string          `json:"email"`
	Password     string          `json:"password"`
	FirstName    string          `json:"first_name"`
	LastName     string          `json:"last_name"`
	Phone        *string         `json:"phone,omitempty"`
	Role         domain.UserRole `json:"role"`
	HomeBranchID *uuid.UUID      `json:"home_branch_id,omitempty"`
}

type AuthService struct {
	userRepo   repository.UserRepository
	auditRepo  repository.AuditRepository
	jwtManager *security.JWTManager
}

func NewAuthService(
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
	jwtManager *security.JWTManager,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		auditRepo:  auditRepo,
		jwtManager: jwtManager,
	}
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput, ipAddress *string) (*domain.User, *security.TokenPair, error) {
	if !input.Role.IsValid() {
		return nil, nil, domain.ErrInvalidRole
	}
	if len(input.Password) < 6 {
		return nil, nil, errors.New("parola en az 6 karakter olmalidir")
	}

	hash, err := security.HashPassword(input.Password)
	if err != nil {
		return nil, nil, err
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: hash,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Phone:        input.Phone,
		Role:         input.Role,
		HomeBranchID: input.HomeBranchID,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, nil, err
	}

	tokens, err := s.jwtManager.GenerateTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	// Store refresh token
	refreshHash := security.HashToken(tokens.RefreshToken)
	_ = s.userRepo.StoreRefreshToken(ctx, user.ID, refreshHash, time.Now().Add(7*24*time.Hour))

	// Append Audit Log
	newVal, _ := json.Marshal(map[string]interface{}{
		"email": user.Email,
		"role":  user.Role,
	})
	_ = s.auditRepo.Append(ctx, &domain.AuditLog{
		UserID:    &user.ID,
		Action:    "USER_REGISTER",
		Entity:    "users",
		EntityID:  user.ID.String(),
		NewValues: newVal,
		IPAddress: ipAddress,
	})

	return user, tokens, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string, ipAddress *string) (*domain.User, *security.TokenPair, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, nil, domain.ErrInvalidPassword
		}
		return nil, nil, err
	}

	if !user.IsActive {
		return nil, nil, domain.ErrInactiveUser
	}

	if !security.CheckPassword(password, user.PasswordHash) {
		return nil, nil, domain.ErrInvalidPassword
	}

	tokens, err := s.jwtManager.GenerateTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	// Store refresh token
	refreshHash := security.HashToken(tokens.RefreshToken)
	_ = s.userRepo.StoreRefreshToken(ctx, user.ID, refreshHash, time.Now().Add(7*24*time.Hour))

	// Append Audit Log
	_ = s.auditRepo.Append(ctx, &domain.AuditLog{
		UserID:    &user.ID,
		Action:    "USER_LOGIN",
		Entity:    "users",
		EntityID:  user.ID.String(),
		IPAddress: ipAddress,
	})

	return user, tokens, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*security.TokenPair, error) {
	claims, err := s.jwtManager.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	if claims.TokenType != "refresh" {
		return nil, errors.New("gecersiz token tipi")
	}

	tokenHash := security.HashToken(refreshTokenString)
	valid, userID, err := s.userRepo.IsRefreshTokenValid(ctx, tokenHash)
	if err != nil || !valid {
		return nil, domain.ErrUnauthorized
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || !user.IsActive {
		return nil, domain.ErrUnauthorized
	}

	// Revoke old refresh token (Token Rotation for safety)
	_ = s.userRepo.RevokeRefreshToken(ctx, tokenHash)

	// Generate new token pair
	newTokens, err := s.jwtManager.GenerateTokenPair(user)
	if err != nil {
		return nil, fmt.Errorf("yeni token cifti uretilemedi: %w", err)
	}

	// Store new refresh token
	newRefreshHash := security.HashToken(newTokens.RefreshToken)
	_ = s.userRepo.StoreRefreshToken(ctx, user.ID, newRefreshHash, time.Now().Add(7*24*time.Hour))

	return newTokens, nil
}

func (s *AuthService) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}
