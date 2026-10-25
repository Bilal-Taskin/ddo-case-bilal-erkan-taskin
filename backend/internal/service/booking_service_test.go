package service_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fitgiris/internal/domain"
	"fitgiris/internal/service"
)

// mockBookingRepo simulates PostgresBookingRepository in memory with thread safety
type mockBookingRepo struct {
	mu             sync.Mutex
	classes        map[uuid.UUID]*domain.Class
	schedules      map[uuid.UUID]*domain.ClassSchedule
	bookings       map[uuid.UUID]*domain.ClassBooking
	waitlist       map[uuid.UUID]*domain.ClassWaitlist
	memberPackages map[uuid.UUID]*domain.MemberPackage
}

func newMockBookingRepo() *mockBookingRepo {
	return &mockBookingRepo{
		classes:        make(map[uuid.UUID]*domain.Class),
		schedules:      make(map[uuid.UUID]*domain.ClassSchedule),
		bookings:       make(map[uuid.UUID]*domain.ClassBooking),
		waitlist:       make(map[uuid.UUID]*domain.ClassWaitlist),
		memberPackages: make(map[uuid.UUID]*domain.MemberPackage),
	}
}

func (m *mockBookingRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(nil)
}

// Class operations
func (m *mockBookingRepo) CreateClass(ctx context.Context, c *domain.Class) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.classes[c.ID] = c
	return nil
}

func (m *mockBookingRepo) GetClassByID(ctx context.Context, id uuid.UUID) (*domain.Class, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.classes[id]
	if !ok {
		return nil, domain.ErrClassNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *mockBookingRepo) ListClasses(ctx context.Context, activeOnly bool) ([]domain.Class, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.Class
	for _, c := range m.classes {
		if !activeOnly || c.IsActive {
			list = append(list, *c)
		}
	}
	return list, nil
}

// Schedule operations
func (m *mockBookingRepo) CreateSchedule(ctx context.Context, s *domain.ClassSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.schedules[s.ID] = s
	return nil
}

func (m *mockBookingRepo) GetScheduleByID(ctx context.Context, id uuid.UUID) (*domain.ClassSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.schedules[id]
	if !ok {
		return nil, domain.ErrScheduleNotFound
	}
	cp := *s
	return &cp, nil
}

func (m *mockBookingRepo) GetScheduleForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassSchedule, error) {
	// Already guarded by WithTransaction lock
	s, ok := m.schedules[id]
	if !ok {
		return nil, domain.ErrScheduleNotFound
	}
	return s, nil
}

func (m *mockBookingRepo) UpdateScheduleBookedCountTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, bookedCount int) error {
	s, ok := m.schedules[id]
	if !ok {
		return domain.ErrScheduleNotFound
	}
	s.BookedCount = bookedCount
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockBookingRepo) UpdateScheduleStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.schedules[id]
	if !ok {
		return domain.ErrScheduleNotFound
	}
	s.Status = status
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockBookingRepo) ListSchedules(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID, from, to *time.Time) ([]domain.ClassSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.ClassSchedule
	for _, s := range m.schedules {
		if branchID != nil && s.BranchID != *branchID {
			continue
		}
		if trainerID != nil && s.TrainerID != *trainerID {
			continue
		}
		if from != nil && s.StartTime.Before(*from) {
			continue
		}
		if to != nil && s.StartTime.After(*to) {
			continue
		}
		list = append(list, *s)
	}
	return list, nil
}

// Booking operations
func (m *mockBookingRepo) CreateOrUpdateBookingTx(ctx context.Context, tx pgx.Tx, b *domain.ClassBooking) error {
	for _, existing := range m.bookings {
		if existing.ScheduleID == b.ScheduleID && existing.MemberID == b.MemberID {
			existing.Status = b.Status
			existing.CancelledAt = b.CancelledAt
			existing.PenaltyApplied = b.PenaltyApplied
			existing.UpdatedAt = time.Now().UTC()
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

func (m *mockBookingRepo) GetBookingByID(ctx context.Context, id uuid.UUID) (*domain.ClassBooking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bookings[id]
	if !ok {
		return nil, domain.ErrBookingNotFound
	}
	cp := *b
	return &cp, nil
}

func (m *mockBookingRepo) GetBookingForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassBooking, error) {
	b, ok := m.bookings[id]
	if !ok {
		return nil, domain.ErrBookingNotFound
	}
	return b, nil
}

func (m *mockBookingRepo) GetBookingByMemberAndSchedule(ctx context.Context, scheduleID, memberID uuid.UUID) (*domain.ClassBooking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.bookings {
		if b.ScheduleID == scheduleID && b.MemberID == memberID {
			cp := *b
			return &cp, nil
		}
	}
	return nil, domain.ErrBookingNotFound
}

func (m *mockBookingRepo) GetBookingByMemberAndScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (*domain.ClassBooking, error) {
	for _, b := range m.bookings {
		if b.ScheduleID == scheduleID && b.MemberID == memberID {
			return b, nil
		}
	}
	return nil, domain.ErrBookingNotFound
}

func (m *mockBookingRepo) ListBookingsByMember(ctx context.Context, memberID uuid.UUID, activeOnly bool) ([]domain.ClassBooking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *mockBookingRepo) ListBookingsBySchedule(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassBooking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.ClassBooking
	for _, b := range m.bookings {
		if b.ScheduleID == scheduleID {
			list = append(list, *b)
		}
	}
	return list, nil
}

func (m *mockBookingRepo) UpdateBookingTx(ctx context.Context, tx pgx.Tx, b *domain.ClassBooking) error {
	m.bookings[b.ID] = b
	return nil
}

func (m *mockBookingRepo) HasTimeConflictTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID, startTime, endTime time.Time, excludeScheduleID *uuid.UUID) (bool, error) {
	for _, b := range m.bookings {
		if b.MemberID == memberID && b.Status == domain.BookingStatusConfirmed {
			s, ok := m.schedules[b.ScheduleID]
			if ok && s.Status == domain.ScheduleStatusScheduled {
				if excludeScheduleID != nil && s.ID == *excludeScheduleID {
					continue
				}
				// Overlap condition: start < otherEnd AND end > otherStart
				if s.StartTime.Before(endTime) && s.EndTime.After(startTime) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// Waitlist operations
func (m *mockBookingRepo) CreateOrUpdateWaitlistTx(ctx context.Context, tx pgx.Tx, w *domain.ClassWaitlist) error {
	for _, existing := range m.waitlist {
		if existing.ScheduleID == w.ScheduleID && existing.MemberID == w.MemberID {
			existing.Status = w.Status
			existing.CreatedAt = w.CreatedAt
			existing.PromotedAt = w.PromotedAt
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

func (m *mockBookingRepo) GetWaitlistByID(ctx context.Context, id uuid.UUID) (*domain.ClassWaitlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.waitlist[id]
	if !ok {
		return nil, domain.ErrWaitlistNotFound
	}
	cp := *w
	return &cp, nil
}

func (m *mockBookingRepo) GetWaitlistForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassWaitlist, error) {
	w, ok := m.waitlist[id]
	if !ok {
		return nil, domain.ErrWaitlistNotFound
	}
	return w, nil
}

func (m *mockBookingRepo) GetWaitlistByMemberAndScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (*domain.ClassWaitlist, error) {
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.MemberID == memberID {
			return w, nil
		}
	}
	return nil, domain.ErrWaitlistNotFound
}

func (m *mockBookingRepo) GetNextWaitingForUpdateSkipLockedTx(ctx context.Context, tx pgx.Tx, scheduleID uuid.UUID) (*domain.ClassWaitlist, error) {
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

func (m *mockBookingRepo) UpdateWaitlistTx(ctx context.Context, tx pgx.Tx, w *domain.ClassWaitlist) error {
	m.waitlist[w.ID] = w
	return nil
}

func (m *mockBookingRepo) GetWaitlistPositionTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (int, error) {
	var targetTime time.Time
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.MemberID == memberID {
			targetTime = w.CreatedAt
			break
		}
	}
	if targetTime.IsZero() {
		return 0, domain.ErrWaitlistNotFound
	}
	count := 1
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.Status == domain.WaitlistStatusWaiting && w.CreatedAt.Before(targetTime) {
			count++
		}
	}
	return count, nil
}

func (m *mockBookingRepo) ListWaitlistBySchedule(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassWaitlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.ClassWaitlist
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID {
			list = append(list, *w)
		}
	}
	return list, nil
}

func (m *mockBookingRepo) ListWaitlistByMember(ctx context.Context, memberID uuid.UUID) ([]domain.ClassWaitlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.ClassWaitlist
	for _, w := range m.waitlist {
		if w.MemberID == memberID {
			list = append(list, *w)
		}
	}
	return list, nil
}

func (m *mockBookingRepo) GetWaitlistCount(ctx context.Context, scheduleID uuid.UUID) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, w := range m.waitlist {
		if w.ScheduleID == scheduleID && w.Status == domain.WaitlistStatusWaiting {
			count++
		}
	}
	return count, nil
}

// Membership / Session Helpers in Tx
func (m *mockBookingRepo) GetActiveMemberPackageTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID) (*domain.MemberPackage, error) {
	pkg, ok := m.memberPackages[memberID]
	if !ok || pkg.Status != domain.StatusActive {
		return nil, domain.ErrNoActiveMembership
	}
	return pkg, nil
}

func (m *mockBookingRepo) DeductSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	for _, pkg := range m.memberPackages {
		if pkg.ID == packageID {
			if pkg.RemainingEntries <= 0 {
				return domain.ErrInsufficientEntries
			}
			pkg.RemainingEntries--
			return nil
		}
	}
	return errors.New("paket bulunamadi")
}

func (m *mockBookingRepo) RefundSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	for _, pkg := range m.memberPackages {
		if pkg.ID == packageID {
			pkg.RemainingEntries++
			return nil
		}
	}
	return errors.New("paket bulunamadi")
}

// Helper to set up mock environment
func setupBookingService() (*service.DefaultBookingService, *mockBookingRepo, *mockUserRepo, *mockAuditRepo) {
	bookingRepo := newMockBookingRepo()
	userRepo := newMockUserRepo()
	auditRepo := newMockAuditRepo()
	svc := service.NewBookingService(bookingRepo, userRepo, auditRepo)
	return svc, bookingRepo, userRepo, auditRepo
}

// ==========================================
// Test Cases
// ==========================================

func TestBookingService_ClassAndScheduleCreation(t *testing.T) {
	svc, _, userRepo, _ := setupBookingService()
	ctx := context.Background()

	// 1. Create Trainer user
	trainer := &domain.User{
		ID:        uuid.New(),
		Email:     "trainer@fitgiris.com",
		FirstName: "Ahmet",
		LastName:  "Hoca",
		Role:      domain.RoleTrainer,
		IsActive:  true,
	}
	_ = userRepo.Create(ctx, trainer)

	branchID := uuid.New()
	managerID := uuid.New()

	// 2. Create Class
	class, err := svc.CreateClass(ctx, service.CreateClassRequest{
		Name:            "CrossFit Pro",
		DurationMinutes: 60,
		DefaultCapacity: 15,
	}, managerID)
	if err != nil {
		t.Fatalf("CreateClass failed: %v", err)
	}
	if class.Name != "CrossFit Pro" {
		t.Errorf("expected class name CrossFit Pro, got %s", class.Name)
	}

	// 3. Create Schedule in the future
	startTime := time.Now().UTC().Add(24 * time.Hour)
	endTime := startTime.Add(60 * time.Minute)

	schedule, err := svc.CreateSchedule(ctx, service.CreateScheduleRequest{
		ClassID:   class.ID,
		BranchID:  branchID,
		TrainerID: trainer.ID,
		StartTime: startTime,
		EndTime:   endTime,
		Capacity:  10,
	}, managerID)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	if schedule.Capacity != 10 || schedule.BookedCount != 0 {
		t.Errorf("unexpected schedule state: %+v", schedule)
	}

	// 4. Past schedule creation should fail
	_, err = svc.CreateSchedule(ctx, service.CreateScheduleRequest{
		ClassID:   class.ID,
		BranchID:  branchID,
		TrainerID: trainer.ID,
		StartTime: time.Now().UTC().Add(-1 * time.Hour),
		EndTime:   time.Now().UTC(),
		Capacity:  10,
	}, managerID)
	if err == nil {
		t.Error("expected error for past schedule, got nil")
	}
}

func TestBookingService_BookingRulesAndMembershipChecks(t *testing.T) {
	svc, repo, _, _ := setupBookingService()
	ctx := context.Background()

	branch1 := uuid.New()
	branch2 := uuid.New()
	trainerID := uuid.New()

	class := &domain.Class{
		ID:              uuid.New(),
		Name:            "Spinning Extreme",
		DurationMinutes: 45,
		DefaultCapacity: 20,
		IsActive:        true,
	}
	_ = repo.CreateClass(ctx, class)

	startTime := time.Now().UTC().Add(48 * time.Hour)
	endTime := startTime.Add(45 * time.Minute)

	schedule := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class.ID,
		BranchID:    branch1,
		TrainerID:   trainerID,
		StartTime:   startTime,
		EndTime:     endTime,
		Capacity:    2,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedule)

	memberNoPackage := uuid.New()
	memberBranchMismatch := uuid.New()
	memberNoSessionBalance := uuid.New()
	validMember := uuid.New()

	// Member without package
	_, err := svc.BookClass(ctx, memberNoPackage, schedule.ID)
	if !errors.Is(err, domain.ErrNoActiveMembership) {
		t.Errorf("expected ErrNoActiveMembership, got %v", err)
	}

	// Member with single branch access for branch2 trying to book branch1
	repo.memberPackages[memberBranchMismatch] = &domain.MemberPackage{
		ID:          uuid.New(),
		MemberID:    memberBranchMismatch,
		BranchID:    branch2,
		AllBranches: false,
		Status:      domain.StatusActive,
		PackageKind: domain.PackageKindDuration,
	}
	_, err = svc.BookClass(ctx, memberBranchMismatch, schedule.ID)
	if !errors.Is(err, domain.ErrBranchAccessDenied) {
		t.Errorf("expected ErrBranchAccessDenied, got %v", err)
	}

	// Member with session package but 0 entries left
	repo.memberPackages[memberNoSessionBalance] = &domain.MemberPackage{
		ID:               uuid.New(),
		MemberID:         memberNoSessionBalance,
		BranchID:         branch1,
		AllBranches:      true,
		Status:           domain.StatusActive,
		PackageKind:      domain.PackageKindSession,
		RemainingEntries: 0,
	}
	_, err = svc.BookClass(ctx, memberNoSessionBalance, schedule.ID)
	if !errors.Is(err, domain.ErrInsufficientEntries) {
		t.Errorf("expected ErrInsufficientEntries, got %v", err)
	}

	// Valid Member with Session Package (3 remaining entries)
	validPkg := &domain.MemberPackage{
		ID:               uuid.New(),
		MemberID:         validMember,
		BranchID:         branch1,
		AllBranches:      true,
		Status:           domain.StatusActive,
		PackageKind:      domain.PackageKindSession,
		RemainingEntries: 3,
	}
	repo.memberPackages[validMember] = validPkg

	result, err := svc.BookClass(ctx, validMember, schedule.ID)
	if err != nil {
		t.Fatalf("valid booking failed: %v", err)
	}
	if result.IsWaitlisted {
		t.Error("expected confirmed booking, got waitlisted")
	}
	if validPkg.RemainingEntries != 2 {
		t.Errorf("expected remaining entries to be decremented to 2, got %d", validPkg.RemainingEntries)
	}

	// Double booking attempt should fail
	_, err = svc.BookClass(ctx, validMember, schedule.ID)
	if !errors.Is(err, domain.ErrClassAlreadyBooked) {
		t.Errorf("expected ErrClassAlreadyBooked, got %v", err)
	}
}

func TestBookingService_TimeConflictCheck(t *testing.T) {
	svc, repo, _, _ := setupBookingService()
	ctx := context.Background()

	branchID := uuid.New()
	trainerID := uuid.New()
	memberID := uuid.New()

	repo.memberPackages[memberID] = &domain.MemberPackage{
		ID:          uuid.New(),
		MemberID:    memberID,
		BranchID:    branchID,
		AllBranches: true,
		Status:      domain.StatusActive,
		PackageKind: domain.PackageKindDuration,
	}

	class1 := &domain.Class{ID: uuid.New(), Name: "Pilates", DurationMinutes: 60, IsActive: true}
	class2 := &domain.Class{ID: uuid.New(), Name: "Boxing", DurationMinutes: 60, IsActive: true}
	_ = repo.CreateClass(ctx, class1)
	_ = repo.CreateClass(ctx, class2)

	// Schedule 1: 14:00 - 15:00
	baseTime := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Hour)
	schedule1 := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class1.ID,
		BranchID:    branchID,
		TrainerID:   trainerID,
		StartTime:   baseTime,
		EndTime:     baseTime.Add(60 * time.Minute),
		Capacity:    10,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedule1)

	// Schedule 2: 14:30 - 15:30 (Overlaps with Schedule 1 by 30 mins)
	schedule2 := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class2.ID,
		BranchID:    branchID,
		TrainerID:   trainerID,
		StartTime:   baseTime.Add(30 * time.Minute),
		EndTime:     baseTime.Add(90 * time.Minute),
		Capacity:    10,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedule2)

	// Book Schedule 1
	_, err := svc.BookClass(ctx, memberID, schedule1.ID)
	if err != nil {
		t.Fatalf("booking schedule1 failed: %v", err)
	}

	// Try booking overlapping Schedule 2 -> should fail with ErrScheduleTimeConflict
	_, err = svc.BookClass(ctx, memberID, schedule2.ID)
	if !errors.Is(err, domain.ErrScheduleTimeConflict) {
		t.Errorf("expected ErrScheduleTimeConflict, got %v", err)
	}
}

func TestBookingService_TwoHourCancellationRuleAndRefund(t *testing.T) {
	svc, repo, _, _ := setupBookingService()
	ctx := context.Background()

	branchID := uuid.New()
	trainerID := uuid.New()
	member1 := uuid.New()
	member2 := uuid.New()

	// Member 1 has SESSION package with 5 entries
	pkg1 := &domain.MemberPackage{
		ID:               uuid.New(),
		MemberID:         member1,
		BranchID:         branchID,
		AllBranches:      true,
		Status:           domain.StatusActive,
		PackageKind:      domain.PackageKindSession,
		RemainingEntries: 5,
	}
	repo.memberPackages[member1] = pkg1

	// Member 2 has SESSION package with 5 entries
	pkg2 := &domain.MemberPackage{
		ID:               uuid.New(),
		MemberID:         member2,
		BranchID:         branchID,
		AllBranches:      true,
		Status:           domain.StatusActive,
		PackageKind:      domain.PackageKindSession,
		RemainingEntries: 5,
	}
	repo.memberPackages[member2] = pkg2

	class := &domain.Class{ID: uuid.New(), Name: "Yoga Flow", DurationMinutes: 60, IsActive: true}
	_ = repo.CreateClass(ctx, class)

	// Scenario A: Class starts in 5 hours (> 2 hours) -> Free Cancellation
	schedFar := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class.ID,
		BranchID:    branchID,
		TrainerID:   trainerID,
		StartTime:   time.Now().UTC().Add(5 * time.Hour),
		EndTime:     time.Now().UTC().Add(6 * time.Hour),
		Capacity:    10,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedFar)

	bookRes1, err := svc.BookClass(ctx, member1, schedFar.ID)
	if err != nil {
		t.Fatalf("booking failed: %v", err)
	}
	if pkg1.RemainingEntries != 4 {
		t.Errorf("expected 4 remaining entries, got %d", pkg1.RemainingEntries)
	}

	// Cancel early (> 2 hours)
	cancelRes1, err := svc.CancelBooking(ctx, member1, domain.RoleMember, bookRes1.Booking.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if cancelRes1.IsLate {
		t.Error("expected IsLate == false for 5-hour advance cancellation")
	}
	if cancelRes1.PenaltyApplied {
		t.Error("expected PenaltyApplied == false for free cancellation")
	}
	if cancelRes1.Booking.Status != domain.BookingStatusCancelled {
		t.Errorf("expected status CANCELLED, got %s", cancelRes1.Booking.Status)
	}
	if pkg1.RemainingEntries != 5 {
		t.Errorf("expected session entry refunded to 5, got %d", pkg1.RemainingEntries)
	}

	// Scenario B: Class starts in 1 hour (< 2 hours) -> Late Cancellation with Penalty
	schedSoon := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class.ID,
		BranchID:    branchID,
		TrainerID:   trainerID,
		StartTime:   time.Now().UTC().Add(1 * time.Hour),
		EndTime:     time.Now().UTC().Add(2 * time.Hour),
		Capacity:    10,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedSoon)

	bookRes2, err := svc.BookClass(ctx, member2, schedSoon.ID)
	if err != nil {
		t.Fatalf("booking 2 failed: %v", err)
	}
	if pkg2.RemainingEntries != 4 {
		t.Errorf("expected 4 remaining entries, got %d", pkg2.RemainingEntries)
	}

	// Cancel late (< 2 hours)
	cancelRes2, err := svc.CancelBooking(ctx, member2, domain.RoleMember, bookRes2.Booking.ID)
	if err != nil {
		t.Fatalf("late cancel failed: %v", err)
	}
	if !cancelRes2.IsLate {
		t.Error("expected IsLate == true for 1-hour advance cancellation")
	}
	if !cancelRes2.PenaltyApplied {
		t.Error("expected PenaltyApplied == true for late cancellation")
	}
	if cancelRes2.Booking.Status != domain.BookingStatusLateCancelled {
		t.Errorf("expected status LATE_CANCELLED, got %s", cancelRes2.Booking.Status)
	}
	if pkg2.RemainingEntries != 4 {
		t.Errorf("expected NO refund on late cancellation (stay 4), got %d", pkg2.RemainingEntries)
	}
}

func TestBookingService_AttendanceAndNoShowPenalty(t *testing.T) {
	svc, repo, _, _ := setupBookingService()
	ctx := context.Background()

	branchID := uuid.New()
	trainerID := uuid.New()
	member1 := uuid.New()
	member2 := uuid.New()

	for _, m := range []uuid.UUID{member1, member2} {
		repo.memberPackages[m] = &domain.MemberPackage{
			ID:          uuid.New(),
			MemberID:    m,
			BranchID:    branchID,
			AllBranches: true,
			Status:      domain.StatusActive,
			PackageKind: domain.PackageKindDuration,
		}
	}

	class := &domain.Class{ID: uuid.New(), Name: "HIIT", DurationMinutes: 45, IsActive: true}
	_ = repo.CreateClass(ctx, class)

	schedule := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class.ID,
		BranchID:    branchID,
		TrainerID:   trainerID,
		StartTime:   time.Now().UTC().Add(2 * time.Hour),
		EndTime:     time.Now().UTC().Add(3 * time.Hour),
		Capacity:    10,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedule)

	_, _ = svc.BookClass(ctx, member1, schedule.ID)
	_, _ = svc.BookClass(ctx, member2, schedule.ID)

	// Member 1 Attended
	err := svc.MarkAttendance(ctx, schedule.ID, member1, domain.BookingStatusAttended, trainerID)
	if err != nil {
		t.Fatalf("MarkAttendance ATTENDED failed: %v", err)
	}
	b1, _ := repo.GetBookingByMemberAndSchedule(ctx, schedule.ID, member1)
	if b1.Status != domain.BookingStatusAttended || b1.PenaltyApplied {
		t.Errorf("unexpected attended booking: %+v", b1)
	}

	// Member 2 No Show
	err = svc.MarkAttendance(ctx, schedule.ID, member2, domain.BookingStatusNoShow, trainerID)
	if err != nil {
		t.Fatalf("MarkAttendance NO_SHOW failed: %v", err)
	}
	b2, _ := repo.GetBookingByMemberAndSchedule(ctx, schedule.ID, member2)
	if b2.Status != domain.BookingStatusNoShow || !b2.PenaltyApplied {
		t.Errorf("unexpected no-show booking: %+v", b2)
	}
}
