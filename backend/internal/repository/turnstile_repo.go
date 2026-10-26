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

type TurnstileRepository interface {
	CreateTurnstile(ctx context.Context, t *domain.Turnstile) error
	GetTurnstileByID(ctx context.Context, id uuid.UUID) (*domain.Turnstile, error)
	GetTurnstileByBranchAndCode(ctx context.Context, branchID uuid.UUID, code string) (*domain.Turnstile, error)
	GetTurnstileByCode(ctx context.Context, code string) (*domain.Turnstile, error)
	ListTurnstiles(ctx context.Context, branchID *uuid.UUID, activeOnly bool) ([]domain.Turnstile, error)
	UpdateTurnstile(ctx context.Context, t *domain.Turnstile) error

	// Access Logs
	LogAccess(ctx context.Context, log *domain.AccessLog) error
	LogAccessBatch(ctx context.Context, logs []domain.AccessLog) error
	GetLastGrantedAccess(ctx context.Context, memberID, branchID uuid.UUID) (*domain.AccessLog, error)
	ListAccessLogs(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.AccessLog, int, error)

	// Member Package helpers for gate check
	GetActiveMemberPackageTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID) (*domain.MemberPackage, error)
	DeductSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error

	WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type PostgresTurnstileRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTurnstileRepository(pool *pgxpool.Pool) *PostgresTurnstileRepository {
	return &PostgresTurnstileRepository{pool: pool}
}

func (r *PostgresTurnstileRepository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("turnstile transaction baslatilamadi: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("turnstile transaction commit edilemedi: %w", err)
	}
	return nil
}

func (r *PostgresTurnstileRepository) CreateTurnstile(ctx context.Context, t *domain.Turnstile) error {
	query := `
		INSERT INTO turnstiles (id, branch_id, code, name, direction, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, query,
		t.ID,
		t.BranchID,
		t.Code,
		t.Name,
		string(t.Direction),
		t.IsActive,
		t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("turnike eklenemedi: %w", err)
	}
	return nil
}

func (r *PostgresTurnstileRepository) GetTurnstileByID(ctx context.Context, id uuid.UUID) (*domain.Turnstile, error) {
	query := `
		SELECT t.id, t.branch_id, t.code, t.name, t.direction, t.is_active, t.created_at,
		       b.name, b.code
		FROM turnstiles t
		JOIN branches b ON t.branch_id = b.id
		WHERE t.id = $1
	`
	var t domain.Turnstile
	var dirStr string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID,
		&t.BranchID,
		&t.Code,
		&t.Name,
		&dirStr,
		&t.IsActive,
		&t.CreatedAt,
		&t.BranchName,
		&t.BranchCode,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTurnstileNotFound
		}
		return nil, fmt.Errorf("turnike getirilemedi: %w", err)
	}
	t.Direction = domain.TurnstileDirection(dirStr)
	return &t, nil
}

func (r *PostgresTurnstileRepository) GetTurnstileByBranchAndCode(ctx context.Context, branchID uuid.UUID, code string) (*domain.Turnstile, error) {
	query := `
		SELECT t.id, t.branch_id, t.code, t.name, t.direction, t.is_active, t.created_at,
		       b.name, b.code
		FROM turnstiles t
		JOIN branches b ON t.branch_id = b.id
		WHERE t.branch_id = $1 AND t.code = $2
	`
	var t domain.Turnstile
	var dirStr string

	err := r.pool.QueryRow(ctx, query, branchID, code).Scan(
		&t.ID,
		&t.BranchID,
		&t.Code,
		&t.Name,
		&dirStr,
		&t.IsActive,
		&t.CreatedAt,
		&t.BranchName,
		&t.BranchCode,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTurnstileNotFound
		}
		return nil, fmt.Errorf("turnike sube ve kod ile getirilemedi: %w", err)
	}
	t.Direction = domain.TurnstileDirection(dirStr)
	return &t, nil
}

func (r *PostgresTurnstileRepository) GetTurnstileByCode(ctx context.Context, code string) (*domain.Turnstile, error) {
	query := `
		SELECT t.id, t.branch_id, t.code, t.name, t.direction, t.is_active, t.created_at,
		       b.name, b.code
		FROM turnstiles t
		JOIN branches b ON t.branch_id = b.id
		WHERE t.code = $1
		LIMIT 1
	`
	var t domain.Turnstile
	var dirStr string

	err := r.pool.QueryRow(ctx, query, code).Scan(
		&t.ID,
		&t.BranchID,
		&t.Code,
		&t.Name,
		&dirStr,
		&t.IsActive,
		&t.CreatedAt,
		&t.BranchName,
		&t.BranchCode,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTurnstileNotFound
		}
		return nil, fmt.Errorf("turnike kod ile getirilemedi: %w", err)
	}
	t.Direction = domain.TurnstileDirection(dirStr)
	return &t, nil
}

func (r *PostgresTurnstileRepository) ListTurnstiles(ctx context.Context, branchID *uuid.UUID, activeOnly bool) ([]domain.Turnstile, error) {
	query := `
		SELECT t.id, t.branch_id, t.code, t.name, t.direction, t.is_active, t.created_at,
		       b.name, b.code
		FROM turnstiles t
		JOIN branches b ON t.branch_id = b.id
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if branchID != nil {
		query += fmt.Sprintf(" AND t.branch_id = $%d", argIdx)
		args = append(args, *branchID)
	}
	if activeOnly {
		query += " AND t.is_active = true"
	}
	query += " ORDER BY t.code ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("turnikeler listelenemedi: %w", err)
	}
	defer rows.Close()

	var list []domain.Turnstile
	for rows.Next() {
		var t domain.Turnstile
		var dirStr string
		if err := rows.Scan(
			&t.ID,
			&t.BranchID,
			&t.Code,
			&t.Name,
			&dirStr,
			&t.IsActive,
			&t.CreatedAt,
			&t.BranchName,
			&t.BranchCode,
		); err != nil {
			return nil, fmt.Errorf("turnike kaydi okunamadi: %w", err)
		}
		t.Direction = domain.TurnstileDirection(dirStr)
		list = append(list, t)
	}
	return list, nil
}

func (r *PostgresTurnstileRepository) UpdateTurnstile(ctx context.Context, t *domain.Turnstile) error {
	query := `
		UPDATE turnstiles
		SET name = $1, direction = $2, is_active = $3
		WHERE id = $4
	`
	_, err := r.pool.Exec(ctx, query, t.Name, string(t.Direction), t.IsActive, t.ID)
	if err != nil {
		return fmt.Errorf("turnike guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresTurnstileRepository) LogAccess(ctx context.Context, log *domain.AccessLog) error {
	query := `
		INSERT INTO access_logs (id, member_id, turnstile_id, branch_id, direction, status, reason, offline_synced, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, query,
		log.ID,
		log.MemberID,
		log.TurnstileID,
		log.BranchID,
		string(log.Direction),
		string(log.Status),
		log.Reason,
		log.OfflineSynced,
		log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("gecis logu kaydedilemedi: %w", err)
	}
	return nil
}

func (r *PostgresTurnstileRepository) LogAccessBatch(ctx context.Context, logs []domain.AccessLog) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("batch log transaction baslatilamadi: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	stmt := `
		INSERT INTO access_logs (id, member_id, turnstile_id, branch_id, direction, status, reason, offline_synced, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	for i := range logs {
		l := &logs[i]
		if l.ID == uuid.Nil {
			l.ID = uuid.New()
		}
		if l.CreatedAt.IsZero() {
			l.CreatedAt = time.Now().UTC()
		}
		_, err := tx.Exec(ctx, stmt,
			l.ID,
			l.MemberID,
			l.TurnstileID,
			l.BranchID,
			string(l.Direction),
			string(l.Status),
			l.Reason,
			l.OfflineSynced,
			l.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("batch log kaydi eklenemedi: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetLastGrantedAccess uses the fast idx_access_logs_anti_passback index for sub-millisecond retrieval.
func (r *PostgresTurnstileRepository) GetLastGrantedAccess(ctx context.Context, memberID, branchID uuid.UUID) (*domain.AccessLog, error) {
	query := `
		SELECT id, member_id, turnstile_id, branch_id, direction, status, reason, offline_synced, created_at
		FROM access_logs
		WHERE member_id = $1 AND branch_id = $2 AND status = 'GRANTED'
		ORDER BY created_at DESC
		LIMIT 1
	`
	var l domain.AccessLog
	var dirStr, statusStr string

	err := r.pool.QueryRow(ctx, query, memberID, branchID).Scan(
		&l.ID,
		&l.MemberID,
		&l.TurnstileID,
		&l.BranchID,
		&dirStr,
		&statusStr,
		&l.Reason,
		&l.OfflineSynced,
		&l.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No prior access record
		}
		return nil, fmt.Errorf("son gecis alinamadi: %w", err)
	}
	l.Direction = domain.TurnstileDirection(dirStr)
	l.Status = domain.AccessStatus(statusStr)
	return &l, nil
}

func (r *PostgresTurnstileRepository) ListAccessLogs(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.AccessLog, int, error) {
	baseWhere := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if branchID != nil {
		baseWhere += fmt.Sprintf(" AND al.branch_id = $%d", argIdx)
		args = append(args, *branchID)
		argIdx++
	}
	if memberID != nil {
		baseWhere += fmt.Sprintf(" AND al.member_id = $%d", argIdx)
		args = append(args, *memberID)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM access_logs al %s", baseWhere)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("toplam log sayisi alinamadi: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT al.id, al.member_id, al.turnstile_id, al.branch_id, al.direction,
		       al.status, al.reason, al.offline_synced, al.created_at,
		       u.first_name || ' ' || u.last_name, u.email,
		       t.name, t.code, b.name
		FROM access_logs al
		JOIN users u ON al.member_id = u.id
		JOIN turnstiles t ON al.turnstile_id = t.id
		JOIN branches b ON al.branch_id = b.id
		%s
		ORDER BY al.created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseWhere, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("gecis loglari listelenemedi: %w", err)
	}
	defer rows.Close()

	var list []domain.AccessLog
	for rows.Next() {
		var l domain.AccessLog
		var dirStr, statusStr string
		if err := rows.Scan(
			&l.ID,
			&l.MemberID,
			&l.TurnstileID,
			&l.BranchID,
			&dirStr,
			&statusStr,
			&l.Reason,
			&l.OfflineSynced,
			&l.CreatedAt,
			&l.MemberName,
			&l.MemberEmail,
			&l.TurnstileName,
			&l.TurnstileCode,
			&l.BranchName,
		); err != nil {
			return nil, 0, fmt.Errorf("gecis logu okunamadi: %w", err)
		}
		l.Direction = domain.TurnstileDirection(dirStr)
		l.Status = domain.AccessStatus(statusStr)
		list = append(list, l)
	}
	return list, total, nil
}

// GetActiveMemberPackageTx finds the active package for turnstile entry and locks it.
func (r *PostgresTurnstileRepository) GetActiveMemberPackageTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID) (*domain.MemberPackage, error) {
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
			return nil, domain.ErrMembershipNotFound
		}
		return nil, fmt.Errorf("aktif paket getirilemedi: %w", err)
	}

	mp.Status = domain.MembershipStatus(statusStr)
	mp.PackageKind = domain.PackageKind(pkgTypeStr)
	return &mp, nil
}

func (r *PostgresTurnstileRepository) DeductSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	query := `
		UPDATE member_packages
		SET remaining_entries = remaining_entries - 1, updated_at = NOW()
		WHERE id = $1 AND remaining_entries > 0
	`
	cmdTag, err := tx.Exec(ctx, query, packageID)
	if err != nil {
		return fmt.Errorf("turnike girisinde giris hakki dusulemedi: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrInsufficientEntries
	}
	return nil
}
