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

type PTRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)

	// Packages
	CreatePackage(ctx context.Context, pkg *domain.PTPackage) error
	CreatePackageTx(ctx context.Context, tx pgx.Tx, pkg *domain.PTPackage) error
	GetPackageByID(ctx context.Context, id uuid.UUID) (*domain.PTPackage, error)
	GetPackageForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.PTPackage, error)
	UpdatePackageTx(ctx context.Context, tx pgx.Tx, pkg *domain.PTPackage) error
	ListPackagesByMember(ctx context.Context, memberID uuid.UUID) ([]domain.PTPackage, error)
	ListPackagesByTrainer(ctx context.Context, trainerID uuid.UUID) ([]domain.PTPackage, error)

	// Appointments
	CreateAppointment(ctx context.Context, app *domain.PTAppointment) error
	CreateAppointmentTx(ctx context.Context, tx pgx.Tx, app *domain.PTAppointment) error
	GetAppointmentByID(ctx context.Context, id uuid.UUID) (*domain.PTAppointment, error)
	GetAppointmentForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.PTAppointment, error)
	UpdateAppointmentTx(ctx context.Context, tx pgx.Tx, app *domain.PTAppointment) error
	ListAppointmentsByMember(ctx context.Context, memberID uuid.UUID) ([]domain.PTAppointment, error)
	ListAppointmentsByTrainer(ctx context.Context, trainerID uuid.UUID, from, to *time.Time) ([]domain.PTAppointment, error)
	HasTrainerConflictTx(ctx context.Context, tx pgx.Tx, trainerID uuid.UUID, startTime, endTime time.Time, excludeID *uuid.UUID) (bool, error)
}

type postgresPTRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresPTRepository(pool *pgxpool.Pool) PTRepository {
	return &postgresPTRepository{pool: pool}
}

func (r *postgresPTRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *postgresPTRepository) CreatePackage(ctx context.Context, pkg *domain.PTPackage) error {
	query := `
		INSERT INTO pt_packages (id, member_id, trainer_id, total_sessions, remaining_sessions, price_kurus, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if pkg.ID == uuid.Nil {
		pkg.ID = uuid.New()
	}
	if pkg.CreatedAt.IsZero() {
		pkg.CreatedAt = time.Now()
	}

	_, err := r.pool.Exec(ctx, query,
		pkg.ID, pkg.MemberID, pkg.TrainerID, pkg.TotalSessions, pkg.RemainingSessions, pkg.PriceKurus, pkg.IsActive, pkg.CreatedAt,
	)
	return err
}

func (r *postgresPTRepository) CreatePackageTx(ctx context.Context, tx pgx.Tx, pkg *domain.PTPackage) error {
	query := `
		INSERT INTO pt_packages (id, member_id, trainer_id, total_sessions, remaining_sessions, price_kurus, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if pkg.ID == uuid.Nil {
		pkg.ID = uuid.New()
	}
	if pkg.CreatedAt.IsZero() {
		pkg.CreatedAt = time.Now()
	}

	_, err := tx.Exec(ctx, query,
		pkg.ID, pkg.MemberID, pkg.TrainerID, pkg.TotalSessions, pkg.RemainingSessions, pkg.PriceKurus, pkg.IsActive, pkg.CreatedAt,
	)
	return err
}

func (r *postgresPTRepository) GetPackageByID(ctx context.Context, id uuid.UUID) (*domain.PTPackage, error) {
	query := `
		SELECT p.id, p.member_id, p.trainer_id, p.total_sessions, p.remaining_sessions, p.price_kurus, p.is_active, p.created_at,
		       COALESCE(u_m.first_name || ' ' || u_m.last_name, '') as member_name,
		       COALESCE(u_t.first_name || ' ' || u_t.last_name, '') as trainer_name
		FROM pt_packages p
		LEFT JOIN users u_m ON p.member_id = u_m.id
		LEFT JOIN users u_t ON p.trainer_id = u_t.id
		WHERE p.id = $1
	`
	var pkg domain.PTPackage
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&pkg.ID, &pkg.MemberID, &pkg.TrainerID, &pkg.TotalSessions, &pkg.RemainingSessions,
		&pkg.PriceKurus, &pkg.IsActive, &pkg.CreatedAt, &pkg.MemberName, &pkg.TrainerName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPTNotFound
	}
	return &pkg, err
}

func (r *postgresPTRepository) GetPackageForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.PTPackage, error) {
	query := `
		SELECT id, member_id, trainer_id, total_sessions, remaining_sessions, price_kurus, is_active, created_at
		FROM pt_packages
		WHERE id = $1
		FOR UPDATE
	`
	var pkg domain.PTPackage
	err := tx.QueryRow(ctx, query, id).Scan(
		&pkg.ID, &pkg.MemberID, &pkg.TrainerID, &pkg.TotalSessions, &pkg.RemainingSessions,
		&pkg.PriceKurus, &pkg.IsActive, &pkg.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPTNotFound
	}
	return &pkg, err
}

func (r *postgresPTRepository) UpdatePackageTx(ctx context.Context, tx pgx.Tx, pkg *domain.PTPackage) error {
	query := `
		UPDATE pt_packages
		SET remaining_sessions = $2, is_active = $3
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, pkg.ID, pkg.RemainingSessions, pkg.IsActive)
	return err
}

func (r *postgresPTRepository) ListPackagesByMember(ctx context.Context, memberID uuid.UUID) ([]domain.PTPackage, error) {
	query := `
		SELECT p.id, p.member_id, p.trainer_id, p.total_sessions, p.remaining_sessions, p.price_kurus, p.is_active, p.created_at,
		       COALESCE(u_t.first_name || ' ' || u_t.last_name, '') as trainer_name
		FROM pt_packages p
		LEFT JOIN users u_t ON p.trainer_id = u_t.id
		WHERE p.member_id = $1
		ORDER BY p.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PTPackage
	for rows.Next() {
		var p domain.PTPackage
		if err := rows.Scan(
			&p.ID, &p.MemberID, &p.TrainerID, &p.TotalSessions, &p.RemainingSessions,
			&p.PriceKurus, &p.IsActive, &p.CreatedAt, &p.TrainerName,
		); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}

func (r *postgresPTRepository) ListPackagesByTrainer(ctx context.Context, trainerID uuid.UUID) ([]domain.PTPackage, error) {
	query := `
		SELECT p.id, p.member_id, p.trainer_id, p.total_sessions, p.remaining_sessions, p.price_kurus, p.is_active, p.created_at,
		       COALESCE(u_m.first_name || ' ' || u_m.last_name, '') as member_name
		FROM pt_packages p
		LEFT JOIN users u_m ON p.member_id = u_m.id
		WHERE p.trainer_id = $1
		ORDER BY p.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, trainerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PTPackage
	for rows.Next() {
		var p domain.PTPackage
		if err := rows.Scan(
			&p.ID, &p.MemberID, &p.TrainerID, &p.TotalSessions, &p.RemainingSessions,
			&p.PriceKurus, &p.IsActive, &p.CreatedAt, &p.MemberName,
		); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}

func (r *postgresPTRepository) CreateAppointment(ctx context.Context, app *domain.PTAppointment) error {
	query := `
		INSERT INTO pt_appointments (id, pt_package_id, member_id, trainer_id, branch_id, start_time, end_time, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	now := time.Now()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}
	app.UpdatedAt = now

	_, err := r.pool.Exec(ctx, query,
		app.ID, app.PTPackageID, app.MemberID, app.TrainerID, app.BranchID,
		app.StartTime, app.EndTime, app.Status, app.Notes, app.CreatedAt, app.UpdatedAt,
	)
	return err
}

func (r *postgresPTRepository) CreateAppointmentTx(ctx context.Context, tx pgx.Tx, app *domain.PTAppointment) error {
	query := `
		INSERT INTO pt_appointments (id, pt_package_id, member_id, trainer_id, branch_id, start_time, end_time, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	now := time.Now()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}
	app.UpdatedAt = now

	_, err := tx.Exec(ctx, query,
		app.ID, app.PTPackageID, app.MemberID, app.TrainerID, app.BranchID,
		app.StartTime, app.EndTime, app.Status, app.Notes, app.CreatedAt, app.UpdatedAt,
	)
	return err
}

func (r *postgresPTRepository) GetAppointmentByID(ctx context.Context, id uuid.UUID) (*domain.PTAppointment, error) {
	query := `
		SELECT a.id, a.pt_package_id, a.member_id, a.trainer_id, a.branch_id, a.start_time, a.end_time, a.status, COALESCE(a.notes, ''), a.created_at, a.updated_at,
		       COALESCE(u_m.first_name || ' ' || u_m.last_name, '') as member_name,
		       COALESCE(u_t.first_name || ' ' || u_t.last_name, '') as trainer_name,
		       COALESCE(b.name, '') as branch_name
		FROM pt_appointments a
		LEFT JOIN users u_m ON a.member_id = u_m.id
		LEFT JOIN users u_t ON a.trainer_id = u_t.id
		LEFT JOIN branches b ON a.branch_id = b.id
		WHERE a.id = $1
	`
	var app domain.PTAppointment
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&app.ID, &app.PTPackageID, &app.MemberID, &app.TrainerID, &app.BranchID,
		&app.StartTime, &app.EndTime, &app.Status, &app.Notes, &app.CreatedAt, &app.UpdatedAt,
		&app.MemberName, &app.TrainerName, &app.BranchName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPTNotFound
	}
	return &app, err
}

func (r *postgresPTRepository) GetAppointmentForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.PTAppointment, error) {
	query := `
		SELECT id, pt_package_id, member_id, trainer_id, branch_id, start_time, end_time, status, COALESCE(notes, ''), created_at, updated_at
		FROM pt_appointments
		WHERE id = $1
		FOR UPDATE
	`
	var app domain.PTAppointment
	err := tx.QueryRow(ctx, query, id).Scan(
		&app.ID, &app.PTPackageID, &app.MemberID, &app.TrainerID, &app.BranchID,
		&app.StartTime, &app.EndTime, &app.Status, &app.Notes, &app.CreatedAt, &app.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPTNotFound
	}
	return &app, err
}

func (r *postgresPTRepository) UpdateAppointmentTx(ctx context.Context, tx pgx.Tx, app *domain.PTAppointment) error {
	query := `
		UPDATE pt_appointments
		SET status = $2, notes = $3, updated_at = $4
		WHERE id = $1
	`
	app.UpdatedAt = time.Now()
	_, err := tx.Exec(ctx, query, app.ID, app.Status, app.Notes, app.UpdatedAt)
	return err
}

func (r *postgresPTRepository) ListAppointmentsByMember(ctx context.Context, memberID uuid.UUID) ([]domain.PTAppointment, error) {
	query := `
		SELECT a.id, a.pt_package_id, a.member_id, a.trainer_id, a.branch_id, a.start_time, a.end_time, a.status, COALESCE(a.notes, ''), a.created_at, a.updated_at,
		       COALESCE(u_t.first_name || ' ' || u_t.last_name, '') as trainer_name,
		       COALESCE(b.name, '') as branch_name
		FROM pt_appointments a
		LEFT JOIN users u_t ON a.trainer_id = u_t.id
		LEFT JOIN branches b ON a.branch_id = b.id
		WHERE a.member_id = $1
		ORDER BY a.start_time DESC
	`
	rows, err := r.pool.Query(ctx, query, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PTAppointment
	for rows.Next() {
		var app domain.PTAppointment
		if err := rows.Scan(
			&app.ID, &app.PTPackageID, &app.MemberID, &app.TrainerID, &app.BranchID,
			&app.StartTime, &app.EndTime, &app.Status, &app.Notes, &app.CreatedAt, &app.UpdatedAt,
			&app.TrainerName, &app.BranchName,
		); err != nil {
			return nil, err
		}
		list = append(list, app)
	}
	return list, nil
}

func (r *postgresPTRepository) ListAppointmentsByTrainer(ctx context.Context, trainerID uuid.UUID, from, to *time.Time) ([]domain.PTAppointment, error) {
	query := `
		SELECT a.id, a.pt_package_id, a.member_id, a.trainer_id, a.branch_id, a.start_time, a.end_time, a.status, COALESCE(a.notes, ''), a.created_at, a.updated_at,
		       COALESCE(u_m.first_name || ' ' || u_m.last_name, '') as member_name,
		       COALESCE(b.name, '') as branch_name
		FROM pt_appointments a
		LEFT JOIN users u_m ON a.member_id = u_m.id
		LEFT JOIN branches b ON a.branch_id = b.id
		WHERE a.trainer_id = $1
	`
	args := []interface{}{trainerID}
	argIdx := 2
	if from != nil {
		query += fmt.Sprintf(" AND a.start_time >= $%d", argIdx)
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		query += fmt.Sprintf(" AND a.end_time <= $%d", argIdx)
		args = append(args, *to)
		argIdx++
	}
	query += " ORDER BY a.start_time ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PTAppointment
	for rows.Next() {
		var app domain.PTAppointment
		if err := rows.Scan(
			&app.ID, &app.PTPackageID, &app.MemberID, &app.TrainerID, &app.BranchID,
			&app.StartTime, &app.EndTime, &app.Status, &app.Notes, &app.CreatedAt, &app.UpdatedAt,
			&app.MemberName, &app.BranchName,
		); err != nil {
			return nil, err
		}
		list = append(list, app)
	}
	return list, nil
}

func (r *postgresPTRepository) HasTrainerConflictTx(ctx context.Context, tx pgx.Tx, trainerID uuid.UUID, startTime, endTime time.Time, excludeID *uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM pt_appointments
			WHERE trainer_id = $1
			  AND status = 'CONFIRMED'
			  AND start_time < $3
			  AND end_time > $2
	`
	args := []interface{}{trainerID, startTime, endTime}
	if excludeID != nil {
		query += " AND id != $4"
		args = append(args, *excludeID)
	}
	query += ")"

	var exists bool
	err := tx.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}
