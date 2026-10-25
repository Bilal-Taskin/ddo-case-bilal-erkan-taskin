package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
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

// In-memory mock booking repository for handler testing
type mockHandlerBookingRepo struct {
	mu             sync.Mutex
	classes        map[uuid.UUID]*domain.Class
	schedules      map[uuid.UUID]*domain.ClassSchedule
	bookings       map[uuid.UUID]*domain.ClassBooking
	waitlist       map[uuid.UUID]*domain.ClassWaitlist
	memberPackages map[uuid.UUID]*domain.MemberPackage
}

func newMockHandlerBookingRepo() *mockHandlerBookingRepo {
	return &mockHandlerBookingRepo{
		classes:        make(map[uuid.UUID]*domain.Class),
		schedules:      make(map[uuid.UUID]*domain.ClassSchedule),
		bookings:       make(map[uuid.UUID]*domain.ClassBooking),
		waitlist:       make(map[uuid.UUID]*domain.ClassWaitlist),
		memberPackages: make(map[uuid.UUID]*domain.MemberPackage),
	}
}

func (m *mockHandlerBookingRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(nil)
}

func (m *mockHandlerBookingRepo) CreateClass(ctx context.Context, c *domain.Class) error {
	m.classes[c.ID] = c
	return nil
}

func (m *mockHandlerBookingRepo) GetClassByID(ctx context.Context, id uuid.UUID) (*domain.Class, error) {
	c, ok := m.classes[id]
	if !ok {
		return nil, domain.ErrClassNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *mockHandlerBookingRepo) ListClasses(ctx context.Context, activeOnly bool) ([]domain.Class, error) {
	var list []domain.Class
	for _, c := range m.classes {
		if !activeOnly || c.IsActive {
			list = append(list, *c)
		}
	}
	return list, nil
}

func (m *mockHandlerBookingRepo) CreateSchedule(ctx context.Context, s *domain.ClassSchedule) error {
	m.schedules[s.ID] = s
	return nil
}

func (m *mockHandlerBookingRepo) GetScheduleByID(ctx context.Context, id uuid.UUID) (*domain.ClassSchedule, error) {
	s, ok := m.schedules[id]
	if !ok {
		return nil, domain.ErrScheduleNotFound
	}
	cp := *s
	return &cp, nil
}

func (m *mockHandlerBookingRepo) GetScheduleForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassSchedule, error) {
	s, ok := m.schedules[id]
	if !ok {
		return nil, domain.ErrScheduleNotFound
	}
	return s, nil
}

func (m *mockHandlerBookingRepo) UpdateScheduleBookedCountTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, bookedCount int) error {
	s, ok := m.schedules[id]
	if !ok {
		return domain.ErrScheduleNotFound
	}
	s.BookedCount = bookedCount
	return nil
}

func (m *mockHandlerBookingRepo) UpdateScheduleStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleStatus) error {
	s, ok := m.schedules[id]
	if !ok {
		return domain.ErrScheduleNotFound
	}
	s.Status = status
	return nil
}

func (m *mockHandlerBookingRepo) ListSchedules(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID, from, to *time.Time) ([]domain.ClassSchedule, error) {
	var list []domain.ClassSchedule
	for _, s := range m.schedules {
		if branchID != nil && s.BranchID != *branchID {
			continue
		}
		if trainerID != nil && s.TrainerID != *trainerID {
			continue
		}
		list = append(list, *s)
	}
	return list, nil
}

func (m *mockHandlerBookingRepo) CreateOrUpdateBookingTx(ctx context.Context, tx pgx.Tx, b *domain.ClassBooking) error {
	for _, existing := range m.bookings {
		if existing.ScheduleID == b.ScheduleID && existing.MemberID == b.MemberID {
			existing.Status = b.Status
			existing.CancelledAt = b.CancelledAt
			existing.PenaltyApplied = b.PenaltyApplied
			b.ID = existing.ID
			return nil
		}
	}
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	m.bookings[b.ID] = b
	return nil
}

func (m *mockHandlerBookingRepo) GetBookingByID(ctx context.Context, id uuid.UUID) (*domain.ClassBooking, error) {
	b, ok := m.bookings[id]
	if !ok {
		return nil, domain.ErrBookingNotFound
	}
	cp := *b
	return &cp, nil
}

func (m *mockHandlerBookingRepo) GetBookingForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassBooking, error) {
	b, ok := m.bookings[id]
	if !ok {
		return nil, domain.ErrBookingNotFound
	}
	return b, nil
}

func (m *mockHandlerBookingRepo) GetBookingByMemberAndSchedule(ctx context.Context, scheduleID, memberID uuid.UUID) (*domain.ClassBooking, error) {
	for _, b := range m.bookings {
		if b.ScheduleID == scheduleID && b.MemberID == memberID {
			cp := *b
			return &cp, nil
		}
	}
	return nil, domain.ErrBookingNotFound
}

func (m *mockHandlerBookingRepo) GetBookingByMemberAndScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (*domain.ClassBooking, error) {
	for _, b := range m.bookings {
		if b.ScheduleID == scheduleID && b.MemberID == memberID {
			return b, nil
		}
	}
	return nil, domain.ErrBookingNotFound
}

func (m *mockHandlerBookingRepo) ListBookingsByMember(ctx context.Context, memberID uuid.UUID, activeOnly bool) ([]domain.ClassBooking, error) {
	var list []domain.ClassBooking
	for _, b := range m.bookings {
		if b.MemberID == memberID {
			if !activeOnly || b.Status == domain.BookingStatusConfirmed {
				list = append(list, *b)
			}
		}
	}
	return list, nil
}

func (m *mockHandlerBookingRepo) ListBookingsBySchedule(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassBooking, error) {
	var list []domain.ClassBooking
	for _, b := range m.bookings {
		if b.ScheduleID == scheduleID {
			list = append(list, *b)
		}
	}
	return list, nil
}

func (m *mockHandlerBookingRepo) UpdateBookingTx(ctx context.Context, tx pgx.Tx, b *domain.ClassBooking) error {
	m.bookings[b.ID] = b
	return nil
}

func (m *mockHandlerBookingRepo) HasTimeConflictTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID, startTime, endTime time.Time, excludeScheduleID *uuid.UUID) (bool, error) {
	for _, b := range m.bookings {
		if b.MemberID == memberID && b.Status == domain.BookingStatusConfirmed {
			s, ok := m.schedules[b.ScheduleID]
			if ok && s.Status == domain.ScheduleStatusScheduled {
				if excludeScheduleID != nil && s.ID == *excludeScheduleID {
					continue
				}
				if s.StartTime.Before(endTime) && s.EndTime.After(startTime) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (m *mockHandlerBookingRepo) CreateOrUpdateWaitlistTx(ctx context.Context, tx pgx.Tx, w *domain.ClassWaitlist) error {
	for _, existing := range m.waitlist {
		if existing.ScheduleID == w.ScheduleID && existing.MemberID == w.MemberID {
			existing.Status = w.Status
			existing.CreatedAt = w.CreatedAt
			w.ID = existing.ID
			return nil
		}
	}
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	m.waitlist[w.ID] = w
	return nil
}

func (m *mockHandlerBookingRepo) GetWaitlistByID(ctx context.Context, id uuid.UUID) (*domain.ClassWaitlist, error) {
	w, ok := m.waitlist[id]
	if !ok {
		return nil, domain.ErrWaitlistNotFound
	}
	cp := *w
	return &cp, nil
}

func (m *mockHandlerBookingRepo) GetWaitlistForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassWaitlist, error) {
	w, ok := m.waitlist[id]
	if !ok {
		return nil, domain.ErrWaitlistNotFound
	}
	return w, nil
}

func (m *mockHandlerBookingRepo) GetWaitlistByMemberAndScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (*domain.ClassWaitlist, error) {
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.MemberID == memberID {
			return w, nil
		}
	}
	return nil, domain.ErrWaitlistNotFound
}

func (m *mockHandlerBookingRepo) GetNextWaitingForUpdateSkipLockedTx(ctx context.Context, tx pgx.Tx, scheduleID uuid.UUID) (*domain.ClassWaitlist, error) {
	var waiting []*domain.ClassWaitlist
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.Status == domain.WaitlistStatusWaiting {
			waiting = append(waiting, w)
		}
	}
	if len(waiting) == 0 {
		return nil, nil
	}
	sort.Slice(waiting, func(i, j int) bool {
		return waiting[i].CreatedAt.Before(waiting[j].CreatedAt)
	})
	return waiting[0], nil
}

func (m *mockHandlerBookingRepo) UpdateWaitlistTx(ctx context.Context, tx pgx.Tx, w *domain.ClassWaitlist) error {
	m.waitlist[w.ID] = w
	return nil
}

func (m *mockHandlerBookingRepo) GetWaitlistPositionTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (int, error) {
	count := 1
	var targetCreatedAt time.Time
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.MemberID == memberID {
			targetCreatedAt = w.CreatedAt
			break
		}
	}
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.Status == domain.WaitlistStatusWaiting && w.CreatedAt.Before(targetCreatedAt) {
			count++
		}
	}
	return count, nil
}

func (m *mockHandlerBookingRepo) ListWaitlistBySchedule(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassWaitlist, error) {
	var list []domain.ClassWaitlist
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID {
			list = append(list, *w)
		}
	}
	return list, nil
}

func (m *mockHandlerBookingRepo) ListWaitlistByMember(ctx context.Context, memberID uuid.UUID) ([]domain.ClassWaitlist, error) {
	var list []domain.ClassWaitlist
	for _, w := range m.waitlist {
		if w.MemberID == memberID {
			list = append(list, *w)
		}
	}
	return list, nil
}

func (m *mockHandlerBookingRepo) GetWaitlistCount(ctx context.Context, scheduleID uuid.UUID) (int, error) {
	count := 0
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.Status == domain.WaitlistStatusWaiting {
			count++
		}
	}
	return count, nil
}

func (m *mockHandlerBookingRepo) GetActiveMemberPackageTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID) (*domain.MemberPackage, error) {
	pkg, ok := m.memberPackages[memberID]
	if !ok || pkg.Status != domain.StatusActive {
		return nil, domain.ErrNoActiveMembership
	}
	return pkg, nil
}

func (m *mockHandlerBookingRepo) DeductSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	for _, p := range m.memberPackages {
		if p.ID == packageID {
			if p.RemainingEntries <= 0 {
				return domain.ErrInsufficientEntries
			}
			p.RemainingEntries--
			return nil
		}
	}
	return nil
}

func (m *mockHandlerBookingRepo) RefundSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	for _, p := range m.memberPackages {
		if p.ID == packageID {
			p.RemainingEntries++
			return nil
		}
	}
	return nil
}

// Router Setup Helper
func setupTestBookingRouter() (*chi.Mux, *mockHandlerBookingRepo, *mockUserRepoForHandler, *security.JWTManager) {
	bookingRepo := newMockHandlerBookingRepo()
	userRepo := &mockUserRepoForHandler{users: make(map[uuid.UUID]*domain.User)}
	auditRepo := &mockAuditRepoForHandler{}
	jwtManager := security.NewJWTManager("test-secret-key-32-bytes-minimum-length-fitgiris!")

	svc := service.NewBookingService(bookingRepo, userRepo, auditRepo)
	h := handler.NewBookingHandler(svc)
	authM := middleware.NewAuthMiddleware(jwtManager)

	r := chi.NewRouter()

	r.Route("/api/v1/classes", func(r chi.Router) {
		r.Use(authM.Authenticate)

		r.Get("/", h.ListClasses)
		r.Get("/schedules", h.ListSchedules)
		r.Get("/schedules/{id}", h.GetSchedule)

		r.With(middleware.RequireRoles(domain.RoleMember)).Get("/my-bookings", h.ListMyBookings)
		r.With(middleware.RequireRoles(domain.RoleMember)).Get("/my-waitlist", h.ListMyWaitlist)
		r.With(middleware.RequireRoles(domain.RoleMember)).Post("/schedules/{id}/book", h.BookClass)
		r.With(middleware.RequireRoles(domain.RoleMember)).Post("/bookings/{id}/cancel", h.CancelBooking)
		r.With(middleware.RequireRoles(domain.RoleMember)).Post("/waitlist/{id}/cancel", h.CancelWaitlist)

		r.With(middleware.RequireRoles(domain.RoleManager)).Post("/", h.CreateClass)
		r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleTrainer)).Post("/schedules", h.CreateSchedule)
		r.With(middleware.RequireRoles(domain.RoleTrainer, domain.RoleReception, domain.RoleManager)).Post("/schedules/{id}/attendance", h.MarkAttendance)
	})

	return r, bookingRepo, userRepo, jwtManager
}

func TestBookingHandler_CreateClassRBAC(t *testing.T) {
	r, _, userRepo, jwtManager := setupTestBookingRouter()
	ctx := context.Background()

	// Member user
	member := &domain.User{ID: uuid.New(), Email: "member@fitgiris.com", Role: domain.RoleMember, IsActive: true}
	_ = userRepo.Create(ctx, member)
	memberTokens, _ := jwtManager.GenerateTokenPair(member)

	// Manager user
	manager := &domain.User{ID: uuid.New(), Email: "mgr@fitgiris.com", Role: domain.RoleManager, IsActive: true}
	_ = userRepo.Create(ctx, manager)
	managerTokens, _ := jwtManager.GenerateTokenPair(manager)

	body := []byte(`{"name":"Pilates Core","duration_minutes":50,"default_capacity":12}`)

	// Member attempts to create class -> 403 Forbidden
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/classes", bytes.NewBuffer(body))
	req1.Header.Set("Authorization", "Bearer "+memberTokens.AccessToken)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for member creating class, got %d", w1.Code)
	}

	// Manager attempts to create class -> 201 Created
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/classes", bytes.NewBuffer(body))
	req2.Header.Set("Authorization", "Bearer "+managerTokens.AccessToken)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Errorf("expected 201 Created for manager creating class, got %d", w2.Code)
	}
}

func TestBookingHandler_BookAndCancelClassFlow(t *testing.T) {
	r, repo, userRepo, jwtManager := setupTestBookingRouter()
	ctx := context.Background()

	branchID := uuid.New()
	trainer := &domain.User{ID: uuid.New(), Email: "trainer@fitgiris.com", Role: domain.RoleTrainer, IsActive: true}
	_ = userRepo.Create(ctx, trainer)

	member := &domain.User{ID: uuid.New(), Email: "uye@fitgiris.com", Role: domain.RoleMember, IsActive: true}
	_ = userRepo.Create(ctx, member)
	tokens, _ := jwtManager.GenerateTokenPair(member)

	// Assign active membership to member
	repo.memberPackages[member.ID] = &domain.MemberPackage{
		ID:          uuid.New(),
		MemberID:    member.ID,
		BranchID:    branchID,
		AllBranches: true,
		Status:      domain.StatusActive,
		PackageKind: domain.PackageKindDuration,
	}

	class := &domain.Class{ID: uuid.New(), Name: "Zumba", DurationMinutes: 50, DefaultCapacity: 1, IsActive: true}
	_ = repo.CreateClass(ctx, class)

	// Schedule with capacity 1
	schedule := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class.ID,
		BranchID:    branchID,
		TrainerID:   trainer.ID,
		StartTime:   time.Now().UTC().Add(4 * time.Hour),
		EndTime:     time.Now().UTC().Add(5 * time.Hour),
		Capacity:    1,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedule)

	// 1. Book the class -> 201 Created
	bookReq := httptest.NewRequest(http.MethodPost, "/api/v1/classes/schedules/"+schedule.ID.String()+"/book", nil)
	bookReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	wBook := httptest.NewRecorder()
	r.ServeHTTP(wBook, bookReq)
	if wBook.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", wBook.Code, wBook.Body.String())
	}

	var bookRes domain.BookingResult
	var respEnvelope struct {
		Data domain.BookingResult `json:"data"`
	}
	_ = json.Unmarshal(wBook.Body.Bytes(), &respEnvelope)
	bookRes = respEnvelope.Data
	if bookRes.IsWaitlisted || bookRes.Booking == nil {
		t.Fatalf("expected confirmed booking, got: %+v", bookRes)
	}

	// 2. Try booking again -> 409 Conflict (ERR_ALREADY_BOOKED)
	reBookReq := httptest.NewRequest(http.MethodPost, "/api/v1/classes/schedules/"+schedule.ID.String()+"/book", nil)
	reBookReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	wReBook := httptest.NewRecorder()
	r.ServeHTTP(wReBook, reBookReq)
	if wReBook.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for double booking, got %d", wReBook.Code)
	}

	// 3. Cancel the booking -> 200 OK
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/classes/bookings/"+bookRes.Booking.ID.String()+"/cancel", nil)
	cancelReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	wCancel := httptest.NewRecorder()
	r.ServeHTTP(wCancel, cancelReq)
	if wCancel.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for cancel, got %d: %s", wCancel.Code, wCancel.Body.String())
	}

	// 4. Try cancelling already cancelled booking -> 409 Conflict
	reCancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/classes/bookings/"+bookRes.Booking.ID.String()+"/cancel", nil)
	reCancelReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	wReCancel := httptest.NewRecorder()
	r.ServeHTTP(wReCancel, reCancelReq)
	if wReCancel.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for cancelling already cancelled booking, got %d", wReCancel.Code)
	}
}
