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

type mockPTRepo struct {
	packages     map[uuid.UUID]*domain.PTPackage
	appointments map[uuid.UUID]*domain.PTAppointment
}

func newMockPTRepo() *mockPTRepo {
	return &mockPTRepo{
		packages:     make(map[uuid.UUID]*domain.PTPackage),
		appointments: make(map[uuid.UUID]*domain.PTAppointment),
	}
}

func (m *mockPTRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return nil, nil
}

func (m *mockPTRepo) CreatePackage(ctx context.Context, pkg *domain.PTPackage) error {
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPTRepo) CreatePackageTx(ctx context.Context, tx pgx.Tx, pkg *domain.PTPackage) error {
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPTRepo) GetPackageByID(ctx context.Context, id uuid.UUID) (*domain.PTPackage, error) {
	if p, ok := m.packages[id]; ok {
		return p, nil
	}
	return nil, domain.ErrPTNotFound
}

func (m *mockPTRepo) GetPackageForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.PTPackage, error) {
	return m.GetPackageByID(ctx, id)
}

func (m *mockPTRepo) UpdatePackageTx(ctx context.Context, tx pgx.Tx, pkg *domain.PTPackage) error {
	m.packages[pkg.ID] = pkg
	return nil
}

func (m *mockPTRepo) ListPackagesByMember(ctx context.Context, memberID uuid.UUID) ([]domain.PTPackage, error) {
	var list []domain.PTPackage
	for _, p := range m.packages {
		if p.MemberID == memberID {
			list = append(list, *p)
		}
	}
	return list, nil
}

func (m *mockPTRepo) ListPackagesByTrainer(ctx context.Context, trainerID uuid.UUID) ([]domain.PTPackage, error) {
	var list []domain.PTPackage
	for _, p := range m.packages {
		if p.TrainerID == trainerID {
			list = append(list, *p)
		}
	}
	return list, nil
}

func (m *mockPTRepo) CreateAppointment(ctx context.Context, app *domain.PTAppointment) error {
	m.appointments[app.ID] = app
	return nil
}

func (m *mockPTRepo) CreateAppointmentTx(ctx context.Context, tx pgx.Tx, app *domain.PTAppointment) error {
	m.appointments[app.ID] = app
	return nil
}

func (m *mockPTRepo) GetAppointmentByID(ctx context.Context, id uuid.UUID) (*domain.PTAppointment, error) {
	if a, ok := m.appointments[id]; ok {
		return a, nil
	}
	return nil, domain.ErrPTNotFound
}

func (m *mockPTRepo) GetAppointmentForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.PTAppointment, error) {
	return m.GetAppointmentByID(ctx, id)
}

func (m *mockPTRepo) UpdateAppointmentTx(ctx context.Context, tx pgx.Tx, app *domain.PTAppointment) error {
	m.appointments[app.ID] = app
	return nil
}

func (m *mockPTRepo) ListAppointmentsByMember(ctx context.Context, memberID uuid.UUID) ([]domain.PTAppointment, error) {
	var list []domain.PTAppointment
	for _, a := range m.appointments {
		if a.MemberID == memberID {
			list = append(list, *a)
		}
	}
	return list, nil
}

func (m *mockPTRepo) ListAppointmentsByTrainer(ctx context.Context, trainerID uuid.UUID, from, to *time.Time) ([]domain.PTAppointment, error) {
	var list []domain.PTAppointment
	for _, a := range m.appointments {
		if a.TrainerID == trainerID {
			list = append(list, *a)
		}
	}
	return list, nil
}

func (m *mockPTRepo) HasTrainerConflictTx(ctx context.Context, tx pgx.Tx, trainerID uuid.UUID, startTime, endTime time.Time, excludeID *uuid.UUID) (bool, error) {
	for _, a := range m.appointments {
		if a.TrainerID == trainerID && a.Status == domain.PTStatusConfirmed {
			if excludeID != nil && a.ID == *excludeID {
				continue
			}
			if a.StartTime.Before(endTime) && a.EndTime.After(startTime) {
				return true, nil
			}
		}
	}
	return false, nil
}

type mockUserRepoForPT struct {
	users map[uuid.UUID]*domain.User
}

func (m *mockUserRepoForPT) Create(ctx context.Context, u *domain.User) error { return nil }
func (m *mockUserRepoForPT) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}
func (m *mockUserRepoForPT) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForPT) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	return nil
}
func (m *mockUserRepoForPT) IsRefreshTokenValid(ctx context.Context, tokenHash string) (bool, uuid.UUID, error) {
	return true, uuid.Nil, nil
}
func (m *mockUserRepoForPT) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return nil
}

type mockAuditRepoForPT struct{}

func (m *mockAuditRepoForPT) Append(ctx context.Context, log *domain.AuditLog) error { return nil }
func (m *mockAuditRepoForPT) List(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	return nil, nil
}

func TestPTHandler_PurchaseAndBook(t *testing.T) {
	ptRepo := newMockPTRepo()
	memberID := uuid.New()
	trainerID := uuid.New()
	branchID := uuid.New()

	userRepo := &mockUserRepoForPT{
		users: map[uuid.UUID]*domain.User{
			trainerID: {
				ID:        trainerID,
				Email:     "trainer@fitgiris.com",
				FirstName: "Murat",
				LastName:  "Eğitmen",
				Role:      domain.RoleTrainer,
				IsActive:  true,
			},
		},
	}
	auditRepo := &mockAuditRepoForPT{}

	svc := service.NewPTService(ptRepo, userRepo, auditRepo)
	ptHandler := handler.NewPTHandler(svc)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), middleware.UserClaimsContextKey, &security.Claims{
				UserID: memberID,
				Email:  "member@fitgiris.com",
				Role:   domain.RoleMember,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/pt/packages", ptHandler.PurchasePackage)
	r.Get("/pt/packages", ptHandler.ListPackages)
	r.Post("/pt/appointments", ptHandler.BookAppointment)
	r.Get("/pt/appointments", ptHandler.ListAppointments)
	r.Post("/pt/appointments/{id}/cancel", ptHandler.CancelAppointment)

	// 1. Purchase Package
	body, _ := json.Marshal(handler.PurchasePTPackageRequest{
		TrainerID:     trainerID,
		TotalSessions: 5,
		PriceKurus:    500000,
	})
	req := httptest.NewRequest("POST", "/pt/packages", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for PT purchase, got %d: %s", rec.Code, rec.Body.String())
	}

	var env struct {
		Data domain.PTPackage `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	purchasedPkg := env.Data
	if purchasedPkg.TotalSessions != 5 || purchasedPkg.RemainingSessions != 5 {
		t.Fatalf("unexpected package sessions: %+v", purchasedPkg)
	}

	// 2. Book Appointment
	now := time.Now().Add(24 * time.Hour)
	bookBody, _ := json.Marshal(handler.BookPTAppointmentRequest{
		PTPackageID: purchasedPkg.ID,
		BranchID:    branchID,
		StartTime:   now,
		EndTime:     now.Add(1 * time.Hour),
		Notes:       "Omuz ve sırt antrenmanı",
	})
	req = httptest.NewRequest("POST", "/pt/appointments", bytes.NewBuffer(bookBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for PT booking, got %d: %s", rec.Code, rec.Body.String())
	}

	var appEnv struct {
		Data domain.PTAppointment `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &appEnv)
	app := appEnv.Data

	// Verify remaining sessions decremented
	if ptRepo.packages[purchasedPkg.ID].RemainingSessions != 4 {
		t.Fatalf("expected 4 remaining sessions, got %d", ptRepo.packages[purchasedPkg.ID].RemainingSessions)
	}

	// 3. Try booking conflicting time
	conflictBody, _ := json.Marshal(handler.BookPTAppointmentRequest{
		PTPackageID: purchasedPkg.ID,
		BranchID:    branchID,
		StartTime:   now.Add(30 * time.Minute),
		EndTime:     now.Add(90 * time.Minute),
		Notes:       "Conflict test",
	})
	req = httptest.NewRequest("POST", "/pt/appointments", bytes.NewBuffer(conflictBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for overlapping appointment, got %d", rec.Code)
	}

	// 4. Cancel appointment & verify session refund
	req = httptest.NewRequest("POST", "/pt/appointments/"+app.ID.String()+"/cancel", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for cancellation, got %d: %s", rec.Code, rec.Body.String())
	}

	if ptRepo.packages[purchasedPkg.ID].RemainingSessions != 5 {
		t.Fatalf("expected 5 sessions after cancellation refund, got %d", ptRepo.packages[purchasedPkg.ID].RemainingSessions)
	}
}
