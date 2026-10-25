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

type BillingRepository interface {
	// Invoices & Sequential Counter
	GetBranchCodeTx(ctx context.Context, tx pgx.Tx, branchID uuid.UUID) (string, error)
	GetNextInvoiceSequenceTx(ctx context.Context, tx pgx.Tx, year int, branchID uuid.UUID) (int64, error)
	CreateInvoiceTx(ctx context.Context, tx pgx.Tx, inv *domain.Invoice) error
	GetInvoiceByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error)
	GetInvoiceByNumber(ctx context.Context, number string) (*domain.Invoice, error)
	ListInvoices(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.Invoice, int, error)
	UpdateInvoiceStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.InvoiceStatus) error

	// Payments
	CreatePaymentTx(ctx context.Context, tx pgx.Tx, p *domain.Payment) error
	GetPaymentByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	GetPaymentByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error)
	GetPaymentByIdempotencyKeyTx(ctx context.Context, tx pgx.Tx, key string) (*domain.Payment, error)
	ListPaymentsByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]domain.Payment, error)

	// Auto Renewal Queue
	EnqueueAutoRenewalTx(ctx context.Context, tx pgx.Tx, q *domain.AutoRenewalQueue) error
	GetPendingAutoRenewals(ctx context.Context, dueDate time.Time) ([]domain.AutoRenewalQueue, error)
	UpdateAutoRenewalTx(ctx context.Context, tx pgx.Tx, q *domain.AutoRenewalQueue) error
	ListAutoRenewals(ctx context.Context, status *domain.AutoRenewalStatus, limit, offset int) ([]domain.AutoRenewalQueue, int, error)

	WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type PostgresBillingRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresBillingRepository(pool *pgxpool.Pool) *PostgresBillingRepository {
	return &PostgresBillingRepository{pool: pool}
}

func (r *PostgresBillingRepository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("billing transaction baslatilamadi: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("billing transaction commit edilemedi: %w", err)
	}
	return nil
}

func (r *PostgresBillingRepository) GetBranchCodeTx(ctx context.Context, tx pgx.Tx, branchID uuid.UUID) (string, error) {
	var code string
	query := `SELECT code FROM branches WHERE id = $1`
	if err := tx.QueryRow(ctx, query, branchID).Scan(&code); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("sube bulunamadi")
		}
		return "", fmt.Errorf("sube kodu alinamadi: %w", err)
	}
	return code, nil
}

// GetNextInvoiceSequenceTx uses SELECT ... FOR UPDATE on the invoice_sequences row.
// This guarantees strictly gapless and duplicate-free numbers even under high concurrency.
func (r *PostgresBillingRepository) GetNextInvoiceSequenceTx(ctx context.Context, tx pgx.Tx, year int, branchID uuid.UUID) (int64, error) {
	// Ensure sequence row exists for the (year, branch_id) tuple
	initQuery := `
		INSERT INTO invoice_sequences (year, branch_id, current_val)
		VALUES ($1, $2, 0)
		ON CONFLICT (year, branch_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, initQuery, year, branchID); err != nil {
		return 0, fmt.Errorf("fatura sayaci baslatilamadi: %w", err)
	}

	// Pessimistically lock the counter row
	lockQuery := `
		SELECT current_val
		FROM invoice_sequences
		WHERE year = $1 AND branch_id = $2
		FOR UPDATE
	`
	var currentVal int64
	if err := tx.QueryRow(ctx, lockQuery, year, branchID).Scan(&currentVal); err != nil {
		return 0, fmt.Errorf("fatura sayaci kilitlenemedi: %w", err)
	}

	// Increment and return new sequence number
	nextVal := currentVal + 1
	updateQuery := `
		UPDATE invoice_sequences
		SET current_val = $3
		WHERE year = $1 AND branch_id = $2
	`
	if _, err := tx.Exec(ctx, updateQuery, year, branchID, nextVal); err != nil {
		return 0, fmt.Errorf("fatura sayaci artirilamadi: %w", err)
	}

	return nextVal, nil
}

func (r *PostgresBillingRepository) CreateInvoiceTx(ctx context.Context, tx pgx.Tx, inv *domain.Invoice) error {
	query := `
		INSERT INTO invoices (
			id, invoice_number, member_id, branch_id, member_package_id,
			year, sequence_number, amount_kurus, tax_kurus, status, payment_method, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = time.Now().UTC()
	}

	_, err := tx.Exec(ctx, query,
		inv.ID,
		inv.InvoiceNumber,
		inv.MemberID,
		inv.BranchID,
		inv.MemberPackageID,
		inv.Year,
		inv.SequenceNumber,
		inv.AmountKurus,
		inv.TaxKurus,
		string(inv.Status),
		inv.PaymentMethod,
		inv.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("fatura kaydedilemedi: %w", err)
	}
	return nil
}

func (r *PostgresBillingRepository) GetInvoiceByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	query := `
		SELECT 
			inv.id, inv.invoice_number, inv.member_id, inv.branch_id, inv.member_package_id,
			inv.year, inv.sequence_number, inv.amount_kurus, inv.tax_kurus, inv.status, inv.payment_method, inv.created_at,
			u.first_name || ' ' || u.last_name, u.email,
			b.name, b.code,
			COALESCE(pt.name, '')
		FROM invoices inv
		JOIN users u ON inv.member_id = u.id
		JOIN branches b ON inv.branch_id = b.id
		LEFT JOIN member_packages mp ON inv.member_package_id = mp.id
		LEFT JOIN package_types pt ON mp.package_type_id = pt.id
		WHERE inv.id = $1
	`
	var inv domain.Invoice
	var statusStr string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&inv.ID,
		&inv.InvoiceNumber,
		&inv.MemberID,
		&inv.BranchID,
		&inv.MemberPackageID,
		&inv.Year,
		&inv.SequenceNumber,
		&inv.AmountKurus,
		&inv.TaxKurus,
		&statusStr,
		&inv.PaymentMethod,
		&inv.CreatedAt,
		&inv.MemberName,
		&inv.MemberEmail,
		&inv.BranchName,
		&inv.BranchCode,
		&inv.PackageName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvoiceNotFound
		}
		return nil, fmt.Errorf("fatura getirilemedi: %w", err)
	}
	inv.Status = domain.InvoiceStatus(statusStr)
	return &inv, nil
}

func (r *PostgresBillingRepository) GetInvoiceByNumber(ctx context.Context, number string) (*domain.Invoice, error) {
	query := `
		SELECT 
			inv.id, inv.invoice_number, inv.member_id, inv.branch_id, inv.member_package_id,
			inv.year, inv.sequence_number, inv.amount_kurus, inv.tax_kurus, inv.status, inv.payment_method, inv.created_at,
			u.first_name || ' ' || u.last_name, u.email,
			b.name, b.code,
			COALESCE(pt.name, '')
		FROM invoices inv
		JOIN users u ON inv.member_id = u.id
		JOIN branches b ON inv.branch_id = b.id
		LEFT JOIN member_packages mp ON inv.member_package_id = mp.id
		LEFT JOIN package_types pt ON mp.package_type_id = pt.id
		WHERE inv.invoice_number = $1
	`
	var inv domain.Invoice
	var statusStr string

	err := r.pool.QueryRow(ctx, query, number).Scan(
		&inv.ID,
		&inv.InvoiceNumber,
		&inv.MemberID,
		&inv.BranchID,
		&inv.MemberPackageID,
		&inv.Year,
		&inv.SequenceNumber,
		&inv.AmountKurus,
		&inv.TaxKurus,
		&statusStr,
		&inv.PaymentMethod,
		&inv.CreatedAt,
		&inv.MemberName,
		&inv.MemberEmail,
		&inv.BranchName,
		&inv.BranchCode,
		&inv.PackageName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvoiceNotFound
		}
		return nil, fmt.Errorf("fatura numarasiyla getirilemedi: %w", err)
	}
	inv.Status = domain.InvoiceStatus(statusStr)
	return &inv, nil
}

func (r *PostgresBillingRepository) ListInvoices(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.Invoice, int, error) {
	baseWhere := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if branchID != nil {
		baseWhere += fmt.Sprintf(" AND inv.branch_id = $%d", argIdx)
		args = append(args, *branchID)
		argIdx++
	}
	if memberID != nil {
		baseWhere += fmt.Sprintf(" AND inv.member_id = $%d", argIdx)
		args = append(args, *memberID)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM invoices inv %s", baseWhere)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("toplam fatura sayisi alinamadi: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT 
			inv.id, inv.invoice_number, inv.member_id, inv.branch_id, inv.member_package_id,
			inv.year, inv.sequence_number, inv.amount_kurus, inv.tax_kurus, inv.status, inv.payment_method, inv.created_at,
			u.first_name || ' ' || u.last_name, u.email,
			b.name, b.code,
			COALESCE(pt.name, '')
		FROM invoices inv
		JOIN users u ON inv.member_id = u.id
		JOIN branches b ON inv.branch_id = b.id
		LEFT JOIN member_packages mp ON inv.member_package_id = mp.id
		LEFT JOIN package_types pt ON mp.package_type_id = pt.id
		%s
		ORDER BY inv.created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseWhere, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("faturalar listelenemedi: %w", err)
	}
	defer rows.Close()

	var list []domain.Invoice
	for rows.Next() {
		var inv domain.Invoice
		var statusStr string
		if err := rows.Scan(
			&inv.ID,
			&inv.InvoiceNumber,
			&inv.MemberID,
			&inv.BranchID,
			&inv.MemberPackageID,
			&inv.Year,
			&inv.SequenceNumber,
			&inv.AmountKurus,
			&inv.TaxKurus,
			&statusStr,
			&inv.PaymentMethod,
			&inv.CreatedAt,
			&inv.MemberName,
			&inv.MemberEmail,
			&inv.BranchName,
			&inv.BranchCode,
			&inv.PackageName,
		); err != nil {
			return nil, 0, fmt.Errorf("fatura kaydi okunamadi: %w", err)
		}
		inv.Status = domain.InvoiceStatus(statusStr)
		list = append(list, inv)
	}
	return list, total, nil
}

func (r *PostgresBillingRepository) UpdateInvoiceStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.InvoiceStatus) error {
	query := `
		UPDATE invoices
		SET status = $1
		WHERE id = $2
	`
	_, err := tx.Exec(ctx, query, string(status), id)
	if err != nil {
		return fmt.Errorf("fatura durumu guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresBillingRepository) CreatePaymentTx(ctx context.Context, tx pgx.Tx, p *domain.Payment) error {
	query := `
		INSERT INTO payments (
			id, invoice_id, member_id, amount_kurus, status, provider_tx_id,
			error_code, error_message, idempotency_key, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}

	_, err := tx.Exec(ctx, query,
		p.ID,
		p.InvoiceID,
		p.MemberID,
		p.AmountKurus,
		string(p.Status),
		p.ProviderTxID,
		p.ErrorCode,
		p.ErrorMessage,
		p.IdempotencyKey,
		p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("odeme kaydedilemedi: %w", err)
	}
	return nil
}

func (r *PostgresBillingRepository) GetPaymentByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	query := `
		SELECT id, invoice_id, member_id, amount_kurus, status, provider_tx_id,
		       error_code, error_message, idempotency_key, created_at
		FROM payments
		WHERE id = $1
	`
	var p domain.Payment
	var statusStr string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.InvoiceID,
		&p.MemberID,
		&p.AmountKurus,
		&statusStr,
		&p.ProviderTxID,
		&p.ErrorCode,
		&p.ErrorMessage,
		&p.IdempotencyKey,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("odeme bulunamadi")
		}
		return nil, fmt.Errorf("odeme getirilemedi: %w", err)
	}
	p.Status = domain.PaymentStatus(statusStr)
	return &p, nil
}

func (r *PostgresBillingRepository) GetPaymentByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	query := `
		SELECT id, invoice_id, member_id, amount_kurus, status, provider_tx_id,
		       error_code, error_message, idempotency_key, created_at
		FROM payments
		WHERE idempotency_key = $1
	`
	var p domain.Payment
	var statusStr string
	err := r.pool.QueryRow(ctx, query, key).Scan(
		&p.ID,
		&p.InvoiceID,
		&p.MemberID,
		&p.AmountKurus,
		&statusStr,
		&p.ProviderTxID,
		&p.ErrorCode,
		&p.ErrorMessage,
		&p.IdempotencyKey,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No existing payment with this idempotency key
		}
		return nil, fmt.Errorf("idempotency anahtari ile odeme sorgulanamadi: %w", err)
	}
	p.Status = domain.PaymentStatus(statusStr)
	return &p, nil
}

func (r *PostgresBillingRepository) GetPaymentByIdempotencyKeyTx(ctx context.Context, tx pgx.Tx, key string) (*domain.Payment, error) {
	query := `
		SELECT id, invoice_id, member_id, amount_kurus, status, provider_tx_id,
		       error_code, error_message, idempotency_key, created_at
		FROM payments
		WHERE idempotency_key = $1
		FOR UPDATE
	`
	var p domain.Payment
	var statusStr string
	err := tx.QueryRow(ctx, query, key).Scan(
		&p.ID,
		&p.InvoiceID,
		&p.MemberID,
		&p.AmountKurus,
		&statusStr,
		&p.ProviderTxID,
		&p.ErrorCode,
		&p.ErrorMessage,
		&p.IdempotencyKey,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("idempotency anahtari ile odeme sorgulanamadi (tx): %w", err)
	}
	p.Status = domain.PaymentStatus(statusStr)
	return &p, nil
}

func (r *PostgresBillingRepository) ListPaymentsByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]domain.Payment, error) {
	query := `
		SELECT id, invoice_id, member_id, amount_kurus, status, provider_tx_id,
		       error_code, error_message, idempotency_key, created_at
		FROM payments
		WHERE invoice_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("faturaya ait odemeler alinamadi: %w", err)
	}
	defer rows.Close()

	var list []domain.Payment
	for rows.Next() {
		var p domain.Payment
		var statusStr string
		if err := rows.Scan(
			&p.ID,
			&p.InvoiceID,
			&p.MemberID,
			&p.AmountKurus,
			&statusStr,
			&p.ProviderTxID,
			&p.ErrorCode,
			&p.ErrorMessage,
			&p.IdempotencyKey,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("odeme satiri okunamadi: %w", err)
		}
		p.Status = domain.PaymentStatus(statusStr)
		list = append(list, p)
	}
	return list, nil
}

// Auto Renewal Queue Operations

func (r *PostgresBillingRepository) EnqueueAutoRenewalTx(ctx context.Context, tx pgx.Tx, q *domain.AutoRenewalQueue) error {
	query := `
		INSERT INTO auto_renewal_queue (
			id, member_package_id, attempt_number, scheduled_for, status,
			last_attempt_at, error_reason, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	now := time.Now().UTC()
	q.CreatedAt = now
	q.UpdatedAt = now

	_, err := tx.Exec(ctx, query,
		q.ID,
		q.MemberPackageID,
		q.AttemptNumber,
		q.ScheduledFor,
		string(q.Status),
		q.LastAttemptAt,
		q.ErrorReason,
		q.CreatedAt,
		q.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("otomatik yenileme kuyruga eklenemedi: %w", err)
	}
	return nil
}

func (r *PostgresBillingRepository) GetPendingAutoRenewals(ctx context.Context, dueDate time.Time) ([]domain.AutoRenewalQueue, error) {
	query := `
		SELECT 
			q.id, q.member_package_id, q.attempt_number, q.scheduled_for, q.status,
			q.last_attempt_at, q.error_reason, q.created_at, q.updated_at,
			mp.member_id, u.first_name || ' ' || u.last_name, u.email,
			pt.name, pt.price_kurus, mp.card_token
		FROM auto_renewal_queue q
		JOIN member_packages mp ON q.member_package_id = mp.id
		JOIN users u ON mp.member_id = u.id
		JOIN package_types pt ON mp.package_type_id = pt.id
		WHERE q.status = 'PENDING' AND q.scheduled_for <= $1
		ORDER BY q.scheduled_for ASC, q.created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, dueDate)
	if err != nil {
		return nil, fmt.Errorf("bekleyen otomatik yenilemeler alinamadi: %w", err)
	}
	defer rows.Close()

	var list []domain.AutoRenewalQueue
	for rows.Next() {
		var q domain.AutoRenewalQueue
		var statusStr string
		if err := rows.Scan(
			&q.ID,
			&q.MemberPackageID,
			&q.AttemptNumber,
			&q.ScheduledFor,
			&statusStr,
			&q.LastAttemptAt,
			&q.ErrorReason,
			&q.CreatedAt,
			&q.UpdatedAt,
			&q.MemberID,
			&q.MemberName,
			&q.MemberEmail,
			&q.PackageName,
			&q.PriceKurus,
			&q.CardToken,
		); err != nil {
			return nil, fmt.Errorf("otomatik yenileme satir okunamadi: %w", err)
		}
		q.Status = domain.AutoRenewalStatus(statusStr)
		list = append(list, q)
	}
	return list, nil
}

func (r *PostgresBillingRepository) UpdateAutoRenewalTx(ctx context.Context, tx pgx.Tx, q *domain.AutoRenewalQueue) error {
	query := `
		UPDATE auto_renewal_queue
		SET attempt_number = $1, scheduled_for = $2, status = $3,
		    last_attempt_at = $4, error_reason = $5, updated_at = NOW()
		WHERE id = $6
	`
	_, err := tx.Exec(ctx, query,
		q.AttemptNumber,
		q.ScheduledFor,
		string(q.Status),
		q.LastAttemptAt,
		q.ErrorReason,
		q.ID,
	)
	if err != nil {
		return fmt.Errorf("otomatik yenileme kaydi guncellenemedi: %w", err)
	}
	return nil
}

func (r *PostgresBillingRepository) ListAutoRenewals(ctx context.Context, status *domain.AutoRenewalStatus, limit, offset int) ([]domain.AutoRenewalQueue, int, error) {
	baseWhere := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if status != nil {
		baseWhere += fmt.Sprintf(" AND q.status = $%d", argIdx)
		args = append(args, string(*status))
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM auto_renewal_queue q %s", baseWhere)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("toplam yenileme kuyrugu sayisi alinamadi: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT 
			q.id, q.member_package_id, q.attempt_number, q.scheduled_for, q.status,
			q.last_attempt_at, q.error_reason, q.created_at, q.updated_at,
			mp.member_id, u.first_name || ' ' || u.last_name, u.email,
			pt.name, pt.price_kurus, mp.card_token
		FROM auto_renewal_queue q
		JOIN member_packages mp ON q.member_package_id = mp.id
		JOIN users u ON mp.member_id = u.id
		JOIN package_types pt ON mp.package_type_id = pt.id
		%s
		ORDER BY q.created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseWhere, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("otomatik yenilemeler listelenemedi: %w", err)
	}
	defer rows.Close()

	var list []domain.AutoRenewalQueue
	for rows.Next() {
		var q domain.AutoRenewalQueue
		var statusStr string
		if err := rows.Scan(
			&q.ID,
			&q.MemberPackageID,
			&q.AttemptNumber,
			&q.ScheduledFor,
			&statusStr,
			&q.LastAttemptAt,
			&q.ErrorReason,
			&q.CreatedAt,
			&q.UpdatedAt,
			&q.MemberID,
			&q.MemberName,
			&q.MemberEmail,
			&q.PackageName,
			&q.PriceKurus,
			&q.CardToken,
		); err != nil {
			return nil, 0, fmt.Errorf("yenileme kaydi okunamadi: %w", err)
		}
		q.Status = domain.AutoRenewalStatus(statusStr)
		list = append(list, q)
	}
	return list, total, nil
}
