package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/security"
	"fitgiris/internal/service"
)

// MockUserRepository
type mockUserRepo struct {
	mu           sync.Mutex
	usersByID    map[uuid.UUID]*domain.User
	usersByEmail map[string]*domain.User
	tokens       map[string]struct {
		userID    uuid.UUID
		expiresAt time.Time
		revoked   bool
	}
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByID:    make(map[uuid.UUID]*domain.User),
		usersByEmail: make(map[string]*domain.User),
		tokens: make(map[string]struct {
			userID    uuid.UUID
			expiresAt time.Time
			revoked   bool
		}),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.usersByEmail[user.Email]; exists {
		return domain.ErrUserAlreadyExists
	}
	m.usersByID[user.ID] = user
	m.usersByEmail[user.Email] = user
	return nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.usersByEmail[email]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.usersByID[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[tokenHash] = struct {
		userID    uuid.UUID
		expiresAt time.Time
		revoked   bool
	}{userID: userID, expiresAt: expiresAt, revoked: false}
	return nil
}

func (m *mockUserRepo) IsRefreshTokenValid(ctx context.Context, tokenHash string) (bool, uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.tokens[tokenHash]
	if !ok || item.revoked || time.Now().After(item.expiresAt) {
		return false, item.userID, nil
	}
	return true, item.userID, nil
}

func (m *mockUserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item, ok := m.tokens[tokenHash]; ok {
		item.revoked = true
		m.tokens[tokenHash] = item
	}
	return nil
}

// MockAuditRepository
type mockAuditRepo struct {
	mu   sync.Mutex
	logs []domain.AuditLog
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{}
}

func (m *mockAuditRepo) Append(ctx context.Context, log *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockAuditRepo) List(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs, nil
}

func TestAuthService_FullLifecycle(t *testing.T) {
	userRepo := newMockUserRepo()
	auditRepo := newMockAuditRepo()
	jwtManager := security.NewJWTManager("test-jwt-secret-key-32-chars-ok!")

	authService := service.NewAuthService(userRepo, auditRepo, jwtManager)
	ctx := context.Background()

	// 1. Register User
	registerInput := service.RegisterInput{
		Email:     "recep@fitgiris.com",
		Password:  "SecurePass123",
		FirstName: "Recep",
		LastName:  "Giriş",
		Role:      domain.RoleReception,
	}

	ip := "127.0.0.1"
	user, tokens, err := authService.Register(ctx, registerInput, &ip)
	if err != nil {
		t.Fatalf("Register hatasi: %v", err)
	}

	if user.Email != registerInput.Email {
		t.Errorf("Beklenen email %s, alinan %s", registerInput.Email, user.Email)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("Tokenlar bos olamaz")
	}

	// Verify Audit Log was appended
	if len(auditRepo.logs) != 1 {
		t.Fatalf("Audit log kaydedilmeliydi, kayit sayisi: %d", len(auditRepo.logs))
	}
	if auditRepo.logs[0].Action != "USER_REGISTER" {
		t.Errorf("Beklenen audit action USER_REGISTER, alinan: %s", auditRepo.logs[0].Action)
	}

	// 2. Duplicate Register should fail
	_, _, err = authService.Register(ctx, registerInput, &ip)
	if !errors.Is(err, domain.ErrUserAlreadyExists) {
		t.Fatalf("Mukerrer kayitta ErrUserAlreadyExists bekleniyordu, alinan: %v", err)
	}

	// 3. Login with wrong password
	_, _, err = authService.Login(ctx, registerInput.Email, "WrongPassword", &ip)
	if !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("Yanlis parolada ErrInvalidPassword bekleniyordu, alinan: %v", err)
	}

	// 4. Login with correct password
	loginUser, loginTokens, err := authService.Login(ctx, registerInput.Email, registerInput.Password, &ip)
	if err != nil {
		t.Fatalf("Login basarisiz: %v", err)
	}
	if loginUser.ID != user.ID {
		t.Errorf("Beklenen user ID %s, alinan %s", user.ID, loginUser.ID)
	}
	if len(auditRepo.logs) != 2 || auditRepo.logs[1].Action != "USER_LOGIN" {
		t.Errorf("Login sonrasi USER_LOGIN audit logu eklenmeliydi")
	}

	// 5. Refresh Token
	newTokens, err := authService.RefreshToken(ctx, loginTokens.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken basarisiz: %v", err)
	}
	if newTokens.AccessToken == "" {
		t.Fatal("Yeni access token bos olamaz")
	}

	// 6. Old Refresh Token should now be revoked (Token Rotation)
	_, err = authService.RefreshToken(ctx, loginTokens.RefreshToken)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Eski refresh token kullanilmaya calisildiginda ErrUnauthorized bekleniyordu, alinan: %v", err)
	}
}
