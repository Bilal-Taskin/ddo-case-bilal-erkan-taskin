package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitgiris/internal/domain"
)

type BookingRepository interface {
	// Classes
	CreateClass(ctx context.Context, c *domain.Class) error
	GetClassByID(ctx context.Context, id uuid.UUID) (*domain.Class, error)
	ListClasses(ctx context.Context, activeOnly bool) ([]domain.Class, error)

	// Schedules
	CreateSchedule(ctx context.Context, s *domain.ClassSchedule) error
	GetScheduleByID(ctx context.Context, id uuid.UUID) (*domain.ClassSchedule, error)
	GetScheduleForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassSchedule, error)
	UpdateScheduleBookedCountTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, bookedCount int) error
	UpdateScheduleStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleStatus) error
	ListSchedules(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID, from, to *time.Time) ([]domain.ClassSchedule, error)

	// Bookings
	CreateOrUpdateBookingTx(ctx context.Context, tx pgx.Tx, b *domain.ClassBooking) error
	GetBookingByID(ctx context.Context, id uuid.UUID) (*domain.ClassBooking, error)
	GetBookingForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassBooking, error)
	GetBookingByMemberAndSchedule(ctx context.Context, scheduleID, memberID uuid.UUID) (*domain.ClassBooking, error)
	GetBookingByMemberAndScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (*domain.ClassBooking, error)
	ListBookingsByMember(ctx context.Context, memberID uuid.UUID, activeOnly bool) ([]domain.ClassBooking, error)
	ListBookingsBySchedule(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassBooking, error)
	UpdateBookingTx(ctx context.Context, tx pgx.Tx, b *domain.ClassBooking) error
	HasTimeConflictTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID, startTime, endTime time.Time, excludeScheduleID *uuid.UUID) (bool, error)

	// Waitlist
	CreateOrUpdateWaitlistTx(ctx context.Context, tx pgx.Tx, w *domain.ClassWaitlist) error
	GetWaitlistByID(ctx context.Context, id uuid.UUID) (*domain.ClassWaitlist, error)
	GetWaitlistForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassWaitlist, error)
	GetWaitlistByMemberAndScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (*domain.ClassWaitlist, error)
	GetNextWaitingForUpdateSkipLockedTx(ctx context.Context, tx pgx.Tx, scheduleID uuid.UUID) (*domain.ClassWaitlist, error)
	UpdateWaitlistTx(ctx context.Context, tx pgx.Tx, w *domain.ClassWaitlist) error
	GetWaitlistPositionTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (int, error)
	ListWaitlistBySchedule(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassWaitlist, error)
	ListWaitlistByMember(ctx context.Context, memberID uuid.UUID) ([]domain.ClassWaitlist, error)
	GetWaitlistCount(ctx context.Context, scheduleID uuid.UUID) (int, error)

	// Membership / Session helpers in Tx
	GetActiveMemberPackageTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID) (*domain.MemberPackage, error)
	DeductSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error
	RefundSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error

	// Transaction helper
	WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type PostgresBookingRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresBookingRepository(pool *pgxpool.Pool) *PostgresBookingRepository {
	return &PostgresBookingRepository{pool: pool}
}

func (r *PostgresBookingRepository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction baslatilamadi: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaction commit edilemedi: %w", err)
	}
	return nil
}

// ==========================================
// Class Operations
// ==========================================

func (r *PostgresBookingRepository) CreateClass(ctx context.Context, c *domain.Class) error {
	query := `
		INSERT INTO classes (
			id, name, description, duration_minutes, default_capacity, is_active, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, query,
		c.ID,
		c.Name,
		c.Description,
		c.DurationMinutes,
		c.DefaultCapacity,
		c.IsActive,
		c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("ders olusturulamadi: %w", err)
	}
	return nil
}

func (r *PostgresBookingRepository) GetClassByID(ctx context.Context, id uuid.UUID) (*domain.Class, error) {
	query := `
		SELECT id, name, description, duration_minutes, default_capacity, is_active, created_at
		FROM classes
		WHERE id = $1
	`
	var c domain.Class
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID,
		&c.Name,
		&c.Description,
		&c.DurationMinutes,
		&c.DefaultCapacity,
		&c.IsActive,
		&c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrClassNotFound
		}
		return nil, fmt.Errorf("ders getirilemedi: %w", err)
	}
	return &c, nil
}

func (r *PostgresBookingRepository) ListClasses(ctx context.Context, activeOnly bool) ([]domain.Class, error) {
	query := `
		SELECT id, name, description, duration_minutes, default_capacity, is_active, created_at
		FROM classes
		WHERE ($1 = false OR is_active = true)
		ORDER BY name ASC
	`
	rows, err := r.pool.Query(ctx, query, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("dersler listelenemedi: %w", err)
	}
	defer rows.Close()

	var classes []domain.Class
	for rows.Next() {
		var c domain.Class
		if err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.Description,
			&c.DurationMinutes,
			&c.DefaultCapacity,
			&c.IsActive,
			&c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ders satiri okunamadi: %w", err)
		}
		classes = append(classes, c)
	}
	return classes, nil
}

// ==========================================
// Class Schedule Operations
// ==========================================

func (r *PostgresBookingRepository) CreateSchedule(ctx context.Context, s *domain.ClassSchedule) error {
	query := `
		INSERT INTO class_schedules (
			id, class_id, branch_id, trainer_id, start_time, end_time, capacity, booked_count, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}
	if s.Status == "" {
		s.Status = domain.ScheduleStatusScheduled
	}

	_, err := r.pool.Exec(ctx, query,
		s.ID,
		s.ClassID,
		s.BranchID,
		s.TrainerID,
		s.StartTime,
		s.EndTime,
		s.Capacity,
		s.BookedCount,
		string(s.Status),
		s.CreatedAt,
		s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("ders seansi olusturulamadi: %w", err)
	}
	return nil
}

func (r *PostgresBookingRepository) GetScheduleByID(ctx context.Context, id uuid.UUID) (*domain.ClassSchedule, error) {
	query := `
		SELECT 
			s.id, s.class_id, s.branch_id, s.trainer_id, s.start_time, s.end_time,
			s.capacity, s.booked_count, s.status, s.created_at, s.updated_at,
			c.name, b.name, u.first_name || ' ' || u.last_name,
			COALESCE((SELECT COUNT(*) FROM class_waitlist w WHERE w.schedule_id = s.id AND w.status = 'WAITING'), 0) as waitlist_count
		FROM class_schedules s
		JOIN classes c ON s.class_id = c.id
		JOIN branches b ON s.branch_id = b.id
		JOIN users u ON s.trainer_id = u.id
		WHERE s.id = $1
	`
	var s domain.ClassSchedule
	var statusStr string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&s.ClassID,
		&s.BranchID,
		&s.TrainerID,
		&s.StartTime,
		&s.EndTime,
		&s.Capacity,
		&s.BookedCount,
		&statusStr,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.ClassName,
		&s.BranchName,
		&s.TrainerName,
		&s.WaitlistCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrScheduleNotFound
		}
		return nil, fmt.Errorf("ders seansi getirilemedi: %w", err)
	}

	s.Status = domain.ScheduleStatus(statusStr)
	s.AvailableSpots = s.Capacity - s.BookedCount
	if s.AvailableSpots < 0 {
		s.AvailableSpots = 0
	}
	return &s, nil
}

// GetScheduleForUpdateTx locks the schedule row pessimistically using SELECT ... FOR UPDATE.
func (r *PostgresBookingRepository) GetScheduleForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassSchedule, error) {
	query := `
		SELECT 
			s.id, s.class_id, s.branch_id, s.trainer_id, s.start_time, s.end_time,
			s.capacity, s.booked_count, s.status, s.created_at, s.updated_at,
			c.name, b.name, u.first_name || ' ' || u.last_name
		FROM class_schedules s
		JOIN classes c ON s.class_id = c.id
		JOIN branches b ON s.branch_id = b.id
		JOIN users u ON s.trainer_id = u.id
		WHERE s.id = $1
		FOR UPDATE OF s
	`
	var s domain.ClassSchedule
	var statusStr string
	err := tx.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&s.ClassID,
		&s.BranchID,
		&s.TrainerID,
		&s.StartTime,
		&s.EndTime,
		&s.Capacity,
		&s.BookedCount,
		&statusStr,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.ClassName,
		&s.BranchName,
		&s.TrainerName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrScheduleNotFound
		}
		return nil, fmt.Errorf("ders seansi kilitlenemedi: %w", err)
	}

	s.Status = domain.ScheduleStatus(statusStr)
	s.AvailableSpots = s.Capacity - s.BookedCount
	if s.AvailableSpots < 0 {
		s.AvailableSpots = 0
	}
	return &s, nil
}

func (r *PostgresBookingRepository) UpdateScheduleBookedCountTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, bookedCount int) error {
	query := `
		UPDATE class_schedules
		SET booked_count = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, id, bookedCount)
	if err != nil {
		return fmt.Errorf("ders seansi kontenjan guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresBookingRepository) UpdateScheduleStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleStatus) error {
	query := `
		UPDATE class_schedules
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, string(status))
	if err != nil {
		return fmt.Errorf("ders seansi durumu guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresBookingRepository) ListSchedules(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID, from, to *time.Time) ([]domain.ClassSchedule, error) {
	baseWhere := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if branchID != nil {
		baseWhere += fmt.Sprintf(" AND s.branch_id = $%d", argIdx)
		args = append(args, *branchID)
		argIdx++
	}
	if trainerID != nil {
		baseWhere += fmt.Sprintf(" AND s.trainer_id = $%d", argIdx)
		args = append(args, *trainerID)
		argIdx++
	}
	if from != nil {
		baseWhere += fmt.Sprintf(" AND s.start_time >= $%d", argIdx)
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		baseWhere += fmt.Sprintf(" AND s.start_time <= $%d", argIdx)
		args = append(args, *to)
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT 
			s.id, s.class_id, s.branch_id, s.trainer_id, s.start_time, s.end_time,
			s.capacity, s.booked_count, s.status, s.created_at, s.updated_at,
			c.name, b.name, u.first_name || ' ' || u.last_name,
			COALESCE((SELECT COUNT(*) FROM class_waitlist w WHERE w.schedule_id = s.id AND w.status = 'WAITING'), 0) as waitlist_count
		FROM class_schedules s
		JOIN classes c ON s.class_id = c.id
		JOIN branches b ON s.branch_id = b.id
		JOIN users u ON s.trainer_id = u.id
		%s
		ORDER BY s.start_time ASC
	`, baseWhere)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ders seanslari listelenemedi: %w", err)
	}
	defer rows.Close()

	var schedules []domain.ClassSchedule
	for rows.Next() {
		var s domain.ClassSchedule
		var statusStr string
		if err := rows.Scan(
			&s.ID,
			&s.ClassID,
			&s.BranchID,
			&s.TrainerID,
			&s.StartTime,
			&s.EndTime,
			&s.Capacity,
			&s.BookedCount,
			&statusStr,
			&s.CreatedAt,
			&s.UpdatedAt,
			&s.ClassName,
			&s.BranchName,
			&s.TrainerName,
			&s.WaitlistCount,
		); err != nil {
			return nil, fmt.Errorf("ders seansi satiri okunamadi: %w", err)
		}
		s.Status = domain.ScheduleStatus(statusStr)
		s.AvailableSpots = s.Capacity - s.BookedCount
		if s.AvailableSpots < 0 {
			s.AvailableSpots = 0
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

// ==========================================
// Booking Operations
// ==========================================

func (r *PostgresBookingRepository) CreateOrUpdateBookingTx(ctx context.Context, tx pgx.Tx, b *domain.ClassBooking) error {
	query := `
		INSERT INTO class_bookings (
			id, schedule_id, member_id, status, cancelled_at, penalty_applied, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (schedule_id, member_id)
		DO UPDATE SET
			status = EXCLUDED.status,
			cancelled_at = EXCLUDED.cancelled_at,
			penalty_applied = EXCLUDED.penalty_applied,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	now := time.Now().UTC()
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	b.UpdatedAt = now

	err := tx.QueryRow(ctx, query,
		b.ID,
		b.ScheduleID,
		b.MemberID,
		string(b.Status),
		b.CancelledAt,
		b.PenaltyApplied,
		b.CreatedAt,
		b.UpdatedAt,
	).Scan(&b.ID)
	if err != nil {
		return fmt.Errorf("rezervasyon olusturulamadi/guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresBookingRepository) GetBookingByID(ctx context.Context, id uuid.UUID) (*domain.ClassBooking, error) {
	query := `
		SELECT 
			b.id, b.schedule_id, b.member_id, b.status, b.cancelled_at, b.penalty_applied, b.created_at, b.updated_at,
			c.name, br.name, u.first_name || ' ' || u.last_name, s.start_time, s.end_time,
			mem.first_name || ' ' || mem.last_name, mem.email
		FROM class_bookings b
		JOIN class_schedules s ON b.schedule_id = s.id
		JOIN classes c ON s.class_id = c.id
		JOIN branches br ON s.branch_id = br.id
		JOIN users u ON s.trainer_id = u.id
		JOIN users mem ON b.member_id = mem.id
		WHERE b.id = $1
	`
	var b domain.ClassBooking
	var statusStr string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&b.ID,
		&b.ScheduleID,
		&b.MemberID,
		&statusStr,
		&b.CancelledAt,
		&b.PenaltyApplied,
		&b.CreatedAt,
		&b.UpdatedAt,
		&b.ClassName,
		&b.BranchName,
		&b.TrainerName,
		&b.StartTime,
		&b.EndTime,
		&b.MemberName,
		&b.MemberEmail,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("rezervasyon getirilemedi: %w", err)
	}
	b.Status = domain.BookingStatus(statusStr)
	return &b, nil
}

func (r *PostgresBookingRepository) GetBookingForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassBooking, error) {
	query := `
		SELECT 
			b.id, b.schedule_id, b.member_id, b.status, b.cancelled_at, b.penalty_applied, b.created_at, b.updated_at,
			c.name, br.name, u.first_name || ' ' || u.last_name, s.start_time, s.end_time,
			mem.first_name || ' ' || mem.last_name, mem.email
		FROM class_bookings b
		JOIN class_schedules s ON b.schedule_id = s.id
		JOIN classes c ON s.class_id = c.id
		JOIN branches br ON s.branch_id = br.id
		JOIN users u ON s.trainer_id = u.id
		JOIN users mem ON b.member_id = mem.id
		WHERE b.id = $1
		FOR UPDATE OF b
	`
	var b domain.ClassBooking
	var statusStr string
	err := tx.QueryRow(ctx, query, id).Scan(
		&b.ID,
		&b.ScheduleID,
		&b.MemberID,
		&statusStr,
		&b.CancelledAt,
		&b.PenaltyApplied,
		&b.CreatedAt,
		&b.UpdatedAt,
		&b.ClassName,
		&b.BranchName,
		&b.TrainerName,
		&b.StartTime,
		&b.EndTime,
		&b.MemberName,
		&b.MemberEmail,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("rezervasyon kilitlenemedi: %w", err)
	}
	b.Status = domain.BookingStatus(statusStr)
	return &b, nil
}

func (r *PostgresBookingRepository) GetBookingByMemberAndSchedule(ctx context.Context, scheduleID, memberID uuid.UUID) (*domain.ClassBooking, error) {
	query := `
		SELECT id, schedule_id, member_id, status, cancelled_at, penalty_applied, created_at, updated_at
		FROM class_bookings
		WHERE schedule_id = $1 AND member_id = $2
	`
	var b domain.ClassBooking
	var statusStr string
	err := r.pool.QueryRow(ctx, query, scheduleID, memberID).Scan(
		&b.ID,
		&b.ScheduleID,
		&b.MemberID,
		&statusStr,
		&b.CancelledAt,
		&b.PenaltyApplied,
		&b.CreatedAt,
		&b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("rezervasyon sorgulanamadi: %w", err)
	}
	b.Status = domain.BookingStatus(statusStr)
	return &b, nil
}

func (r *PostgresBookingRepository) GetBookingByMemberAndScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (*domain.ClassBooking, error) {
	query := `
		SELECT id, schedule_id, member_id, status, cancelled_at, penalty_applied, created_at, updated_at
		FROM class_bookings
		WHERE schedule_id = $1 AND member_id = $2
		FOR UPDATE
	`
	var b domain.ClassBooking
	var statusStr string
	err := tx.QueryRow(ctx, query, scheduleID, memberID).Scan(
		&b.ID,
		&b.ScheduleID,
		&b.MemberID,
		&statusStr,
		&b.CancelledAt,
		&b.PenaltyApplied,
		&b.CreatedAt,
		&b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("rezervasyon sorgulanamadi: %w", err)
	}
	b.Status = domain.BookingStatus(statusStr)
	return &b, nil
}

func (r *PostgresBookingRepository) ListBookingsByMember(ctx context.Context, memberID uuid.UUID, activeOnly bool) ([]domain.ClassBooking, error) {
	baseWhere := "WHERE b.member_id = $1"
	if activeOnly {
		baseWhere += " AND b.status = 'CONFIRMED'"
	}

	query := fmt.Sprintf(`
		SELECT 
			b.id, b.schedule_id, b.member_id, b.status, b.cancelled_at, b.penalty_applied, b.created_at, b.updated_at,
			c.name, br.name, u.first_name || ' ' || u.last_name, s.start_time, s.end_time,
			mem.first_name || ' ' || mem.last_name, mem.email
		FROM class_bookings b
		JOIN class_schedules s ON b.schedule_id = s.id
		JOIN classes c ON s.class_id = c.id
		JOIN branches br ON s.branch_id = br.id
		JOIN users u ON s.trainer_id = u.id
		JOIN users mem ON b.member_id = mem.id
		%s
		ORDER BY s.start_time DESC
	`, baseWhere)

	rows, err := r.pool.Query(ctx, query, memberID)
	if err != nil {
		return nil, fmt.Errorf("uye rezervasyonlari listelenemedi: %w", err)
	}
	defer rows.Close()

	var list []domain.ClassBooking
	for rows.Next() {
		var b domain.ClassBooking
		var statusStr string
		if err := rows.Scan(
			&b.ID,
			&b.ScheduleID,
			&b.MemberID,
			&statusStr,
			&b.CancelledAt,
			&b.PenaltyApplied,
			&b.CreatedAt,
			&b.UpdatedAt,
			&b.ClassName,
			&b.BranchName,
			&b.TrainerName,
			&b.StartTime,
			&b.EndTime,
			&b.MemberName,
			&b.MemberEmail,
		); err != nil {
			return nil, fmt.Errorf("uye rezervasyon satiri okunamadi: %w", err)
		}
		b.Status = domain.BookingStatus(statusStr)
		list = append(list, b)
	}
	return list, nil
}

func (r *PostgresBookingRepository) ListBookingsBySchedule(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassBooking, error) {
	query := `
		SELECT 
			b.id, b.schedule_id, b.member_id, b.status, b.cancelled_at, b.penalty_applied, b.created_at, b.updated_at,
			c.name, br.name, u.first_name || ' ' || u.last_name, s.start_time, s.end_time,
			mem.first_name || ' ' || mem.last_name, mem.email
		FROM class_bookings b
		JOIN class_schedules s ON b.schedule_id = s.id
		JOIN classes c ON s.class_id = c.id
		JOIN branches br ON s.branch_id = br.id
		JOIN users u ON s.trainer_id = u.id
		JOIN users mem ON b.member_id = mem.id
		WHERE b.schedule_id = $1
		ORDER BY b.created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("seans rezervasyonlari listelenemedi: %w", err)
	}
	defer rows.Close()

	var list []domain.ClassBooking
	for rows.Next() {
		var b domain.ClassBooking
		var statusStr string
		if err := rows.Scan(
			&b.ID,
			&b.ScheduleID,
			&b.MemberID,
			&statusStr,
			&b.CancelledAt,
			&b.PenaltyApplied,
			&b.CreatedAt,
			&b.UpdatedAt,
			&b.ClassName,
			&b.BranchName,
			&b.TrainerName,
			&b.StartTime,
			&b.EndTime,
			&b.MemberName,
			&b.MemberEmail,
		); err != nil {
			return nil, fmt.Errorf("seans rezervasyon satiri okunamadi: %w", err)
		}
		b.Status = domain.BookingStatus(statusStr)
		list = append(list, b)
	}
	return list, nil
}

func (r *PostgresBookingRepository) UpdateBookingTx(ctx context.Context, tx pgx.Tx, b *domain.ClassBooking) error {
	query := `
		UPDATE class_bookings
		SET status = $2,
		    cancelled_at = $3,
		    penalty_applied = $4,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, b.ID, string(b.Status), b.CancelledAt, b.PenaltyApplied)
	if err != nil {
		return fmt.Errorf("rezervasyon guncellenemedi: %w", err)
	}
	return nil
}

// HasTimeConflictTx checks if a member already has another active (CONFIRMED) class overlapping this time window.
func (r *PostgresBookingRepository) HasTimeConflictTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID, startTime, endTime time.Time, excludeScheduleID *uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM class_bookings b
			JOIN class_schedules s ON b.schedule_id = s.id
			WHERE b.member_id = $1
			  AND b.status = 'CONFIRMED'
			  AND s.status = 'SCHEDULED'
			  AND s.start_time < $3
			  AND s.end_time > $2
			  AND ($4::uuid IS NULL OR s.id != $4)
		)
	`
	var conflict bool
	err := tx.QueryRow(ctx, query, memberID, startTime, endTime, excludeScheduleID).Scan(&conflict)
	if err != nil {
		return false, fmt.Errorf("zaman cakismasi kontrol edilemedi: %w", err)
	}
	return conflict, nil
}

// ==========================================
// Waitlist Operations
// ==========================================

func (r *PostgresBookingRepository) CreateOrUpdateWaitlistTx(ctx context.Context, tx pgx.Tx, w *domain.ClassWaitlist) error {
	query := `
		INSERT INTO class_waitlist (
			id, schedule_id, member_id, status, created_at, promoted_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (schedule_id, member_id)
		DO UPDATE SET
			status = EXCLUDED.status,
			created_at = EXCLUDED.created_at,
			promoted_at = EXCLUDED.promoted_at
		RETURNING id
	`
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now().UTC()
	}
	if w.Status == "" {
		w.Status = domain.WaitlistStatusWaiting
	}

	err := tx.QueryRow(ctx, query,
		w.ID,
		w.ScheduleID,
		w.MemberID,
		string(w.Status),
		w.CreatedAt,
		w.PromotedAt,
	).Scan(&w.ID)
	if err != nil {
		return fmt.Errorf("bekleme listesi kaydi olusturulamadi/guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresBookingRepository) GetWaitlistByID(ctx context.Context, id uuid.UUID) (*domain.ClassWaitlist, error) {
	query := `
		SELECT id, schedule_id, member_id, status, created_at, promoted_at
		FROM class_waitlist
		WHERE id = $1
	`
	var w domain.ClassWaitlist
	var statusStr string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&w.ID,
		&w.ScheduleID,
		&w.MemberID,
		&statusStr,
		&w.CreatedAt,
		&w.PromotedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrWaitlistNotFound
		}
		return nil, fmt.Errorf("bekleme listesi kaydi getirilemedi: %w", err)
	}
	w.Status = domain.WaitlistStatus(statusStr)
	return &w, nil
}

func (r *PostgresBookingRepository) GetWaitlistForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.ClassWaitlist, error) {
	query := `
		SELECT id, schedule_id, member_id, status, created_at, promoted_at
		FROM class_waitlist
		WHERE id = $1
		FOR UPDATE
	`
	var w domain.ClassWaitlist
	var statusStr string
	err := tx.QueryRow(ctx, query, id).Scan(
		&w.ID,
		&w.ScheduleID,
		&w.MemberID,
		&statusStr,
		&w.CreatedAt,
		&w.PromotedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrWaitlistNotFound
		}
		return nil, fmt.Errorf("bekleme listesi kilitlenemedi: %w", err)
	}
	w.Status = domain.WaitlistStatus(statusStr)
	return &w, nil
}

func (r *PostgresBookingRepository) GetWaitlistByMemberAndScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (*domain.ClassWaitlist, error) {
	query := `
		SELECT id, schedule_id, member_id, status, created_at, promoted_at
		FROM class_waitlist
		WHERE schedule_id = $1 AND member_id = $2
		FOR UPDATE
	`
	var w domain.ClassWaitlist
	var statusStr string
	err := tx.QueryRow(ctx, query, scheduleID, memberID).Scan(
		&w.ID,
		&w.ScheduleID,
		&w.MemberID,
		&statusStr,
		&w.CreatedAt,
		&w.PromotedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrWaitlistNotFound
		}
		return nil, fmt.Errorf("bekleme listesi kaydi sorgulanamadi: %w", err)
	}
	w.Status = domain.WaitlistStatus(statusStr)
	return &w, nil
}

// GetNextWaitingForUpdateSkipLockedTx consumes the next waiting member in FIFO order.
// Uses SELECT ... FOR UPDATE SKIP LOCKED as mandated by fitgiris-rules.
func (r *PostgresBookingRepository) GetNextWaitingForUpdateSkipLockedTx(ctx context.Context, tx pgx.Tx, scheduleID uuid.UUID) (*domain.ClassWaitlist, error) {
	query := `
		SELECT id, schedule_id, member_id, status, created_at, promoted_at
		FROM class_waitlist
		WHERE schedule_id = $1 AND status = 'WAITING'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`
	var w domain.ClassWaitlist
	var statusStr string
	err := tx.QueryRow(ctx, query, scheduleID).Scan(
		&w.ID,
		&w.ScheduleID,
		&w.MemberID,
		&statusStr,
		&w.CreatedAt,
		&w.PromotedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No waiting members found
		}
		return nil, fmt.Errorf("siradaki bekleme kaydi alinamadi (SKIP LOCKED): %w", err)
	}
	w.Status = domain.WaitlistStatus(statusStr)
	return &w, nil
}

func (r *PostgresBookingRepository) UpdateWaitlistTx(ctx context.Context, tx pgx.Tx, w *domain.ClassWaitlist) error {
	query := `
		UPDATE class_waitlist
		SET status = $2,
		    promoted_at = $3
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, w.ID, string(w.Status), w.PromotedAt)
	if err != nil {
		return fmt.Errorf("bekleme listesi kaydi guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresBookingRepository) GetWaitlistPositionTx(ctx context.Context, tx pgx.Tx, scheduleID, memberID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) + 1
		FROM class_waitlist
		WHERE schedule_id = $1
		  AND status = 'WAITING'
		  AND created_at < (
		      SELECT created_at FROM class_waitlist WHERE schedule_id = $1 AND member_id = $2
		  )
	`
	var position int
	err := tx.QueryRow(ctx, query, scheduleID, memberID).Scan(&position)
	if err != nil {
		return 0, fmt.Errorf("bekleme listesi sirasi alinamadi: %w", err)
	}
	return position, nil
}

func (r *PostgresBookingRepository) ListWaitlistBySchedule(ctx context.Context, scheduleID uuid.UUID) ([]domain.ClassWaitlist, error) {
	query := `
		SELECT 
			w.id, w.schedule_id, w.member_id, w.status, w.created_at, w.promoted_at,
			c.name, br.name, s.start_time, s.end_time,
			u.first_name || ' ' || u.last_name, u.email,
			(SELECT COUNT(*) + 1 FROM class_waitlist w2 WHERE w2.schedule_id = w.schedule_id AND w2.status = 'WAITING' AND w2.created_at < w.created_at) as position
		FROM class_waitlist w
		JOIN class_schedules s ON w.schedule_id = s.id
		JOIN classes c ON s.class_id = c.id
		JOIN branches br ON s.branch_id = br.id
		JOIN users u ON w.member_id = u.id
		WHERE w.schedule_id = $1
		ORDER BY w.created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("seans bekleme listesi alinamadi: %w", err)
	}
	defer rows.Close()

	var list []domain.ClassWaitlist
	for rows.Next() {
		var w domain.ClassWaitlist
		var statusStr string
		if err := rows.Scan(
			&w.ID,
			&w.ScheduleID,
			&w.MemberID,
			&statusStr,
			&w.CreatedAt,
			&w.PromotedAt,
			&w.ClassName,
			&w.BranchName,
			&w.StartTime,
			&w.EndTime,
			&w.MemberName,
			&w.MemberEmail,
			&w.Position,
		); err != nil {
			return nil, fmt.Errorf("bekleme listesi satiri okunamadi: %w", err)
		}
		w.Status = domain.WaitlistStatus(statusStr)
		list = append(list, w)
	}
	return list, nil
}

func (r *PostgresBookingRepository) ListWaitlistByMember(ctx context.Context, memberID uuid.UUID) ([]domain.ClassWaitlist, error) {
	query := `
		SELECT 
			w.id, w.schedule_id, w.member_id, w.status, w.created_at, w.promoted_at,
			c.name, br.name, s.start_time, s.end_time,
			u.first_name || ' ' || u.last_name, u.email,
			(SELECT COUNT(*) + 1 FROM class_waitlist w2 WHERE w2.schedule_id = w.schedule_id AND w2.status = 'WAITING' AND w2.created_at < w.created_at) as position
		FROM class_waitlist w
		JOIN class_schedules s ON w.schedule_id = s.id
		JOIN classes c ON s.class_id = c.id
		JOIN branches br ON s.branch_id = br.id
		JOIN users u ON w.member_id = u.id
		WHERE w.member_id = $1
		ORDER BY w.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, memberID)
	if err != nil {
		return nil, fmt.Errorf("uye bekleme listesi alinamadi: %w", err)
	}
	defer rows.Close()

	var list []domain.ClassWaitlist
	for rows.Next() {
		var w domain.ClassWaitlist
		var statusStr string
		if err := rows.Scan(
			&w.ID,
			&w.ScheduleID,
			&w.MemberID,
			&statusStr,
			&w.CreatedAt,
			&w.PromotedAt,
			&w.ClassName,
			&w.BranchName,
			&w.StartTime,
			&w.EndTime,
			&w.MemberName,
			&w.MemberEmail,
			&w.Position,
		); err != nil {
			return nil, fmt.Errorf("uye bekleme listesi satiri okunamadi: %w", err)
		}
		w.Status = domain.WaitlistStatus(statusStr)
		list = append(list, w)
	}
	return list, nil
}

func (r *PostgresBookingRepository) GetWaitlistCount(ctx context.Context, scheduleID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM class_waitlist
		WHERE schedule_id = $1 AND status = 'WAITING'
	`
	var count int
	err := r.pool.QueryRow(ctx, query, scheduleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("bekleme listesi sayisi alinamadi: %w", err)
	}
	return count, nil
}

// ==========================================
// Membership / Session Helpers in Tx
// ==========================================

func (r *PostgresBookingRepository) GetActiveMemberPackageTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID) (*domain.MemberPackage, error) {
	query := `
		SELECT 
			mp.id, mp.member_id, mp.package_type_id, mp.branch_id, mp.status,
			mp.start_date, mp.end_date, mp.remaining_entries, mp.total_entries,
			mp.price_paid_kurus, mp.auto_renew, mp.card_token, mp.consecutive_failed_attempts,
			mp.next_retry_date, mp.created_at, mp.updated_at,
			pt.name, pt.type, pt.all_branches, pt.max_freeze_days,
			b.name,
			u.first_name || ' ' || u.last_name, u.email
		FROM member_packages mp
		JOIN package_types pt ON mp.package_type_id = pt.id
		JOIN branches b ON mp.branch_id = b.id
		JOIN users u ON mp.member_id = u.id
		WHERE mp.member_id = $1
		  AND mp.status = 'ACTIVE'
		  AND mp.start_date <= CURRENT_DATE
		  AND mp.end_date >= CURRENT_DATE
		ORDER BY mp.end_date DESC
		LIMIT 1
		FOR UPDATE OF mp
	`
	var mp domain.MemberPackage
	var statusStr, pkgTypeStr string
	err := tx.QueryRow(ctx, query, memberID).Scan(
		&mp.ID,
		&mp.MemberID,
		&mp.PackageTypeID,
		&mp.BranchID,
		&statusStr,
		&mp.StartDate,
		&mp.EndDate,
		&mp.RemainingEntries,
		&mp.TotalEntries,
		&mp.PricePaidKurus,
		&mp.AutoRenew,
		&mp.CardToken,
		&mp.ConsecutiveFailedAttempts,
		&mp.NextRetryDate,
		&mp.CreatedAt,
		&mp.UpdatedAt,
		&mp.PackageName,
		&pkgTypeStr,
		&mp.AllBranches,
		&mp.MaxFreezeDays,
		&mp.BranchName,
		&mp.MemberName,
		&mp.MemberEmail,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoActiveMembership
		}
		return nil, fmt.Errorf("aktif uye paketi kilitlenemedi: %w", err)
	}

	mp.Status = domain.MembershipStatus(statusStr)
	mp.PackageKind = domain.PackageKind(pkgTypeStr)
	return &mp, nil
}

func (r *PostgresBookingRepository) DeductSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	query := `
		UPDATE member_packages
		SET remaining_entries = remaining_entries - 1, updated_at = NOW()
		WHERE id = $1 AND remaining_entries > 0
	`
	cmdTag, err := tx.Exec(ctx, query, packageID)
	if err != nil {
		return fmt.Errorf("seans giris hakki dusulemedi: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrInsufficientEntries
	}
	return nil
}

func (r *PostgresBookingRepository) RefundSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	query := `
		UPDATE member_packages
		SET remaining_entries = remaining_entries + 1, updated_at = NOW()
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, packageID)
	if err != nil {
		return fmt.Errorf("seans giris hakki iade edilemedi: %w", err)
	}
	return nil
}
