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

type MembershipRepository interface {
	// Package Types
	CreatePackageType(ctx context.Context, pt *domain.PackageType) error
	GetPackageTypeByID(ctx context.Context, id uuid.UUID) (*domain.PackageType, error)
	ListPackageTypes(ctx context.Context, activeOnly bool) ([]domain.PackageType, error)

	// Member Packages
	CreateMemberPackage(ctx context.Context, mp *domain.MemberPackage) error
	CreateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error
	GetMemberPackageByID(ctx context.Context, id uuid.UUID) (*domain.MemberPackage, error)
	GetMemberPackageForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.MemberPackage, error)
	ListMemberPackages(ctx context.Context, memberID *uuid.UUID, status *domain.MembershipStatus, limit, offset int) ([]domain.MemberPackage, int, error)
	UpdateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error

	// Freezes
	CreateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error
	GetActiveFreezeTx(ctx context.Context, tx pgx.Tx, memberPackageID uuid.UUID) (*domain.PackageFreeze, error)
	UpdateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error
	GetTotalFrozenDays(ctx context.Context, memberPackageID uuid.UUID) (int, error)

	// Transfers
	CreateTransferTx(ctx context.Context, tx pgx.Tx, transfer *domain.PackageTransfer) error

	// Transaction helper
	WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type PostgresMembershipRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresMembershipRepository(pool *pgxpool.Pool) *PostgresMembershipRepository {
	return &PostgresMembershipRepository{pool: pool}
}

func (r *PostgresMembershipRepository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
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

// Package Types

func (r *PostgresMembershipRepository) CreatePackageType(ctx context.Context, pt *domain.PackageType) error {
	query := `
		INSERT INTO package_types (
			id, name, type, duration_months, total_entries, price_kurus, all_branches, max_freeze_days, is_active, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	if pt.ID == uuid.Nil {
		pt.ID = uuid.New()
	}
	if pt.CreatedAt.IsZero() {
		pt.CreatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, query,
		pt.ID,
		pt.Name,
		string(pt.Type),
		pt.DurationMonths,
		pt.TotalEntries,
		pt.PriceKurus,
		pt.AllBranches,
		pt.MaxFreezeDays,
		pt.IsActive,
		pt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("paket tipi olusturulamadi: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) GetPackageTypeByID(ctx context.Context, id uuid.UUID) (*domain.PackageType, error) {
	query := `
		SELECT id, name, type, duration_months, total_entries, price_kurus, all_branches, max_freeze_days, is_active, created_at
		FROM package_types
		WHERE id = $1
	`
	var pt domain.PackageType
	var typeStr string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&pt.ID,
		&pt.Name,
		&typeStr,
		&pt.DurationMonths,
		&pt.TotalEntries,
		&pt.PriceKurus,
		&pt.AllBranches,
		&pt.MaxFreezeDays,
		&pt.IsActive,
		&pt.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPackageTypeNotFound
		}
		return nil, fmt.Errorf("paket tipi getirilemedi: %w", err)
	}
	pt.Type = domain.PackageKind(typeStr)
	return &pt, nil
}

func (r *PostgresMembershipRepository) ListPackageTypes(ctx context.Context, activeOnly bool) ([]domain.PackageType, error) {
	query := `
		SELECT id, name, type, duration_months, total_entries, price_kurus, all_branches, max_freeze_days, is_active, created_at
		FROM package_types
	`
	if activeOnly {
		query += " WHERE is_active = true"
	}
	query += " ORDER BY price_kurus ASC"

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("paket tipleri listelenemedi: %w", err)
	}
	defer rows.Close()

	var list []domain.PackageType
	for rows.Next() {
		var pt domain.PackageType
		var typeStr string
		if err := rows.Scan(
			&pt.ID,
			&pt.Name,
			&typeStr,
			&pt.DurationMonths,
			&pt.TotalEntries,
			&pt.PriceKurus,
			&pt.AllBranches,
			&pt.MaxFreezeDays,
			&pt.IsActive,
			&pt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("paket tipi okunamadi: %w", err)
		}
		pt.Type = domain.PackageKind(typeStr)
		list = append(list, pt)
	}
	return list, nil
}

// Member Packages

func (r *PostgresMembershipRepository) CreateMemberPackage(ctx context.Context, mp *domain.MemberPackage) error {
	query := `
		INSERT INTO member_packages (
			id, member_id, package_type_id, branch_id, status, start_date, end_date,
			remaining_entries, total_entries, price_paid_kurus, auto_renew, card_token,
			consecutive_failed_attempts, next_retry_date, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	if mp.ID == uuid.Nil {
		mp.ID = uuid.New()
	}
	now := time.Now().UTC()
	mp.CreatedAt = now
	mp.UpdatedAt = now

	_, err := r.pool.Exec(ctx, query,
		mp.ID,
		mp.MemberID,
		mp.PackageTypeID,
		mp.BranchID,
		string(mp.Status),
		mp.StartDate,
		mp.EndDate,
		mp.RemainingEntries,
		mp.TotalEntries,
		mp.PricePaidKurus,
		mp.AutoRenew,
		mp.CardToken,
		mp.ConsecutiveFailedAttempts,
		mp.NextRetryDate,
		mp.CreatedAt,
		mp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("uye paketi olusturulamadi: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) CreateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	query := `
		INSERT INTO member_packages (
			id, member_id, package_type_id, branch_id, status, start_date, end_date,
			remaining_entries, total_entries, price_paid_kurus, auto_renew, card_token,
			consecutive_failed_attempts, next_retry_date, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	if mp.ID == uuid.Nil {
		mp.ID = uuid.New()
	}
	now := time.Now().UTC()
	mp.CreatedAt = now
	mp.UpdatedAt = now

	_, err := tx.Exec(ctx, query,
		mp.ID,
		mp.MemberID,
		mp.PackageTypeID,
		mp.BranchID,
		string(mp.Status),
		mp.StartDate,
		mp.EndDate,
		mp.RemainingEntries,
		mp.TotalEntries,
		mp.PricePaidKurus,
		mp.AutoRenew,
		mp.CardToken,
		mp.ConsecutiveFailedAttempts,
		mp.NextRetryDate,
		mp.CreatedAt,
		mp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("uye paketi (tx) olusturulamadi: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) GetMemberPackageByID(ctx context.Context, id uuid.UUID) (*domain.MemberPackage, error) {
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
		WHERE mp.id = $1
	`
	var mp domain.MemberPackage
	var statusStr, pkgTypeStr string

	err := r.pool.QueryRow(ctx, query, id).Scan(
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
		return nil, fmt.Errorf("uye paketi getirilemedi: %w", err)
	}

	mp.Status = domain.MembershipStatus(statusStr)
	mp.PackageKind = domain.PackageKind(pkgTypeStr)
	return &mp, nil
}

// GetMemberPackageForUpdate uses SELECT ... FOR UPDATE to pessimistically lock the membership row.
func (r *PostgresMembershipRepository) GetMemberPackageForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.MemberPackage, error) {
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
		WHERE mp.id = $1
		FOR UPDATE OF mp
	`
	var mp domain.MemberPackage
	var statusStr, pkgTypeStr string

	err := tx.QueryRow(ctx, query, id).Scan(
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
		return nil, fmt.Errorf("uye paketi kilitlenemedi: %w", err)
	}

	mp.Status = domain.MembershipStatus(statusStr)
	mp.PackageKind = domain.PackageKind(pkgTypeStr)
	return &mp, nil
}

func (r *PostgresMembershipRepository) ListMemberPackages(ctx context.Context, memberID *uuid.UUID, status *domain.MembershipStatus, limit, offset int) ([]domain.MemberPackage, int, error) {
	baseWhere := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if memberID != nil {
		baseWhere += fmt.Sprintf(" AND mp.member_id = $%d", argIdx)
		args = append(args, *memberID)
		argIdx++
	}
	if status != nil {
		baseWhere += fmt.Sprintf(" AND mp.status = $%d", argIdx)
		args = append(args, string(*status))
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM member_packages mp %s", baseWhere)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("toplam uye paketi sayisi alinamadi: %w", err)
	}

	query := fmt.Sprintf(`
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
		%s
		ORDER BY mp.created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseWhere, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("uye paketleri listelenemedi: %w", err)
	}
	defer rows.Close()

	var list []domain.MemberPackage
	for rows.Next() {
		var mp domain.MemberPackage
		var statusStr, pkgTypeStr string

		if err := rows.Scan(
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
		); err != nil {
			return nil, 0, fmt.Errorf("uye paketi satir okuma hatasi: %w", err)
		}
		mp.Status = domain.MembershipStatus(statusStr)
		mp.PackageKind = domain.PackageKind(pkgTypeStr)
		list = append(list, mp)
	}

	return list, total, nil
}

func (r *PostgresMembershipRepository) UpdateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	query := `
		UPDATE member_packages
		SET status = $2,
		    start_date = $3,
		    end_date = $4,
		    remaining_entries = $5,
		    total_entries = $6,
		    price_paid_kurus = $7,
		    auto_renew = $8,
		    card_token = $9,
		    consecutive_failed_attempts = $10,
		    next_retry_date = $11,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query,
		mp.ID,
		string(mp.Status),
		mp.StartDate,
		mp.EndDate,
		mp.RemainingEntries,
		mp.TotalEntries,
		mp.PricePaidKurus,
		mp.AutoRenew,
		mp.CardToken,
		mp.ConsecutiveFailedAttempts,
		mp.NextRetryDate,
	)
	if err != nil {
		return fmt.Errorf("uye paketi guncellenemedi: %w", err)
	}
	return nil
}

// Freezes

func (r *PostgresMembershipRepository) CreateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	query := `
		INSERT INTO package_freezes (
			id, member_package_id, freeze_start_date, freeze_end_date, actual_unfreeze_date, days_frozen, reason, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if freeze.ID == uuid.Nil {
		freeze.ID = uuid.New()
	}
	if freeze.CreatedAt.IsZero() {
		freeze.CreatedAt = time.Now().UTC()
	}

	_, err := tx.Exec(ctx, query,
		freeze.ID,
		freeze.MemberPackageID,
		freeze.FreezeStartDate,
		freeze.FreezeEndDate,
		freeze.ActualUnfreezeDate,
		freeze.DaysFrozen,
		freeze.Reason,
		freeze.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("dondurma kaydi olusturulamadi: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) GetActiveFreezeTx(ctx context.Context, tx pgx.Tx, memberPackageID uuid.UUID) (*domain.PackageFreeze, error) {
	query := `
		SELECT id, member_package_id, freeze_start_date, freeze_end_date, actual_unfreeze_date, days_frozen, reason, created_at
		FROM package_freezes
		WHERE member_package_id = $1 AND actual_unfreeze_date IS NULL
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`
	var f domain.PackageFreeze
	err := tx.QueryRow(ctx, query, memberPackageID).Scan(
		&f.ID,
		&f.MemberPackageID,
		&f.FreezeStartDate,
		&f.FreezeEndDate,
		&f.ActualUnfreezeDate,
		&f.DaysFrozen,
		&f.Reason,
		&f.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoActiveFreeze
		}
		return nil, fmt.Errorf("aktif dondurma kaydi getirilemedi: %w", err)
	}
	return &f, nil
}

func (r *PostgresMembershipRepository) UpdateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	query := `
		UPDATE package_freezes
		SET actual_unfreeze_date = $2,
		    days_frozen = $3
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, freeze.ID, freeze.ActualUnfreezeDate, freeze.DaysFrozen)
	if err != nil {
		return fmt.Errorf("dondurma kaydi guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) GetTotalFrozenDays(ctx context.Context, memberPackageID uuid.UUID) (int, error) {
	query := `
		SELECT COALESCE(SUM(days_frozen), 0)
		FROM package_freezes
		WHERE member_package_id = $1
	`
	var total int
	err := r.pool.QueryRow(ctx, query, memberPackageID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("toplam dondurulan gun sayisi hesaplanamadi: %w", err)
	}
	return total, nil
}

// Transfers

func (r *PostgresMembershipRepository) CreateTransferTx(ctx context.Context, tx pgx.Tx, transfer *domain.PackageTransfer) error {
	query := `
		INSERT INTO package_transfers (
			id, from_member_id, to_member_id, source_package_id, new_package_id, transferred_days, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if transfer.ID == uuid.Nil {
		transfer.ID = uuid.New()
	}
	if transfer.CreatedAt.IsZero() {
		transfer.CreatedAt = time.Now().UTC()
	}

	_, err := tx.Exec(ctx, query,
		transfer.ID,
		transfer.FromMemberID,
		transfer.ToMemberID,
		transfer.SourcePackageID,
		transfer.NewPackageID,
		transfer.TransferredDays,
		transfer.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("devir kaydi olusturulamadi: %w", err)
	}
	return nil
}
