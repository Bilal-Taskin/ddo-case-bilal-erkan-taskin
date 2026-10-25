package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fitgiris/internal/domain"
	"fitgiris/internal/handler"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/security"
	"fitgiris/internal/service"
)

// Mock Turnstile Repo for Handler Tests
type mockHndTurnstileRepo struct {
	mu         sync.Mutex
	turnstiles map[uuid.UUID]*domain.Turnstile
	accessLogs []domain.AccessLog
	memberPkgs map[uuid.UUID]*domain.MemberPackage
}

func newMockHndTurnstileRepo() *mockHndTurnstileRepo {
	return &mockHndTurnstileRepo{
		turnstiles: make(map[uuid.UUID]*domain.Turnstile),
		memberPkgs: make(map[uuid.UUID]*domain.MemberPackage),
	}
}

func (m *mockHndTurnstileRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(nil)
}

func (m *mockHndTurnstileRepo) CreateTurnstile(ctx context.Context, t *domain.Turnstile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	m.turnstiles[t.ID] = t
	return nil
}

func (m *mockHndTurnstileRepo) GetTurnstileByID(ctx context.Context, id uuid.UUID) (*domain.Turnstile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.turnstiles[id]
	if !ok {
		return nil, domain.ErrTurnstileNotFound
	}
	return t, nil
}

func (m *mockHndTurnstileRepo) GetTurnstileByBranchAndCode(ctx context.Context, branchID uuid.UUID, code string) (*domain.Turnstile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.turnstiles {
		if t.BranchID == branchID && t.Code == code {
			return t, nil
		}
	}
	return nil, domain.ErrTurnstileNotFound
}

func (m *mockHndTurnstileRepo) GetTurnstileByCode(ctx context.Context, code string) (*domain.Turnstile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.turnstiles {
		if t.Code == code {
			return t, nil
		}
	}
	return nil, domain.ErrTurnstileNotFound
}

func (m *mockHndTurnstileRepo) ListTurnstiles(ctx context.Context, branchID *uuid.UUID, activeOnly bool) ([]domain.Turnstile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.Turnstile
	for _, t := range m.turnstiles {
		list = append(list, *t)
	}
	return list, nil
}

func (m *mockHndTurnstileRepo) UpdateTurnstile(ctx context.Context, t *domain.Turnstile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.turnstiles[t.ID] = t
	return nil
}

func (m *mockHndTurnstileRepo) LogAccess(ctx context.Context, log *domain.AccessLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessLogs = append(m.accessLogs, *log)
	return nil
}

func (m *mockHndTurnstileRepo) LogAccessBatch(ctx context.Context, logs []domain.AccessLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessLogs = append(m.accessLogs, logs...)
	return nil
}

func (m *mockHndTurnstileRepo) GetLastGrantedAccess(ctx context.Context, memberID, branchID uuid.UUID) (*domain.AccessLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.accessLogs) - 1; i >= 0; i-- {
		l := m.accessLogs[i]
		if l.MemberID == memberID && l.BranchID == branchID && l.Status == domain.AccessGranted {
			return &l, nil
		}
	}
	return nil, nil
}

func (m *mockHndTurnstileRepo) ListAccessLogs(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.AccessLog, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.accessLogs, len(m.accessLogs), nil
}

func (m *mockHndTurnstileRepo) GetActiveMemberPackageTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID) (*domain.MemberPackage, error) {
	pkg, ok := m.memberPkgs[memberID]
	if !ok || pkg.Status != domain.StatusActive {
		return nil, domain.ErrMembershipNotFound
	}
	return pkg, nil
}

func (m *mockHndTurnstileRepo) DeductSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	return nil
}

// Mock User Repo
type mockHndUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func (m *mockHndUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}
func (m *mockHndUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (m *mockHndUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}
func (m *mockHndUserRepo) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	return nil
}
func (m *mockHndUserRepo) IsRefreshTokenValid(ctx context.Context, tokenHash string) (bool, uuid.UUID, error) {
	return false, uuid.Nil, nil
}
func (m *mockHndUserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return nil
}

type mockHndAuditRepo struct{}

func (m *mockHndAuditRepo) Append(ctx context.Context, log *domain.AuditLog) error {
	return nil
}
func (m *mockHndAuditRepo) List(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	return nil, nil
}

func setupTurnstileHandlerRouter() (*chi.Mux, *mockHndTurnstileRepo, *mockHndUserRepo, *security.JWTManager) {
	repo := newMockHndTurnstileRepo()
	userRepo := &mockHndUserRepo{users: make(map[uuid.UUID]*domain.User)}
	auditRepo := &mockHndAuditRepo{}

	jwtMgr := security.NewJWTManager("test_secret_for_turnstile_tests_32char_key!")
	authMw := middleware.NewAuthMiddleware(jwtMgr)

	svc := service.NewTurnstileService(repo, userRepo, auditRepo)
	h := handler.NewTurnstileHandler(svc)

	r := chi.NewRouter()
	r.Route("/api/v1/turnstiles", func(r chi.Router) {
		r.Use(authMw.Authenticate)

		r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Post("/access", h.EvaluateAccess)
		r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Post("/sync-offline", h.SyncOffline)
		r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Get("/logs", h.ListAccessLogs)
		r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Get("/", h.ListTurnstiles)
		r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Get("/{id}", h.GetTurnstile)
		r.With(middleware.RequireRoles(domain.RoleManager)).Post("/", h.CreateTurnstile)
	})

	return r, repo, userRepo, jwtMgr
}

func TestTurnstileHandler_CreateTurnstile_RBAC(t *testing.T) {
	r, _, _, jwtMgr := setupTurnstileHandlerRouter()

	p1, _ := jwtMgr.GenerateTokenPair(&domain.User{ID: uuid.New(), Role: domain.RoleManager})
	managerToken := p1.AccessToken
	p2, _ := jwtMgr.GenerateTokenPair(&domain.User{ID: uuid.New(), Role: domain.RoleMember})
	memberToken := p2.AccessToken

	branchID := uuid.New()
	body := map[string]interface{}{
		"branch_id": branchID,
		"code":      "GATE-01",
		"name":      "Giris Kapisi",
		"direction": "IN",
	}
	jsonBody, _ := json.Marshal(body)

	// 1. Manager -> 201 Created
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/turnstiles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+managerToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for manager, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Member -> 403 Forbidden
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/turnstiles", bytes.NewBuffer(jsonBody))
	req2.Header.Set("Authorization", "Bearer "+memberToken)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for member, got %d", w2.Code)
	}
}

func TestTurnstileHandler_EvaluateAccess(t *testing.T) {
	r, repo, userRepo, jwtMgr := setupTurnstileHandlerRouter()

	p, _ := jwtMgr.GenerateTokenPair(&domain.User{ID: uuid.New(), Role: domain.RoleReception})
	receptionToken := p.AccessToken

	branchID := uuid.New()
	gate := &domain.Turnstile{
		ID:        uuid.New(),
		BranchID:  branchID,
		Code:      "IN-01",
		Name:      "Turnike 1",
		Direction: domain.DirectionIn,
		IsActive:  true,
	}
	_ = repo.CreateTurnstile(context.Background(), gate)

	memberID := uuid.New()
	userRepo.users[memberID] = &domain.User{ID: memberID, Role: domain.RoleMember, IsActive: true}
	repo.memberPkgs[memberID] = &domain.MemberPackage{
		ID:          uuid.New(),
		MemberID:    memberID,
		BranchID:    branchID,
		AllBranches: true,
		Status:      domain.StatusActive,
		PackageKind: domain.PackageKindDuration,
	}

	accessBody := map[string]interface{}{
		"turnstile_id": gate.ID,
		"member_id":    memberID,
	}
	jsonBody, _ := json.Marshal(accessBody)

	// Evaluate access
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/turnstiles/access", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+receptionToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data domain.AccessDecision `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Data.Granted || resp.Data.Status != domain.AccessGranted {
		t.Fatalf("expected Granted = true, got %v (%s)", resp.Data.Granted, resp.Data.Reason)
	}
}
