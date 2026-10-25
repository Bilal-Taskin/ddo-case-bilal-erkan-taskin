package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/dateutil"
	"fitgiris/internal/repository"
)

type CreateInvoiceRequest struct {
	MemberID        uuid.UUID  `json:"member_id"`
	BranchID        uuid.UUID  `json:"branch_id"`
	MemberPackageID *uuid.UUID `json:"member_package_id,omitempty"`
	AmountKurus     int64      `json:"amount_kurus"`
	PaymentMethod   string     `json:"payment_method"`
}

type ProcessPaymentRequest struct {
	InvoiceID      uuid.UUID `json:"invoice_id"`
	MemberID       uuid.UUID `json:"member_id"`
	CardToken      string    `json:"card_token"`
	IdempotencyKey string    `json:"idempotency_key"`
}

type AutoRenewalBatchResult struct {
	TotalProcessed int `json:"total_processed"`
	SuccessCount   int `json:"success_count"`
	FailedCount    int `json:"failed_count"`
	SuspendedCount int `json:"suspended_count"`
}

type BillingService interface {
	CreateSequentialInvoice(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error)
	GetInvoiceByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error)
	GetInvoiceByNumber(ctx context.Context, number string) (*domain.Invoice, error)
	ListInvoices(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.Invoice, int, error)

	ProcessPayment(ctx context.Context, req ProcessPaymentRequest) (*domain.Payment, error)

	ProcessAutoRenewalQueue(ctx context.Context, currentDate time.Time) (*AutoRenewalBatchResult, error)
	EnqueueAutoRenewal(ctx context.Context, packageID uuid.UUID, scheduledDate time.Time) error
	ListAutoRenewals(ctx context.Context, status *domain.AutoRenewalStatus, limit, offset int) ([]domain.AutoRenewalQueue, int, error)
}

type billingService struct {
	billingRepo    repository.BillingRepository
	membershipRepo repository.MembershipRepository
	userRepo       repository.UserRepository
	auditRepo      repository.AuditRepository
	paymentProv    PaymentProvider
}

func NewBillingService(
	billingRepo repository.BillingRepository,
	membershipRepo repository.MembershipRepository,
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
	paymentProv PaymentProvider,
) BillingService {
	return &billingService{
		billingRepo:    billingRepo,
		membershipRepo: membershipRepo,
		userRepo:       userRepo,
		auditRepo:      auditRepo,
		paymentProv:    paymentProv,
	}
}

// CreateSequentialInvoice creates a new invoice with guaranteed zero gaps and zero duplicates.
// It locks the sequence row for (year, branch_id) using SELECT ... FOR UPDATE.
func (s *billingService) CreateSequentialInvoice(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error) {
	if req.AmountKurus < 0 {
		return nil, domain.ErrInvalidInvoiceAmount
	}

	year := time.Now().UTC().Year()
	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "CREDIT_CARD"
	}

	var invoice *domain.Invoice

	err := s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// 1. Get branch code
		branchCode, err := s.billingRepo.GetBranchCodeTx(ctx, tx, req.BranchID)
		if err != nil {
			return err
		}

		// 2. Fetch and increment gapless sequence with row-level lock
		seqNo, err := s.billingRepo.GetNextInvoiceSequenceTx(ctx, tx, year, req.BranchID)
		if err != nil {
			return err
		}

		invoiceNumber := domain.FormatInvoiceNumber(year, branchCode, seqNo)

		// 20% KDV calculation (standard Turkish VAT)
		taxKurus := (req.AmountKurus * 20) / 120

		invoice = &domain.Invoice{
			ID:              uuid.New(),
			InvoiceNumber:   invoiceNumber,
			MemberID:        req.MemberID,
			BranchID:        req.BranchID,
			MemberPackageID: req.MemberPackageID,
			Year:            year,
			SequenceNumber:  seqNo,
			AmountKurus:     req.AmountKurus,
			TaxKurus:        taxKurus,
			Status:          domain.InvoiceStatusPending,
			PaymentMethod:   paymentMethod,
			CreatedAt:       time.Now().UTC(),
		}

		return s.billingRepo.CreateInvoiceTx(ctx, tx, invoice)
	})

	if err != nil {
		return nil, fmt.Errorf("fatura olusturulamadi: %w", err)
	}

	// Fetch enriched invoice
	return s.billingRepo.GetInvoiceByID(ctx, invoice.ID)
}

func (s *billingService) GetInvoiceByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	return s.billingRepo.GetInvoiceByID(ctx, id)
}

func (s *billingService) GetInvoiceByNumber(ctx context.Context, number string) (*domain.Invoice, error) {
	return s.billingRepo.GetInvoiceByNumber(ctx, number)
}

func (s *billingService) ListInvoices(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.Invoice, int, error) {
	return s.billingRepo.ListInvoices(ctx, branchID, memberID, limit, offset)
}

// ProcessPayment handles card charging with strict idempotency.
// Repeated requests with the same Idempotency-Key return the exact existing payment record without double-charging.
func (s *billingService) ProcessPayment(ctx context.Context, req ProcessPaymentRequest) (*domain.Payment, error) {
	// 1. Idempotency Check: if key provided, verify whether already processed
	var idempKey *string
	if req.IdempotencyKey != "" {
		keyStr := req.IdempotencyKey
		idempKey = &keyStr
		existing, err := s.billingRepo.GetPaymentByIdempotencyKey(ctx, req.IdempotencyKey)
		if err == nil && existing != nil {
			slog.Info("Idempotent odeme yakalandi (cache hit)", "key", req.IdempotencyKey, "payment_id", existing.ID)
			return existing, nil
		}
	}

	// 2. Fetch and validate invoice
	invoice, err := s.billingRepo.GetInvoiceByID(ctx, req.InvoiceID)
	if err != nil {
		return nil, err
	}

	if invoice.Status == domain.InvoiceStatusPaid {
		// Invoice is already settled
		payments, err := s.billingRepo.ListPaymentsByInvoice(ctx, invoice.ID)
		if err == nil && len(payments) > 0 {
			for _, p := range payments {
				if p.Status == domain.PaymentStatusSuccess {
					return &p, nil
				}
			}
		}
		return nil, domain.ErrInvoiceAlreadyPaid
	}

	// 3. Perform Payment Provider Charge
	chargeResult, err := s.paymentProv.Charge(ctx, ChargeRequest{
		MemberID:       req.MemberID,
		AmountKurus:    invoice.AmountKurus,
		CardToken:      req.CardToken,
		IdempotencyKey: req.IdempotencyKey,
		Description:    fmt.Sprintf("FitGiris Fatura %s", invoice.InvoiceNumber),
	})
	if err != nil {
		return nil, fmt.Errorf("odeme saglayici hatasi: %w", err)
	}

	var paymentRecord *domain.Payment

	// 4. Record payment and update invoice inside transaction
	txErr := s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Concurrent check for idempotency key inside locked transaction
		if idempKey != nil {
			existingTx, err := s.billingRepo.GetPaymentByIdempotencyKeyTx(ctx, tx, *idempKey)
			if err == nil && existingTx != nil {
				paymentRecord = existingTx
				return nil
			}
		}

		if !chargeResult.Success {
			// Record failed payment
			errCode := chargeResult.ErrorCode
			errMsg := chargeResult.ErrorMessage
			paymentRecord = &domain.Payment{
				ID:             uuid.New(),
				InvoiceID:      invoice.ID,
				MemberID:       req.MemberID,
				AmountKurus:    invoice.AmountKurus,
				Status:         domain.PaymentStatusFailed,
				ErrorCode:      &errCode,
				ErrorMessage:   &errMsg,
				IdempotencyKey: idempKey,
				CreatedAt:      time.Now().UTC(),
			}
			if err := s.billingRepo.CreatePaymentTx(ctx, tx, paymentRecord); err != nil {
				return err
			}

			failPayload, _ := json.Marshal(map[string]interface{}{
				"invoice_id":   invoice.ID,
				"amount_kurus": invoice.AmountKurus,
				"error_code":   chargeResult.ErrorCode,
			})
			_ = s.auditRepo.Append(ctx, &domain.AuditLog{
				UserID:    &req.MemberID,
				Action:    "PAYMENT_FAILED",
				Entity:    "payments",
				EntityID:  paymentRecord.ID.String(),
				NewValues: failPayload,
				CreatedAt: time.Now().UTC(),
			})

			return domain.ErrPaymentFailed
		}

		// Payment SUCCESS
		txID := chargeResult.ProviderTxID
		paymentRecord = &domain.Payment{
			ID:             uuid.New(),
			InvoiceID:      invoice.ID,
			MemberID:       req.MemberID,
			AmountKurus:    invoice.AmountKurus,
			Status:         domain.PaymentStatusSuccess,
			ProviderTxID:   &txID,
			IdempotencyKey: idempKey,
			CreatedAt:      time.Now().UTC(),
		}

		if err := s.billingRepo.CreatePaymentTx(ctx, tx, paymentRecord); err != nil {
			return err
		}

		// Update invoice to PAID
		if err := s.billingRepo.UpdateInvoiceStatusTx(ctx, tx, invoice.ID, domain.InvoiceStatusPaid); err != nil {
			return err
		}

		// If invoice was for a member package in PENDING_PAYMENT or SUSPENDED, activate it
		if invoice.MemberPackageID != nil {
			mp, err := s.membershipRepo.GetMemberPackageForUpdate(ctx, tx, *invoice.MemberPackageID)
			if err == nil && mp != nil {
				if mp.Status == domain.StatusPendingPayment || mp.Status == domain.StatusSuspended {
					mp.Status = domain.StatusActive
					mp.ConsecutiveFailedAttempts = 0
					mp.NextRetryDate = nil
					_ = s.membershipRepo.UpdateMemberPackageTx(ctx, tx, mp)
				}
			}
		}

		successPayload, _ := json.Marshal(map[string]interface{}{
			"invoice_id":     invoice.ID,
			"amount_kurus":   invoice.AmountKurus,
			"provider_tx_id": chargeResult.ProviderTxID,
		})
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &req.MemberID,
			Action:    "PAYMENT_SUCCESS",
			Entity:    "payments",
			EntityID:  paymentRecord.ID.String(),
			NewValues: successPayload,
			CreatedAt: time.Now().UTC(),
		})

		return nil
	})

	if txErr != nil && txErr != domain.ErrPaymentFailed {
		return nil, txErr
	}

	return paymentRecord, txErr
}

// ProcessAutoRenewalQueue iterates through due recurring billing attempts:
// Implements 3 retry schedule (Day 1, Day 3, Day 5) and 7th day suspension rule.
func (s *billingService) ProcessAutoRenewalQueue(ctx context.Context, currentDate time.Time) (*AutoRenewalBatchResult, error) {
	result := &AutoRenewalBatchResult{}

	dueJobs, err := s.billingRepo.GetPendingAutoRenewals(ctx, currentDate)
	if err != nil {
		return nil, fmt.Errorf("bekleyen yenileme kuyrugu alinamadi: %w", err)
	}

	result.TotalProcessed = len(dueJobs)

	for _, job := range dueJobs {
		processErr := s.processSingleRenewal(ctx, job, currentDate, result)
		if processErr != nil {
			slog.Warn("Yenileme islemi hatasi", "job_id", job.ID, "error", processErr)
		}
	}

	return result, nil
}

func (s *billingService) processSingleRenewal(
	ctx context.Context,
	job domain.AutoRenewalQueue,
	currentDate time.Time,
	result *AutoRenewalBatchResult,
) error {
	nowUTC := time.Now().UTC()

	// 1. Check if package has 3 previous failed attempts and reached 7 days threshold
	// Attempt 1 fails -> Day 3 scheduled (attempt 2)
	// Attempt 2 fails -> Day 5 scheduled (attempt 3)
	// Attempt 3 fails -> Day 7 scheduled (attempt 4)
	if job.AttemptNumber >= 4 {
		// Suspension threshold!
		return s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
			mp, err := s.membershipRepo.GetMemberPackageForUpdate(ctx, tx, job.MemberPackageID)
			if err != nil {
				return err
			}

			// Transition to SUSPENDED
			mp.Status = domain.StatusSuspended
			mp.ConsecutiveFailedAttempts = job.AttemptNumber
			mp.NextRetryDate = nil
			if err := s.membershipRepo.UpdateMemberPackageTx(ctx, tx, mp); err != nil {
				return err
			}

			reason := "7 gun boyunca tahsilat basarisiz oldugu icin uyelik askiya alindi"
			job.Status = domain.AutoRenewalAbandoned
			job.LastAttemptAt = &nowUTC
			job.ErrorReason = &reason
			if err := s.billingRepo.UpdateAutoRenewalTx(ctx, tx, &job); err != nil {
				return err
			}

			result.SuspendedCount++

			suspendPayload, _ := json.Marshal(map[string]interface{}{
				"status":  domain.StatusSuspended,
				"reason":  reason,
				"attempt": job.AttemptNumber,
			})
			_ = s.auditRepo.Append(ctx, &domain.AuditLog{
				UserID:    &mp.MemberID,
				Action:    "MEMBERSHIP_SUSPENDED_NONPAYMENT",
				Entity:    "member_packages",
				EntityID:  mp.ID.String(),
				NewValues: suspendPayload,
				CreatedAt: nowUTC,
			})
			return nil
		})
	}

	// 2. Lookup package & package type
	mp, err := s.membershipRepo.GetMemberPackageByID(ctx, job.MemberPackageID)
	if err != nil || mp == nil {
		job.Status = domain.AutoRenewalAbandoned
		reason := "Uye paketi bulunamadi"
		job.ErrorReason = &reason
		_ = s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
			return s.billingRepo.UpdateAutoRenewalTx(ctx, tx, &job)
		})
		result.FailedCount++
		return nil
	}

	if mp.Status == domain.StatusCancelled || mp.Status == domain.StatusTransferred {
		job.Status = domain.AutoRenewalAbandoned
		reason := "Paket iptal edilmis veya devredilmis"
		job.ErrorReason = &reason
		_ = s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
			return s.billingRepo.UpdateAutoRenewalTx(ctx, tx, &job)
		})
		result.FailedCount++
		return nil
	}

	cardToken := ""
	if mp.CardToken != nil {
		cardToken = *mp.CardToken
	}
	if cardToken == "" {
		job.Status = domain.AutoRenewalAbandoned
		reason := "Kayitli kart tokeni bulunamadi"
		job.ErrorReason = &reason
		_ = s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
			return s.billingRepo.UpdateAutoRenewalTx(ctx, tx, &job)
		})
		result.FailedCount++
		return nil
	}

	pkgType, err := s.membershipRepo.GetPackageTypeByID(ctx, mp.PackageTypeID)
	if err != nil {
		result.FailedCount++
		return err
	}

	// Generate deterministic idempotency key for this renewal attempt
	idempKey := fmt.Sprintf("auto_renew_%s_attempt_%d_%s", job.MemberPackageID, job.AttemptNumber, currentDate.Format("20060102"))

	// 3. Attempt charge
	chargeResult, err := s.paymentProv.Charge(ctx, ChargeRequest{
		MemberID:       mp.MemberID,
		AmountKurus:    pkgType.PriceKurus,
		CardToken:      cardToken,
		IdempotencyKey: idempKey,
		Description:    fmt.Sprintf("FitGiris Otomatik Yenileme (%s)", pkgType.Name),
	})
	if err != nil || !chargeResult.Success {
		// CHARGE FAILED: Schedule next retry or suspend if on 3rd attempt
		nextRetryDate, canRetry := domain.CalculateNextRetry(job.AttemptNumber, currentDate)
		errMsg := "Banka provizyon hatasi"
		if chargeResult != nil && chargeResult.ErrorMessage != "" {
			errMsg = chargeResult.ErrorMessage
		}

		err = s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
			lockedPkg, err := s.membershipRepo.GetMemberPackageForUpdate(ctx, tx, mp.ID)
			if err != nil {
				return err
			}

			lockedPkg.ConsecutiveFailedAttempts = job.AttemptNumber
			lockedPkg.NextRetryDate = &nextRetryDate

			if !canRetry {
				// 3rd attempt failed -> 7th day reached -> Suspend immediately!
				lockedPkg.Status = domain.StatusSuspended
				job.Status = domain.AutoRenewalAbandoned
				result.SuspendedCount++

				suspendPayload, _ := json.Marshal(map[string]interface{}{
					"status":  domain.StatusSuspended,
					"reason":  errMsg,
					"attempt": job.AttemptNumber,
				})
				_ = s.auditRepo.Append(ctx, &domain.AuditLog{
					UserID:    &mp.MemberID,
					Action:    "MEMBERSHIP_SUSPENDED_NONPAYMENT",
					Entity:    "member_packages",
					EntityID:  mp.ID.String(),
					NewValues: suspendPayload,
					CreatedAt: nowUTC,
				})
			} else {
				job.AttemptNumber++
				job.ScheduledFor = nextRetryDate
				job.Status = domain.AutoRenewalPending
				result.FailedCount++
			}

			job.LastAttemptAt = &nowUTC
			job.ErrorReason = &errMsg

			if err := s.membershipRepo.UpdateMemberPackageTx(ctx, tx, lockedPkg); err != nil {
				return err
			}
			return s.billingRepo.UpdateAutoRenewalTx(ctx, tx, &job)
		})
		return err
	}

	// 4. CHARGE SUCCESSFUL: Create sequential invoice and advance membership dates
	err = s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// A. Get branch code & next sequence
		branchCode, err := s.billingRepo.GetBranchCodeTx(ctx, tx, mp.BranchID)
		if err != nil {
			return err
		}

		seqNo, err := s.billingRepo.GetNextInvoiceSequenceTx(ctx, tx, currentDate.Year(), mp.BranchID)
		if err != nil {
			return err
		}

		invoiceNumber := domain.FormatInvoiceNumber(currentDate.Year(), branchCode, seqNo)
		taxKurus := (pkgType.PriceKurus * 20) / 120

		inv := &domain.Invoice{
			ID:              uuid.New(),
			InvoiceNumber:   invoiceNumber,
			MemberID:        mp.MemberID,
			BranchID:        mp.BranchID,
			MemberPackageID: &mp.ID,
			Year:            currentDate.Year(),
			SequenceNumber:  seqNo,
			AmountKurus:     pkgType.PriceKurus,
			TaxKurus:        taxKurus,
			Status:          domain.InvoiceStatusPaid,
			PaymentMethod:   "AUTO_CREDIT_CARD",
			CreatedAt:       nowUTC,
		}
		if err := s.billingRepo.CreateInvoiceTx(ctx, tx, inv); err != nil {
			return err
		}

		// Record payment
		txID := chargeResult.ProviderTxID
		payment := &domain.Payment{
			ID:             uuid.New(),
			InvoiceID:      inv.ID,
			MemberID:       mp.MemberID,
			AmountKurus:    pkgType.PriceKurus,
			Status:         domain.PaymentStatusSuccess,
			ProviderTxID:   &txID,
			IdempotencyKey: &idempKey,
			CreatedAt:      nowUTC,
		}
		if err := s.billingRepo.CreatePaymentTx(ctx, tx, payment); err != nil {
			return err
		}

		// Advance membership package dates using Clamped Month arithmetic
		lockedPkg, err := s.membershipRepo.GetMemberPackageForUpdate(ctx, tx, mp.ID)
		if err != nil {
			return err
		}

		newStartDate := lockedPkg.EndDate
		newEndDate := dateutil.AddClampedMonths(lockedPkg.EndDate, pkgType.DurationMonths)
		lockedPkg.StartDate = newStartDate
		lockedPkg.EndDate = newEndDate
		lockedPkg.Status = domain.StatusActive
		lockedPkg.ConsecutiveFailedAttempts = 0
		lockedPkg.NextRetryDate = nil

		if pkgType.Type == domain.PackageKindSession {
			lockedPkg.RemainingEntries += pkgType.TotalEntries
			lockedPkg.TotalEntries += pkgType.TotalEntries
		}

		if err := s.membershipRepo.UpdateMemberPackageTx(ctx, tx, lockedPkg); err != nil {
			return err
		}

		// Mark current job as SUCCESS
		job.Status = domain.AutoRenewalSuccess
		job.LastAttemptAt = &nowUTC
		job.ErrorReason = nil
		if err := s.billingRepo.UpdateAutoRenewalTx(ctx, tx, &job); err != nil {
			return err
		}

		// Enqueue NEXT scheduled renewal for new end date
		nextJob := &domain.AutoRenewalQueue{
			ID:              uuid.New(),
			MemberPackageID: lockedPkg.ID,
			AttemptNumber:   1,
			ScheduledFor:    newEndDate,
			Status:          domain.AutoRenewalPending,
			CreatedAt:       nowUTC,
			UpdatedAt:       nowUTC,
		}
		if err := s.billingRepo.EnqueueAutoRenewalTx(ctx, tx, nextJob); err != nil {
			return err
		}

		result.SuccessCount++

		renewPayload, _ := json.Marshal(map[string]interface{}{
			"new_start_date": newStartDate,
			"new_end_date":   newEndDate,
			"invoice_number": invoiceNumber,
			"amount_kurus":   pkgType.PriceKurus,
		})
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &mp.MemberID,
			Action:    "PACKAGE_AUTO_RENEWED",
			Entity:    "member_packages",
			EntityID:  mp.ID.String(),
			NewValues: renewPayload,
			CreatedAt: nowUTC,
		})

		return nil
	})

	return err
}

func (s *billingService) EnqueueAutoRenewal(ctx context.Context, packageID uuid.UUID, scheduledDate time.Time) error {
	job := &domain.AutoRenewalQueue{
		ID:              uuid.New(),
		MemberPackageID: packageID,
		AttemptNumber:   1,
		ScheduledFor:    scheduledDate,
		Status:          domain.AutoRenewalPending,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	return s.billingRepo.WithTransaction(ctx, func(tx pgx.Tx) error {
		return s.billingRepo.EnqueueAutoRenewalTx(ctx, tx, job)
	})
}

func (s *billingService) ListAutoRenewals(ctx context.Context, status *domain.AutoRenewalStatus, limit, offset int) ([]domain.AutoRenewalQueue, int, error) {
	return s.billingRepo.ListAutoRenewals(ctx, status, limit, offset)
}
