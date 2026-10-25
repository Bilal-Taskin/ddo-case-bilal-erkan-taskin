package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fitgiris/internal/domain"
	"fitgiris/internal/repository"
)

type BookingService interface {
	// Classes
	CreateClass(ctx context.Context, req CreateClassRequest, creatorID uuid.UUID) (*domain.Class, error)
	ListClasses(ctx context.Context, activeOnly bool) ([]domain.Class, error)

	// Schedules
	CreateSchedule(ctx context.Context, req CreateScheduleRequest, creatorID uuid.UUID) (*domain.ClassSchedule, error)
	GetScheduleByID(ctx context.Context, id uuid.UUID) (*domain.ClassSchedule, error)
	ListSchedules(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID, from, to *time.Time) ([]domain.ClassSchedule, error)

	// Booking & Concurrency
	BookClass(ctx context.Context, memberID, scheduleID uuid.UUID) (*domain.BookingResult, error)
	CancelBooking(ctx context.Context, callerID uuid.UUID, callerRole domain.UserRole, bookingID uuid.UUID) (*domain.CancelBookingResult, error)
	CancelWaitlist(ctx context.Context, callerID uuid.UUID, callerRole domain.UserRole, waitlistID uuid.UUID) error

	// Attendance
	MarkAttendance(ctx context.Context, scheduleID, memberID uuid.UUID, status domain.BookingStatus, staffID uuid.UUID) error

	// Queries
	ListMyBookings(ctx context.Context, memberID uuid.UUID, activeOnly bool) ([]domain.ClassBooking, error)
	ListMyWaitlist(ctx context.Context, memberID uuid.UUID) ([]domain.ClassWaitlist, error)
	ListScheduleBookings(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassBooking, error)
	ListScheduleWaitlist(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassWaitlist, error)
}

type DefaultBookingService struct {
	bookingRepo repository.BookingRepository
	userRepo    repository.UserRepository
	auditRepo   repository.AuditRepository
}

func NewBookingService(
	bookingRepo repository.BookingRepository,
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
) *DefaultBookingService {
	return &DefaultBookingService{
		bookingRepo: bookingRepo,
		userRepo:    userRepo,
		auditRepo:   auditRepo,
	}
}

// Request Models

type CreateClassRequest struct {
	Name            string  `json:"name"`
	Description     *string `json:"description,omitempty"`
	DurationMinutes int     `json:"duration_minutes"`
	DefaultCapacity int     `json:"default_capacity"`
}

type CreateScheduleRequest struct {
	ClassID   uuid.UUID `json:"class_id"`
	BranchID  uuid.UUID `json:"branch_id"`
	TrainerID uuid.UUID `json:"trainer_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Capacity  int       `json:"capacity"`
}

// ==========================================
// Class Operations
// ==========================================

func (s *DefaultBookingService) CreateClass(ctx context.Context, req CreateClassRequest, creatorID uuid.UUID) (*domain.Class, error) {
	if req.Name == "" {
		return nil, errors.New("ders adi bos olamaz")
	}
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 50
	}
	if req.DefaultCapacity <= 0 {
		req.DefaultCapacity = 20
	}

	class := &domain.Class{
		ID:              uuid.New(),
		Name:            req.Name,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		DefaultCapacity: req.DefaultCapacity,
		IsActive:        true,
		CreatedAt:       time.Now().UTC(),
	}

	if err := s.bookingRepo.CreateClass(ctx, class); err != nil {
		return nil, fmt.Errorf("ders olusturulamadi: %w", err)
	}

	newVal, _ := json.Marshal(class)
	_ = s.auditRepo.Append(ctx, &domain.AuditLog{
		UserID:    &creatorID,
		Action:    "CLASS_CREATED",
		Entity:    "classes",
		EntityID:  class.ID.String(),
		NewValues: newVal,
		CreatedAt: time.Now().UTC(),
	})

	return class, nil
}

func (s *DefaultBookingService) ListClasses(ctx context.Context, activeOnly bool) ([]domain.Class, error) {
	return s.bookingRepo.ListClasses(ctx, activeOnly)
}

// ==========================================
// Schedule Operations
// ==========================================

func (s *DefaultBookingService) CreateSchedule(ctx context.Context, req CreateScheduleRequest, creatorID uuid.UUID) (*domain.ClassSchedule, error) {
	if req.ClassID == uuid.Nil {
		return nil, errors.New("class_id zorunludur")
	}
	if req.BranchID == uuid.Nil {
		return nil, errors.New("branch_id zorunludur")
	}
	if req.TrainerID == uuid.Nil {
		return nil, errors.New("trainer_id zorunludur")
	}
	if req.Capacity <= 0 {
		return nil, errors.New("kontenjan 0'dan buyuk olmalidir")
	}
	if req.StartTime.Before(time.Now().UTC()) {
		return nil, errors.New("ders baslangic zamani gelecekte olmalidir")
	}
	if !req.EndTime.After(req.StartTime) {
		return nil, errors.New("ders bitis zamani baslangic zamanindan sonra olmalidir")
	}

	// Validate class exists
	class, err := s.bookingRepo.GetClassByID(ctx, req.ClassID)
	if err != nil {
		return nil, err
	}
	if !class.IsActive {
		return nil, errors.New("secilen ders aktif durumda degil")
	}

	// Validate trainer exists and has TRAINER role
	trainer, err := s.userRepo.GetByID(ctx, req.TrainerID)
	if err != nil {
		return nil, fmt.Errorf("egitmen dogrulanamadi: %w", err)
	}
	if trainer.Role != domain.RoleTrainer && trainer.Role != domain.RoleManager {
		return nil, errors.New("secilen kullanici egitmen veya mudur rolu tasimalidir")
	}

	now := time.Now().UTC()
	schedule := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     req.ClassID,
		BranchID:    req.BranchID,
		TrainerID:   req.TrainerID,
		StartTime:   req.StartTime.UTC(),
		EndTime:     req.EndTime.UTC(),
		Capacity:    req.Capacity,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.bookingRepo.CreateSchedule(ctx, schedule); err != nil {
		return nil, fmt.Errorf("ders seansi olusturulamadi: %w", err)
	}

	newVal, _ := json.Marshal(schedule)
	_ = s.auditRepo.Append(ctx, &domain.AuditLog{
		UserID:    &creatorID,
		Action:    "SCHEDULE_CREATED",
		Entity:    "class_schedules",
		EntityID:  schedule.ID.String(),
		NewValues: newVal,
		CreatedAt: time.Now().UTC(),
	})

	return schedule, nil
}

func (s *DefaultBookingService) GetScheduleByID(ctx context.Context, id uuid.UUID) (*domain.ClassSchedule, error) {
	return s.bookingRepo.GetScheduleByID(ctx, id)
}

func (s *DefaultBookingService) ListSchedules(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID, from, to *time.Time) ([]domain.ClassSchedule, error) {
	return s.bookingRepo.ListSchedules(ctx, branchID, trainerID, from, to)
}

// ==========================================
// Booking & Concurrency Handling
// ==========================================

// BookClass handles concurrent class reservations using SELECT ... FOR UPDATE pessimistic row locks.
// If capacity is available, creates a CONFIRMED booking and increments booked_count.
// If full, adds member to class_waitlist in FIFO order.
// Ensures members cannot double-book overlapping classes or book without an active membership.
func (s *DefaultBookingService) BookClass(ctx context.Context, memberID, scheduleID uuid.UUID) (*domain.BookingResult, error) {
	var result *domain.BookingResult

	err := s.bookingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// 1. Verify Member has an active membership package
		pkg, err := s.bookingRepo.GetActiveMemberPackageTx(ctx, tx, memberID)
		if err != nil {
			return err
		}

		// 2. Pessimistic Row Lock on class_schedules to prevent race conditions on capacity
		schedule, err := s.bookingRepo.GetScheduleForUpdateTx(ctx, tx, scheduleID)
		if err != nil {
			return err
		}

		if schedule.Status != domain.ScheduleStatusScheduled {
			return domain.ErrScheduleCancelled
		}
		if schedule.StartTime.Before(time.Now().UTC()) {
			return domain.ErrSchedulePast
		}

		// 3. Check Branch Access
		if !pkg.AllBranches && pkg.BranchID != schedule.BranchID {
			return domain.ErrBranchAccessDenied
		}

		// 4. Check Session Balance
		if pkg.PackageKind == domain.PackageKindSession && pkg.RemainingEntries <= 0 {
			return domain.ErrInsufficientEntries
		}

		// 5. Check overlapping class conflict
		conflict, err := s.bookingRepo.HasTimeConflictTx(ctx, tx, memberID, schedule.StartTime, schedule.EndTime, &schedule.ID)
		if err != nil {
			return err
		}
		if conflict {
			return domain.ErrScheduleTimeConflict
		}

		// 6. Check if member already has CONFIRMED booking for this schedule
		existingBooking, err := s.bookingRepo.GetBookingByMemberAndScheduleTx(ctx, tx, scheduleID, memberID)
		if err == nil && existingBooking != nil && existingBooking.Status == domain.BookingStatusConfirmed {
			return domain.ErrClassAlreadyBooked
		}

		// 7. Check if member is already in waitlist
		existingWaitlist, err := s.bookingRepo.GetWaitlistByMemberAndScheduleTx(ctx, tx, scheduleID, memberID)
		if err == nil && existingWaitlist != nil && existingWaitlist.Status == domain.WaitlistStatusWaiting {
			return domain.ErrClassAlreadyInWaitlist
		}

		now := time.Now().UTC()

		// 8. Capacity Check
		if schedule.BookedCount < schedule.Capacity {
			// Spot Available!
			newBookedCount := schedule.BookedCount + 1
			if err := s.bookingRepo.UpdateScheduleBookedCountTx(ctx, tx, schedule.ID, newBookedCount); err != nil {
				return err
			}

			booking := &domain.ClassBooking{
				ID:             uuid.New(),
				ScheduleID:     schedule.ID,
				MemberID:       memberID,
				Status:         domain.BookingStatusConfirmed,
				CancelledAt:    nil,
				PenaltyApplied: false,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if existingBooking != nil {
				booking.ID = existingBooking.ID
			}

			if err := s.bookingRepo.CreateOrUpdateBookingTx(ctx, tx, booking); err != nil {
				return err
			}

			// Deduct session entry if session-based package
			if pkg.PackageKind == domain.PackageKindSession {
				if err := s.bookingRepo.DeductSessionEntryTx(ctx, tx, pkg.ID); err != nil {
					return err
				}
			}

			newVal, _ := json.Marshal(booking)
			_ = s.auditRepo.Append(ctx, &domain.AuditLog{
				UserID:    &memberID,
				Action:    "CLASS_BOOKED",
				Entity:    "class_bookings",
				EntityID:  booking.ID.String(),
				NewValues: newVal,
				CreatedAt: now,
			})

			result = &domain.BookingResult{
				IsWaitlisted: false,
				Booking:      booking,
				Message:      "Rezervasyonunuz basariyla olusturuldu",
			}
			return nil
		}

		// Capacity is Full! Add to FIFO Waitlist
		waitlist := &domain.ClassWaitlist{
			ID:         uuid.New(),
			ScheduleID: schedule.ID,
			MemberID:   memberID,
			Status:     domain.WaitlistStatusWaiting,
			CreatedAt:  now,
		}
		if existingWaitlist != nil {
			waitlist.ID = existingWaitlist.ID
		}

		if err := s.bookingRepo.CreateOrUpdateWaitlistTx(ctx, tx, waitlist); err != nil {
			return err
		}

		position, err := s.bookingRepo.GetWaitlistPositionTx(ctx, tx, schedule.ID, memberID)
		if err != nil {
			return err
		}
		waitlist.Position = position

		newVal, _ := json.Marshal(waitlist)
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &memberID,
			Action:    "CLASS_WAITLIST_JOINED",
			Entity:    "class_waitlist",
			EntityID:  waitlist.ID.String(),
			NewValues: newVal,
			CreatedAt: now,
		})

		result = &domain.BookingResult{
			IsWaitlisted:     true,
			Waitlist:         waitlist,
			WaitlistPosition: position,
			Message:          fmt.Sprintf("Kontenjan dolu oldugu icin bekleme listesine %d. siradan eklendiniz", position),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// CancelBooking implements the 2-hour cancellation rule and automatic waitlist promotion using SKIP LOCKED.
// If canceled >= 2 hours before start_time:
//   - Status = CANCELLED, PenaltyApplied = false, entry refunded (if session package).
// If canceled < 2 hours before start_time:
//   - Status = LATE_CANCELLED, PenaltyApplied = true, NO entry refund (penalty applied).
// In both cases, the first member in the waitlist is atomically promoted to CONFIRMED using SELECT ... FOR UPDATE SKIP LOCKED.
func (s *DefaultBookingService) CancelBooking(ctx context.Context, callerID uuid.UUID, callerRole domain.UserRole, bookingID uuid.UUID) (*domain.CancelBookingResult, error) {
	var cancelResult *domain.CancelBookingResult

	err := s.bookingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// 1. Lock the booking
		booking, err := s.bookingRepo.GetBookingForUpdateTx(ctx, tx, bookingID)
		if err != nil {
			return err
		}

		// Check permission: Member can only cancel their own booking
		if callerRole == domain.RoleMember && booking.MemberID != callerID {
			return domain.ErrForbidden
		}

		if booking.Status != domain.BookingStatusConfirmed {
			return domain.ErrBookingAlreadyCancelled
		}

		// 2. Lock the schedule
		schedule, err := s.bookingRepo.GetScheduleForUpdateTx(ctx, tx, booking.ScheduleID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		if now.After(schedule.StartTime) {
			return domain.ErrCannotCancelPastClass
		}

		// 3. Evaluate 2-hour rule
		isLate := domain.IsLateCancellation(schedule.StartTime, now)
		booking.CancelledAt = &now

		if !isLate {
			// Free cancellation (>= 2 hours before start)
			booking.Status = domain.BookingStatusCancelled
			booking.PenaltyApplied = false

			// Refund session entry if member has an active session package
			pkg, err := s.bookingRepo.GetActiveMemberPackageTx(ctx, tx, booking.MemberID)
			if err == nil && pkg.PackageKind == domain.PackageKindSession {
				_ = s.bookingRepo.RefundSessionEntryTx(ctx, tx, pkg.ID)
			}
		} else {
			// Late cancellation (< 2 hours before start)
			booking.Status = domain.BookingStatusLateCancelled
			booking.PenaltyApplied = true
			// No refund; penalty applied
		}

		if err := s.bookingRepo.UpdateBookingTx(ctx, tx, booking); err != nil {
			return err
		}

		// 4. Automatic Waitlist Promotion using SELECT ... FOR UPDATE SKIP LOCKED
		var promotedMemberID *uuid.UUID
		nextWaiting, err := s.bookingRepo.GetNextWaitingForUpdateSkipLockedTx(ctx, tx, schedule.ID)
		if err != nil {
			return err
		}

		if nextWaiting != nil {
			// Promote next waitlisted member
			nextWaiting.Status = domain.WaitlistStatusPromoted
			nextWaiting.PromotedAt = &now
			if err := s.bookingRepo.UpdateWaitlistTx(ctx, tx, nextWaiting); err != nil {
				return err
			}

			// Create CONFIRMED booking for promoted member
			promotedBooking := &domain.ClassBooking{
				ID:             uuid.New(),
				ScheduleID:     schedule.ID,
				MemberID:       nextWaiting.MemberID,
				Status:         domain.BookingStatusConfirmed,
				CancelledAt:    nil,
				PenaltyApplied: false,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := s.bookingRepo.CreateOrUpdateBookingTx(ctx, tx, promotedBooking); err != nil {
				return err
			}

			// If promoted member has session package, deduct session entry
			promotedPkg, err := s.bookingRepo.GetActiveMemberPackageTx(ctx, tx, nextWaiting.MemberID)
			if err == nil && promotedPkg.PackageKind == domain.PackageKindSession {
				_ = s.bookingRepo.DeductSessionEntryTx(ctx, tx, promotedPkg.ID)
			}

			promotedMemberID = &nextWaiting.MemberID
			// Note: booked_count does not change (1 cancelled spot was immediately filled by promoted member)

			promotedVal, _ := json.Marshal(promotedBooking)
			_ = s.auditRepo.Append(ctx, &domain.AuditLog{
				UserID:    &nextWaiting.MemberID,
				Action:    "WAITLIST_PROMOTED",
				Entity:    "class_bookings",
				EntityID:  promotedBooking.ID.String(),
				NewValues: promotedVal,
				CreatedAt: now,
			})
		} else {
			// No waitlist: decrement booked_count
			newCount := schedule.BookedCount - 1
			if newCount < 0 {
				newCount = 0
			}
			if err := s.bookingRepo.UpdateScheduleBookedCountTx(ctx, tx, schedule.ID, newCount); err != nil {
				return err
			}
		}

		action := "BOOKING_CANCELLED"
		msg := "Rezervasyonunuz basariyla iptal edildi"
		if isLate {
			action = "BOOKING_LATE_CANCELLED"
			msg = "Ders baslangicina 2 saatten az sure kaldigi icin gec iptal cezasi uygulandi (giris hakki iade edilmedi)"
		}

		auditVal, _ := json.Marshal(booking)
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &callerID,
			Action:    action,
			Entity:    "class_bookings",
			EntityID:  booking.ID.String(),
			NewValues: auditVal,
			CreatedAt: now,
		})

		cancelResult = &domain.CancelBookingResult{
			Booking:          booking,
			IsLate:           isLate,
			PenaltyApplied:   booking.PenaltyApplied,
			PromotedMemberID: promotedMemberID,
			Message:          msg,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return cancelResult, nil
}

func (s *DefaultBookingService) CancelWaitlist(ctx context.Context, callerID uuid.UUID, callerRole domain.UserRole, waitlistID uuid.UUID) error {
	return s.bookingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
		w, err := s.bookingRepo.GetWaitlistForUpdateTx(ctx, tx, waitlistID)
		if err != nil {
			return err
		}

		if callerRole == domain.RoleMember && w.MemberID != callerID {
			return domain.ErrForbidden
		}

		if w.Status != domain.WaitlistStatusWaiting {
			return domain.ErrWaitlistAlreadyCancelled
		}

		w.Status = domain.WaitlistStatusCancelled
		if err := s.bookingRepo.UpdateWaitlistTx(ctx, tx, w); err != nil {
			return err
		}

		now := time.Now().UTC()
		val, _ := json.Marshal(w)
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &callerID,
			Action:    "WAITLIST_CANCELLED",
			Entity:    "class_waitlist",
			EntityID:  w.ID.String(),
			NewValues: val,
			CreatedAt: now,
		})

		return nil
	})
}

// MarkAttendance updates booking status to ATTENDED or NO_SHOW (with penalty applied for no-shows).
func (s *DefaultBookingService) MarkAttendance(ctx context.Context, scheduleID, memberID uuid.UUID, status domain.BookingStatus, staffID uuid.UUID) error {
	if status != domain.BookingStatusAttended && status != domain.BookingStatusNoShow {
		return domain.ErrInvalidAttendanceStatus
	}

	return s.bookingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
		booking, err := s.bookingRepo.GetBookingByMemberAndScheduleTx(ctx, tx, scheduleID, memberID)
		if err != nil {
			return err
		}

		if booking.Status != domain.BookingStatusConfirmed {
			return errors.New("sadece onayli (CONFIRMED) rezervasyonlar icin yoklama girilebilir")
		}

		booking.Status = status
		if status == domain.BookingStatusNoShow {
			booking.PenaltyApplied = true
		}

		if err := s.bookingRepo.UpdateBookingTx(ctx, tx, booking); err != nil {
			return err
		}

		now := time.Now().UTC()
		val, _ := json.Marshal(booking)
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &staffID,
			Action:    "ATTENDANCE_MARKED",
			Entity:    "class_bookings",
			EntityID:  booking.ID.String(),
			NewValues: val,
			CreatedAt: now,
		})

		return nil
	})
}

func (s *DefaultBookingService) ListMyBookings(ctx context.Context, memberID uuid.UUID, activeOnly bool) ([]domain.ClassBooking, error) {
	return s.bookingRepo.ListBookingsByMember(ctx, memberID, activeOnly)
}

func (s *DefaultBookingService) ListMyWaitlist(ctx context.Context, memberID uuid.UUID) ([]domain.ClassWaitlist, error) {
	return s.bookingRepo.ListWaitlistByMember(ctx, memberID)
}

func (s *DefaultBookingService) ListScheduleBookings(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassBooking, error) {
	return s.bookingRepo.ListBookingsBySchedule(ctx, scheduleID)
}

func (s *DefaultBookingService) ListScheduleWaitlist(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassWaitlist, error) {
	return s.bookingRepo.ListWaitlistBySchedule(ctx, scheduleID)
}
