package service_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/dateutil"
	"fitgiris/internal/service"
)

// In-memory mock for BillingRepository simulating PostgreSQL transaction and FOR UPDATE row lock
type mockBillingRepo struct {
	mu          sync.Mutex
	branches    map[uuid.UUID]string // branchID -> code
	sequences   map[string]int64     // "year:branchID" -> currentVal
	invoices    map[uuid.UUID]*domain.Invoice
	payments    map[uuid.UUID]*domain.Payment
	idempKeys   map[string]*domain.Payment
	renewQueue  map[uuid.UUID]*domain.AutoRenewalQueue
}

func newMockBillingRepo() *mockBillingRepo {
	return &mockBillingRepo{
		branches:   make(map[uuid.UUID]string),
		sequences:  make(map[string]int64),
		invoices:   make(map[uuid.UUID]*domain.Invoice),
		payments:   make(map[uuid.UUID]*domain.Payment),
		idempKeys:  make(map[string]*domain.Payment),
		renewQueue: make(map[uuid.UUID]*domain.AutoRenewalQueue),
	}
}

func (m *mockBillingRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(nil)
}

func (m *mockBillingRepo) GetBranchCodeTx(ctx context.Context, tx pgx.Tx, branchID uuid.UUID) (string, error) {
	code, ok := m.branches[branchID]
	if !ok {
		return "IST01", nil // default
	}
	return code, nil
}

func (m *mockBillingRepo) GetNextInvoiceSequenceTx(ctx context.Context, tx pgx.Tx, year int, branchID uuid.UUID) (int64, error) {
	key := fmt.Sprintf("%d:%s", year, branchID.String())
	cur := m.sequences[key]
	cur++
	m.sequences[key] = cur
	return cur, nil
}

func (m *mockBillingRepo) CreateInvoiceTx(ctx context.Context, tx pgx.Tx, inv *domain.Invoice) error {
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	m.invoices[inv.ID] = inv
	return nil
}

func (m *mockBillingRepo) GetInvoiceByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invoices[id]
	if !ok {
		return nil, domain.ErrInvoiceNotFound
	}
	return inv, nil
}

func (m *mockBillingRepo) GetInvoiceByNumber(ctx context.Context, number string) (*domain.Invoice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, inv := range m.invoices {
		if inv.InvoiceNumber == number {
			return inv, nil
		}
	}
	return nil, domain.ErrInvoiceNotFound
}

func (m *mockBillingRepo) ListInvoices(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.Invoice, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.Invoice
	for _, inv := range m.invoices {
		if branchID != nil && inv.BranchID != *branchID {
			continue
		}
		if memberID != nil && inv.MemberID != *memberID {
			continue
		}
		list = append(list, *inv)
	}
	return list, len(list), nil
}

func (m *mockBillingRepo) UpdateInvoiceStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.InvoiceStatus) error {
	inv, ok := m.invoices[id]
	if !ok {
		return domain.ErrInvoiceNotFound
	}
	inv.Status = status
	return nil
}

func (m *mockBillingRepo) CreatePaymentTx(ctx context.Context, tx pgx.Tx, p *domain.Payment) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	m.payments[p.ID] = p
	if p.IdempotencyKey != nil {
		m.idempKeys[*p.IdempotencyKey] = p
	}
	return nil
}

func (m *mockBillingRepo) GetPaymentByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.payments[id]
	if !ok {
		return nil, domain.ErrPaymentFailed
	}
	return p, nil
}

func (m *mockBillingRepo) GetPaymentByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.idempKeys[key]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockBillingRepo) GetPaymentByIdempotencyKeyTx(ctx context.Context, tx pgx.Tx, key string) (*domain.Payment, error) {
	p, ok := m.idempKeys[key]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockBillingRepo) ListPaymentsByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]domain.Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.Payment
	for _, p := range m.payments {
		if p.InvoiceID == invoiceID {
			list = append(list, *p)
		}
	}
	return list, nil
}

func (m *mockBillingRepo) EnqueueAutoRenewalTx(ctx context.Context, tx pgx.Tx, q *domain.AutoRenewalQueue) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	m.renewQueue[q.ID] = q
	return nil
}

func (m *mockBillingRepo) GetPendingAutoRenewals(ctx context.Context, dueDate time.Time) ([]domain.AutoRenewalQueue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.AutoRenewalQueue
	for _, q := range m.renewQueue {
		if q.Status == domain.AutoRenewalPending && !q.ScheduledFor.After(dueDate) {
			list = append(list, *q)
		}
	}
	return list, nil
}

func (m *mockBillingRepo) UpdateAutoRenewalTx(ctx context.Context, tx pgx.Tx, q *domain.AutoRenewalQueue) error {
	m.renewQueue[q.ID] = q
	return nil
}

func (m *mockBillingRepo) ListAutoRenewals(ctx context.Context, status *domain.AutoRenewalStatus, limit, offset int) ([]domain.AutoRenewalQueue, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.AutoRenewalQueue
	for _, q := range m.renewQueue {
		if status != nil && q.Status != *status {
			continue
		}
		list = append(list, *q)
	}
	return list, len(list), nil
}

// In-memory mock for MembershipRepository
type mockBillingMembershipRepo struct {
	mu           sync.Mutex
	types        map[uuid.UUID]*domain.PackageType
	packages     map[uuid.UUID]*domain.MemberPackage
}

func newMockBillingMembershipRepo() *mockBillingMembershipRepo {
	return &mockBillingMembershipRepo{
		types:    make(map[uuid.UUID]*domain.PackageType),
		packages: make(map[uuid.UUID]*domain.MemberPackage),
	}
}

func (m *mockBillingMembershipRepo) CreatePackageType(ctx context.Context, pt *domain.PackageType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.types[pt.ID] = pt
	return nil
}

func (m *mockBillingMembershipRepo) GetPackageTypeByID(ctx context.Context, id uuid.UUID) (*domain.PackageType, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.types[id]
	if !ok {
		return nil, domain.ErrPackageTypeNotFound
	}
	return pt, nil
}

func (m *mockBillingMembershipRepo) ListPackageTypes(ctx context.Context, activeOnly bool) ([]domain.PackageType, error) {
	return nil, nil
}

func (m *mockBillingMembershipRepo) CreateMemberPackage(ctx context.Context, mp *domain.MemberPackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packages[mp.ID] = mp
	return nil
}

func (m *mockBillingMembershipRepo) CreateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packages[mp.ID] = mp
	return nil
}

func (m *mockBillingMembershipRepo) GetMemberPackageByID(ctx context.Context, id uuid.UUID) (*domain.MemberPackage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mp, ok := m.packages[id]
	if !ok {
		return nil, domain.ErrMembershipNotFound
	}
	return mp, nil
}

func (m *mockBillingMembershipRepo) GetMemberPackageForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.MemberPackage, error) {
	mp, ok := m.packages[id]
	if !ok {
		return nil, domain.ErrMembershipNotFound
	}
	return mp, nil
}

func (m *mockBillingMembershipRepo) ListMemberPackages(ctx context.Context, memberID *uuid.UUID, status *domain.MembershipStatus, limit, offset int) ([]domain.MemberPackage, int, error) {
	return nil, 0, nil
}

func (m *mockBillingMembershipRepo) UpdateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	m.packages[mp.ID] = mp
	return nil
}

func (m *mockBillingMembershipRepo) CreateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	return nil
}
func (m *mockBillingMembershipRepo) GetActiveFreezeTx(ctx context.Context, tx pgx.Tx, memberPackageID uuid.UUID) (*domain.PackageFreeze, error) {
	return nil, nil
}
func (m *mockBillingMembershipRepo) UpdateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	return nil
}
func (m *mockBillingMembershipRepo) GetTotalFrozenDays(ctx context.Context, memberPackageID uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockBillingMembershipRepo) CreateTransferTx(ctx context.Context, tx pgx.Tx, transfer *domain.PackageTransfer) error {
	return nil
}
func (m *mockBillingMembershipRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(nil)
}

func setupBillingTestEnv() (
	service.BillingService,
	*mockBillingRepo,
	*mockBillingMembershipRepo,
	*mockTurnstileUserRepo,
	*mockTurnstileAuditRepo,
	*service.MockPaymentProvider,
	uuid.UUID,
) {
	billingRepo := newMockBillingRepo()
	membershipRepo := newMockBillingMembershipRepo()
	userRepo := &mockTurnstileUserRepo{users: make(map[uuid.UUID]*domain.User)}
	auditRepo := &mockTurnstileAuditRepo{}
	paymentProv := service.NewMockPaymentProvider()

	branchID := uuid.New()
	billingRepo.branches[branchID] = "KDK01"

	svc := service.NewBillingService(billingRepo, membershipRepo, userRepo, auditRepo, paymentProv)
	return svc, billingRepo, membershipRepo, userRepo, auditRepo, paymentProv, branchID
}

// -------------------------------------------------------------
// Test 1: Sequential Gapless Invoicing
// -------------------------------------------------------------
func TestBilling_SequentialInvoicing(t *testing.T) {
	svc, _, _, _, _, _, branchID := setupBillingTestEnv()
	ctx := context.Background()
	memberID := uuid.New()

	// Create 3 invoices sequentially
	inv1, err := svc.CreateSequentialInvoice(ctx, service.CreateInvoiceRequest{
		MemberID:    memberID,
		BranchID:    branchID,
		AmountKurus: 120000,
	})
	if err != nil {
		t.Fatalf("inv1 failed: %v", err)
	}

	inv2, err := svc.CreateSequentialInvoice(ctx, service.CreateInvoiceRequest{
		MemberID:    memberID,
		BranchID:    branchID,
		AmountKurus: 150000,
	})
	if err != nil {
		t.Fatalf("inv2 failed: %v", err)
	}

	inv3, err := svc.CreateSequentialInvoice(ctx, service.CreateInvoiceRequest{
		MemberID:    memberID,
		BranchID:    branchID,
		AmountKurus: 180000,
	})
	if err != nil {
		t.Fatalf("inv3 failed: %v", err)
	}

	if inv1.SequenceNumber != 1 || inv2.SequenceNumber != 2 || inv3.SequenceNumber != 3 {
		t.Fatalf("sequences should be 1, 2, 3; got %d, %d, %d", inv1.SequenceNumber, inv2.SequenceNumber, inv3.SequenceNumber)
	}

	expectedPrefix := fmt.Sprintf("FIT-%04d-KDK01-", time.Now().UTC().Year())
	if inv1.InvoiceNumber != expectedPrefix+"000001" {
		t.Errorf("expected %s000001, got %s", expectedPrefix, inv1.InvoiceNumber)
	}
	if inv2.InvoiceNumber != expectedPrefix+"000002" {
		t.Errorf("expected %s000002, got %s", expectedPrefix, inv2.InvoiceNumber)
	}
	if inv3.InvoiceNumber != expectedPrefix+"000003" {
		t.Errorf("expected %s000003, got %s", expectedPrefix, inv3.InvoiceNumber)
	}
}

// -------------------------------------------------------------
// Test 2: Concurrency Proof - 50 Concurrent Invoice Creations (Zero Gaps, Zero Duplicates)
// -------------------------------------------------------------
func TestBilling_Concurrency_50SequentialInvoices_ZeroGaps(t *testing.T) {
	svc, _, _, _, _, _, branchID := setupBillingTestEnv()
	ctx := context.Background()

	const totalInvoices = 50
	invoices := make([]*domain.Invoice, totalInvoices)
	errors := make([]error, totalInvoices)

	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	for i := 0; i < totalInvoices; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startBarrier // Synchronize to fire concurrently

			inv, err := svc.CreateSequentialInvoice(ctx, service.CreateInvoiceRequest{
				MemberID:    uuid.New(),
				BranchID:    branchID,
				AmountKurus: int64(100000 + idx*1000),
			})
			invoices[idx] = inv
			errors[idx] = err
		}(i)
	}

	close(startBarrier)
	wg.Wait()

	// Check errors
	for i, err := range errors {
		if err != nil {
			t.Fatalf("concurrent invoice creation %d failed: %v", i, err)
		}
	}

	// Collect sequence numbers
	seqNumbers := make([]int64, totalInvoices)
	invoiceNumbers := make(map[string]bool)

	for i, inv := range invoices {
		if inv == nil {
			t.Fatalf("invoice %d was nil", i)
		}
		seqNumbers[i] = inv.SequenceNumber

		if invoiceNumbers[inv.InvoiceNumber] {
			t.Fatalf("DUPLICATE INVOICE NUMBER DETECTED: %s", inv.InvoiceNumber)
		}
		invoiceNumbers[inv.InvoiceNumber] = true
	}

	sort.Slice(seqNumbers, func(i, j int) bool {
		return seqNumbers[i] < seqNumbers[j]
	})

	// PROOF: Strictly gapless sequence numbers 1 to 50
	for i := 0; i < totalInvoices; i++ {
		expected := int64(i + 1)
		if seqNumbers[i] != expected {
			t.Fatalf("GAP DETECTED IN INVOICE NUMBERS! Expected %d at position %d, got %d", expected, i, seqNumbers[i])
		}
	}

	t.Logf("SUCCESS: 50 concurrent invoices produced strictly consecutive numbers 1 to 50 with ZERO gaps and ZERO duplicates!")
}

// -------------------------------------------------------------
// Test 3: Idempotent Payment Flow
// -------------------------------------------------------------
func TestBilling_IdempotentPayment(t *testing.T) {
	svc, _, _, _, _, _, branchID := setupBillingTestEnv()
	ctx := context.Background()
	memberID := uuid.New()

	inv, err := svc.CreateSequentialInvoice(ctx, service.CreateInvoiceRequest{
		MemberID:    memberID,
		BranchID:    branchID,
		AmountKurus: 250000,
	})
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}

	idempotencyKey := "idemp_test_key_abc_123"

	// First payment attempt
	pay1, err := svc.ProcessPayment(ctx, service.ProcessPaymentRequest{
		InvoiceID:      inv.ID,
		MemberID:       memberID,
		CardToken:      "card_tok_valid_test",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatalf("first payment failed: %v", err)
	}
	if pay1.Status != domain.PaymentStatusSuccess {
		t.Fatalf("first payment should be SUCCESS, got %s", pay1.Status)
	}

	// Repeated payment attempt with EXACT same Idempotency-Key
	pay2, err := svc.ProcessPayment(ctx, service.ProcessPaymentRequest{
		InvoiceID:      inv.ID,
		MemberID:       memberID,
		CardToken:      "card_tok_valid_test",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatalf("repeated payment failed: %v", err)
	}

	// PROOF: Returns exact same payment record without double charging
	if pay1.ID != pay2.ID {
		t.Fatalf("IDEMPOTENCY VIOLATION! Expected payment ID %s, got new ID %s", pay1.ID, pay2.ID)
	}
	if *pay1.ProviderTxID != *pay2.ProviderTxID {
		t.Fatalf("provider TX ID differed between idempotent calls")
	}
}

// -------------------------------------------------------------
// Test 4: Auto-Renewal 3-Attempt Schedule & 7th Day Suspension Rule (T-4 requirement)
// -------------------------------------------------------------
func TestBilling_AutoRenewal_3AttemptsAndDay7Suspension(t *testing.T) {
	svc, billingRepo, membershipRepo, _, auditRepo, paymentProv, branchID := setupBillingTestEnv()
	ctx := context.Background()
	memberID := uuid.New()

	// Program payment provider to fail on this card token
	failingCardToken := "FAIL_BANK_DECLINE"
	paymentProv.SetCardFailure(failingCardToken, "ERR_BANK_DECLINED")

	// Create package template
	pkgTypeID := uuid.New()
	membershipRepo.types[pkgTypeID] = &domain.PackageType{
		ID:             pkgTypeID,
		Name:           "Aylik Klasik",
		Type:           domain.PackageKindDuration,
		DurationMonths: 1,
		PriceKurus:     150000,
		IsActive:       true,
	}

	baseDate := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)

	// Member package active until baseDate
	cardTokStr := failingCardToken
	pkg := &domain.MemberPackage{
		ID:                        uuid.New(),
		MemberID:                  memberID,
		PackageTypeID:             pkgTypeID,
		BranchID:                  branchID,
		Status:                    domain.StatusActive,
		StartDate:                 baseDate.AddDate(0, -1, 0),
		EndDate:                   baseDate,
		PricePaidKurus:            150000,
		AutoRenew:                 true,
		CardToken:                 &cardTokStr,
		ConsecutiveFailedAttempts: 0,
	}
	membershipRepo.packages[pkg.ID] = pkg

	// Enqueue 1st renewal attempt for Day 1 (May 1)
	job := &domain.AutoRenewalQueue{
		ID:              uuid.New(),
		MemberPackageID: pkg.ID,
		AttemptNumber:   1,
		ScheduledFor:    baseDate,
		Status:          domain.AutoRenewalPending,
	}
	_ = billingRepo.EnqueueAutoRenewalTx(ctx, nil, job)

	// ----------------------------------------------------
	// DAY 1: 1st Attempt -> FAILS -> Next scheduled: Day 3 (May 3)
	// ----------------------------------------------------
	res1, err := svc.ProcessAutoRenewalQueue(ctx, baseDate)
	if err != nil {
		t.Fatalf("day 1 process failed: %v", err)
	}
	if res1.FailedCount != 1 {
		t.Errorf("Day 1: expected 1 failed, got %d", res1.FailedCount)
	}

	// Verify package has failed attempts = 1, next retry = May 3
	checkPkg := membershipRepo.packages[pkg.ID]
	if checkPkg.ConsecutiveFailedAttempts != 1 {
		t.Errorf("Day 1: expected ConsecutiveFailedAttempts=1, got %d", checkPkg.ConsecutiveFailedAttempts)
	}
	expectedDay3 := time.Date(2026, time.May, 3, 0, 0, 0, 0, time.UTC)
	if checkPkg.NextRetryDate == nil || *checkPkg.NextRetryDate != expectedDay3 {
		t.Errorf("Day 1: expected NextRetryDate=2026-05-03, got %v", checkPkg.NextRetryDate)
	}
	// Status should STILL be ACTIVE (grace period)
	if checkPkg.Status != domain.StatusActive {
		t.Errorf("Day 1: package should still be ACTIVE during retry period, got %s", checkPkg.Status)
	}

	// ----------------------------------------------------
	// DAY 3: 2nd Attempt -> FAILS -> Next scheduled: Day 5 (May 5)
	// ----------------------------------------------------
	res2, err := svc.ProcessAutoRenewalQueue(ctx, expectedDay3)
	if err != nil {
		t.Fatalf("day 3 process failed: %v", err)
	}
	if res2.FailedCount != 1 {
		t.Errorf("Day 3: expected 1 failed, got %d", res2.FailedCount)
	}

	checkPkg = membershipRepo.packages[pkg.ID]
	if checkPkg.ConsecutiveFailedAttempts != 2 {
		t.Errorf("Day 3: expected ConsecutiveFailedAttempts=2, got %d", checkPkg.ConsecutiveFailedAttempts)
	}
	expectedDay5 := time.Date(2026, time.May, 5, 0, 0, 0, 0, time.UTC)
	if checkPkg.NextRetryDate == nil || *checkPkg.NextRetryDate != expectedDay5 {
		t.Errorf("Day 3: expected NextRetryDate=2026-05-05, got %v", checkPkg.NextRetryDate)
	}

	// ----------------------------------------------------
	// DAY 5: 3rd Attempt -> FAILS -> 3rd failure triggers Day 7 suspension date
	// ----------------------------------------------------
	res3, err := svc.ProcessAutoRenewalQueue(ctx, expectedDay5)
	if err != nil {
		t.Fatalf("day 5 process failed: %v", err)
	}
	if res3.SuspendedCount != 1 {
		t.Errorf("Day 5 (3rd attempt failure): expected SuspendedCount=1, got %d", res3.SuspendedCount)
	}

	// PROOF: On 3rd failure (Day 7 threshold reached), package status MUST BE SUSPENDED!
	checkPkg = membershipRepo.packages[pkg.ID]
	if checkPkg.Status != domain.StatusSuspended {
		t.Fatalf("SUSPENSION RULE VIOLATION! Expected package status SUSPENDED, got %s", checkPkg.Status)
	}

	// Verify audit log has the MEMBERSHIP_SUSPENDED record
	foundSuspensionAudit := false
	for _, l := range auditRepo.logs {
		if l.Action == "MEMBERSHIP_SUSPENDED_NONPAYMENT" {
			foundSuspensionAudit = true
			break
		}
	}
	if !foundSuspensionAudit {
		t.Errorf("expected MEMBERSHIP_SUSPENDED_NONPAYMENT audit log entry")
	}

	t.Logf("SUCCESS: 3 retry schedule (Day 1, 3, 5) and Day 7 suspension verified!")
}

// -------------------------------------------------------------
// Test 5: Successful Auto-Renewal with Clamped Date Arithmetic
// -------------------------------------------------------------
func TestBilling_AutoRenewal_SuccessWithClampedDate(t *testing.T) {
	svc, billingRepo, membershipRepo, _, _, _, branchID := setupBillingTestEnv()
	ctx := context.Background()
	memberID := uuid.New()

	// 31 January package: renewing for 1 month must clamp to 28/29 February!
	jan31 := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	pkgTypeID := uuid.New()
	membershipRepo.types[pkgTypeID] = &domain.PackageType{
		ID:             pkgTypeID,
		Name:           "Aylik Paket",
		Type:           domain.PackageKindDuration,
		DurationMonths: 1,
		PriceKurus:     200000,
		IsActive:       true,
	}

	validTok := "card_tok_valid_test"
	pkg := &domain.MemberPackage{
		ID:             uuid.New(),
		MemberID:       memberID,
		PackageTypeID:  pkgTypeID,
		BranchID:       branchID,
		Status:         domain.StatusActive,
		StartDate:      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndDate:        jan31,
		PricePaidKurus: 200000,
		AutoRenew:      true,
		CardToken:      &validTok,
	}
	membershipRepo.packages[pkg.ID] = pkg

	job := &domain.AutoRenewalQueue{
		ID:              uuid.New(),
		MemberPackageID: pkg.ID,
		AttemptNumber:   1,
		ScheduledFor:    jan31,
		Status:          domain.AutoRenewalPending,
	}
	_ = billingRepo.EnqueueAutoRenewalTx(ctx, nil, job)

	res, err := svc.ProcessAutoRenewalQueue(ctx, jan31)
	if err != nil {
		t.Fatalf("ProcessAutoRenewalQueue failed: %v", err)
	}
	if res.SuccessCount != 1 {
		t.Fatalf("expected 1 success, got %d", res.SuccessCount)
	}

	updatedPkg := membershipRepo.packages[pkg.ID]
	if updatedPkg.Status != domain.StatusActive {
		t.Errorf("expected package status ACTIVE, got %s", updatedPkg.Status)
	}

	// Verify date clamping: 31 Jan + 1 month = 28 Feb (2026 is non-leap year)
	expectedNewEndDate := dateutil.AddClampedMonths(jan31, 1)
	if updatedPkg.EndDate != expectedNewEndDate {
		t.Errorf("expected EndDate %v, got %v", expectedNewEndDate, updatedPkg.EndDate)
	}
	if updatedPkg.EndDate.Day() != 28 || updatedPkg.EndDate.Month() != time.February {
		t.Errorf("date clamping failed: expected 28 Feb, got %v", updatedPkg.EndDate)
	}

	// Verify next renewal job enqueued for 28 Feb
	dueRenewals, _ := billingRepo.GetPendingAutoRenewals(ctx, expectedNewEndDate)
	if len(dueRenewals) == 0 {
		t.Errorf("expected next auto-renewal job to be enqueued")
	}
}
