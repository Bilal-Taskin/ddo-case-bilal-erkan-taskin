package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// In-memory mock membership repository for handler tests
type mockMemRepo struct {
	packageTypes   map[uuid.UUID]*domain.PackageType
	memberships    map[uuid.UUID]*domain.MemberPackage
	packageFreezes map[uuid.UUID][]*domain.PackageFreeze
}

func newMockMemRepo() *mockMemRepo {
	return &mockMemRepo{
		packageTypes:   make(map[uuid.UUID]*domain.PackageType),
		memberships:    make(map[uuid.UUID]*domain.MemberPackage),
		packageFreezes: make(map[uuid.UUID][]*domain.PackageFreeze),
	}
}

func (m *mockMemRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

func (m *mockMemRepo) CreatePackageType(ctx context.Context, pt *domain.PackageType) error {
	m.packageTypes[pt.ID] = pt
	return nil
}

func (m *mockMemRepo) GetPackageTypeByID(ctx context.Context, id uuid.UUID) (*domain.PackageType, error) {
	pt, ok := m.packageTypes[id]
	if !ok {
		return nil, domain.ErrPackageTypeNotFound
	}
	copyPt := *pt
	return &copyPt, nil
}

func (m *mockMemRepo) ListPackageTypes(ctx context.Context, activeOnly bool) ([]domain.PackageType, error) {
	var list []domain.PackageType
	for _, pt := range m.packageTypes {
		if !activeOnly || pt.IsActive {
			list = append(list, *pt)
		}
	}
	return list, nil
}

func (m *mockMemRepo) CreateMemberPackage(ctx context.Context, mp *domain.MemberPackage) error {
	m.memberships[mp.ID] = mp
	return nil
}

func (m *mockMemRepo) CreateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	m.memberships[mp.ID] = mp
	return nil
}

func (m *mockMemRepo) GetMemberPackageByID(ctx context.Context, id uuid.UUID) (*domain.MemberPackage, error) {
	mp, ok := m.memberships[id]
	if !ok {
		return nil, domain.ErrMembershipNotFound
	}
	copyMp := *mp
	if pt, exists := m.packageTypes[copyMp.PackageTypeID]; exists {
		copyMp.PackageName = pt.Name
		copyMp.PackageKind = pt.Type
		copyMp.MaxFreezeDays = pt.MaxFreezeDays
	}
	return &copyMp, nil
}

func (m *mockMemRepo) GetMemberPackageForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.MemberPackage, error) {
	mp, ok := m.memberships[id]
	if !ok {
		return nil, domain.ErrMembershipNotFound
	}
	copyMp := *mp
	if pt, exists := m.packageTypes[copyMp.PackageTypeID]; exists {
		copyMp.PackageName = pt.Name
		copyMp.PackageKind = pt.Type
		copyMp.MaxFreezeDays = pt.MaxFreezeDays
	}
	return &copyMp, nil
}

func (m *mockMemRepo) ListMemberPackages(ctx context.Context, memberID *uuid.UUID, status *domain.MembershipStatus, limit, offset int) ([]domain.MemberPackage, int, error) {
	var list []domain.MemberPackage
	for _, mp := range m.memberships {
		if memberID != nil && mp.MemberID != *memberID {
			continue
		}
		if status != nil && mp.Status != *status {
			continue
		}
		list = append(list, *mp)
	}
	return list, len(list), nil
}

func (m *mockMemRepo) UpdateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	m.memberships[mp.ID] = mp
	return nil
}

func (m *mockMemRepo) CreateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	m.packageFreezes[freeze.MemberPackageID] = append(m.packageFreezes[freeze.MemberPackageID], freeze)
	return nil
}

func (m *mockMemRepo) GetActiveFreezeTx(ctx context.Context, tx pgx.Tx, memberPackageID uuid.UUID) (*domain.PackageFreeze, error) {
	freezes := m.packageFreezes[memberPackageID]
	for i := len(freezes) - 1; i >= 0; i-- {
		if freezes[i].ActualUnfreezeDate == nil {
			return freezes[i], nil
		}
	}
	return nil, domain.ErrNoActiveFreeze
}

func (m *mockMemRepo) UpdateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	return nil
}

func (m *mockMemRepo) GetTotalFrozenDays(ctx context.Context, memberPackageID uuid.UUID) (int, error) {
	var total int
	for _, f := range m.packageFreezes[memberPackageID] {
		total += f.DaysFrozen
	}
	return total, nil
}

func (m *mockMemRepo) CreateTransferTx(ctx context.Context, tx pgx.Tx, transfer *domain.PackageTransfer) error {
	return nil
}

// Mock User & Audit repos for handler tests
type mockUserRepoForHandler struct {
	users map[uuid.UUID]*domain.User
}

func (m *mockUserRepoForHandler) Create(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}
func (m *mockUserRepoForHandler) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForHandler) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}
func (m *mockUserRepoForHandler) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	return nil
}
func (m *mockUserRepoForHandler) IsRefreshTokenValid(ctx context.Context, tokenHash string) (bool, uuid.UUID, error) {
	return false, uuid.Nil, nil
}
func (m *mockUserRepoForHandler) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return nil
}

type mockAuditRepoForHandler struct{}

func (m *mockAuditRepoForHandler) Append(ctx context.Context, log *domain.AuditLog) error {
	return nil
}
func (m *mockAuditRepoForHandler) List(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	return nil, nil
}

func setupTestMembershipRouter() (*chi.Mux, *mockMemRepo, *mockUserRepoForHandler, *security.JWTManager) {
	memRepo := newMockMemRepo()
	userRepo := &mockUserRepoForHandler{users: make(map[uuid.UUID]*domain.User)}
	auditRepo := &mockAuditRepoForHandler{}
	jwtManager := security.NewJWTManager("test-secret-key-32-chars-long-ok!")

	svc := service.NewMembershipService(memRepo, userRepo, auditRepo)
	h := handler.NewMembershipHandler(svc)
	authM := middleware.NewAuthMiddleware(jwtManager)

	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/packages", func(r chi.Router) {
			r.Get("/", h.ListPackageTypes)
			r.Get("/{id}", h.GetPackageType)
			r.Group(func(r chi.Router) {
				r.Use(authM.Authenticate)
				r.With(middleware.RequireRoles(domain.RoleManager)).Post("/", h.CreatePackageType)
			})
		})

		r.Route("/memberships", func(r chi.Router) {
			r.Use(authM.Authenticate)
			r.Get("/", h.List)
			r.Get("/{id}", h.GetByID)
			r.Post("/purchase", h.Purchase)
			r.Post("/{id}/freeze", h.Freeze)
			r.Post("/{id}/unfreeze", h.Unfreeze)
			r.Post("/{id}/transfer", h.Transfer)
			r.Get("/{id}/upgrade-quote", h.UpgradeQuote)
			r.Post("/{id}/upgrade", h.Upgrade)
			r.Post("/{id}/cancel", h.Cancel)
		})
	})

	return r, memRepo, userRepo, jwtManager
}

func TestMembershipHandler_UnauthorizedAccess(t *testing.T) {
	r, _, _, _ := setupTestMembershipRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memberships", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Yetkisiz istek 401 dönmeliydi, dönen: %d", rr.Code)
	}
}

func TestMembershipHandler_RoleAccessControl(t *testing.T) {
	r, _, userRepo, jwtManager := setupTestMembershipRouter()

	member := &domain.User{
		ID:       uuid.New(),
		Email:    "uye@fitgiris.com",
		Role:     domain.RoleMember,
		IsActive: true,
	}
	_ = userRepo.Create(context.Background(), member)

	tokens, err := jwtManager.GenerateTokenPair(member)
	if err != nil {
		t.Fatalf("Token olusturulamadi: %v", err)
	}

	body := []byte(`{"name":"Yillik","type":"DURATION","duration_months":12,"price_kurus":500000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// MEMBER is forbidden from creating packages (requires MANAGER)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("MEMBER rolu paket olusturamamali (403 bekleniyordu), donen: %d", rr.Code)
	}
}

func TestMembershipHandler_FSMConflictResponse(t *testing.T) {
	r, memRepo, userRepo, jwtManager := setupTestMembershipRouter()

	member := &domain.User{
		ID:       uuid.New(),
		Email:    "fsm@fitgiris.com",
		Role:     domain.RoleMember,
		IsActive: true,
	}
	_ = userRepo.Create(context.Background(), member)

	tokens, _ := jwtManager.GenerateTokenPair(member)

	// Add an already CANCELLED membership
	pkgID := uuid.New()
	memRepo.memberships[pkgID] = &domain.MemberPackage{
		ID:             pkgID,
		MemberID:       member.ID,
		Status:         domain.StatusCancelled,
		StartDate:      time.Now().Add(-30 * 24 * time.Hour),
		EndDate:        time.Now().Add(30 * 24 * time.Hour),
		PricePaidKurus: 100000,
	}

	// Attempt to freeze a CANCELLED membership: FSM must return 409 Conflict with ERR_INVALID_STATE_TRANSITION
	freezeBody := []byte(`{"days": 10, "reason": "Deneme"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memberships/"+pkgID.String()+"/freeze", bytes.NewBuffer(freezeBody))
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("Gecersiz FSM durum gecisi 409 donmeliydi, donen: %d", rr.Code)
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("JSON parse hatasi: %v", err)
	}

	if errResp.Error.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("Beklenen error code %s, alinan: %s", domain.ErrCodeInvalidStateTransition, errResp.Error.Code)
	}
}
