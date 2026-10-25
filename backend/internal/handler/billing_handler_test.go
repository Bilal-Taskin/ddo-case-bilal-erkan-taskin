package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

type mockHndBillingRepo struct {
	mu        sync.Mutex
	branches  map[uuid.UUID]string
	sequences map[string]int64
	invoices  map[uuid.UUID]*domain.Invoice
	payments  map[uuid.UUID]*domain.Payment
	idempKeys map[string]*domain.Payment
	queue     map[uuid.UUID]*domain.AutoRenewalQueue
}

func newMockHndBillingRepo() *mockHndBillingRepo {
	return &mockHndBillingRepo{
		branches:  make(map[uuid.UUID]string),
		sequences: make(map[string]int64),
		invoices:  make(map[uuid.UUID]*domain.Invoice),
		payments:  make(map[uuid.UUID]*domain.Payment),
		idempKeys: make(map[string]*domain.Payment),
		queue:     make(map[uuid.UUID]*domain.AutoRenewalQueue),
	}
}

func (m *mockHndBillingRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(nil)
}

func (m *mockHndBillingRepo) GetBranchCodeTx(ctx context.Context, tx pgx.Tx, branchID uuid.UUID) (string, error) {
	code, ok := m.branches[branchID]
	if !ok {
		return "IST01", nil
	}
	return code, nil
}

func (m *mockHndBillingRepo) GetNextInvoiceSequenceTx(ctx context.Context, tx pgx.Tx, year int, branchID uuid.UUID) (int64, error) {
	key := branchID.String()
	m.sequences[key]++
	return m.sequences[key], nil
}

func (m *mockHndBillingRepo) CreateInvoiceTx(ctx context.Context, tx pgx.Tx, inv *domain.Invoice) error {
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	m.invoices[inv.ID] = inv
	return nil
}

func (m *mockHndBillingRepo) GetInvoiceByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invoices[id]
	if !ok {
		return nil, domain.ErrInvoiceNotFound
	}
	return inv, nil
}

func (m *mockHndBillingRepo) GetInvoiceByNumber(ctx context.Context, number string) (*domain.Invoice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, inv := range m.invoices {
		if inv.InvoiceNumber == number {
			return inv, nil
		}
	}
	return nil, domain.ErrInvoiceNotFound
}

func (m *mockHndBillingRepo) ListInvoices(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.Invoice, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.Invoice
	for _, inv := range m.invoices {
		if memberID != nil && inv.MemberID != *memberID {
			continue
		}
		list = append(list, *inv)
	}
	return list, len(list), nil
}

func (m *mockHndBillingRepo) UpdateInvoiceStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.InvoiceStatus) error {
	if inv, ok := m.invoices[id]; ok {
		inv.Status = status
	}
	return nil
}

func (m *mockHndBillingRepo) CreatePaymentTx(ctx context.Context, tx pgx.Tx, p *domain.Payment) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	m.payments[p.ID] = p
	if p.IdempotencyKey != nil {
		m.idempKeys[*p.IdempotencyKey] = p
	}
	return nil
}

func (m *mockHndBillingRepo) GetPaymentByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.payments[id]
	if !ok {
		return nil, domain.ErrPaymentFailed
	}
	return p, nil
}

func (m *mockHndBillingRepo) GetPaymentByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.idempKeys[key], nil
}

func (m *mockHndBillingRepo) GetPaymentByIdempotencyKeyTx(ctx context.Context, tx pgx.Tx, key string) (*domain.Payment, error) {
	return m.idempKeys[key], nil
}

func (m *mockHndBillingRepo) ListPaymentsByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]domain.Payment, error) {
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

func (m *mockHndBillingRepo) EnqueueAutoRenewalTx(ctx context.Context, tx pgx.Tx, q *domain.AutoRenewalQueue) error {
	m.queue[q.ID] = q
	return nil
}

func (m *mockHndBillingRepo) GetPendingAutoRenewals(ctx context.Context, dueDate time.Time) ([]domain.AutoRenewalQueue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.AutoRenewalQueue
	for _, q := range m.queue {
		if q.Status == domain.AutoRenewalPending {
			list = append(list, *q)
		}
	}
	return list, nil
}

func (m *mockHndBillingRepo) UpdateAutoRenewalTx(ctx context.Context, tx pgx.Tx, q *domain.AutoRenewalQueue) error {
	m.queue[q.ID] = q
	return nil
}

func (m *mockHndBillingRepo) ListAutoRenewals(ctx context.Context, status *domain.AutoRenewalStatus, limit, offset int) ([]domain.AutoRenewalQueue, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.AutoRenewalQueue
	for _, q := range m.queue {
		list = append(list, *q)
	}
	return list, len(list), nil
}

type mockHndMembershipRepo struct{}

func (m *mockHndMembershipRepo) CreatePackageType(ctx context.Context, pt *domain.PackageType) error {
	return nil
}
func (m *mockHndMembershipRepo) GetPackageTypeByID(ctx context.Context, id uuid.UUID) (*domain.PackageType, error) {
	return nil, nil
}
func (m *mockHndMembershipRepo) ListPackageTypes(ctx context.Context, activeOnly bool) ([]domain.PackageType, error) {
	return nil, nil
}
func (m *mockHndMembershipRepo) CreateMemberPackage(ctx context.Context, mp *domain.MemberPackage) error {
	return nil
}
func (m *mockHndMembershipRepo) CreateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	return nil
}
func (m *mockHndMembershipRepo) GetMemberPackageByID(ctx context.Context, id uuid.UUID) (*domain.MemberPackage, error) {
	return nil, nil
}
func (m *mockHndMembershipRepo) GetMemberPackageForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.MemberPackage, error) {
	return nil, nil
}
func (m *mockHndMembershipRepo) ListMemberPackages(ctx context.Context, memberID *uuid.UUID, status *domain.MembershipStatus, limit, offset int) ([]domain.MemberPackage, int, error) {
	return nil, 0, nil
}
func (m *mockHndMembershipRepo) UpdateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	return nil
}
func (m *mockHndMembershipRepo) CreateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	return nil
}
func (m *mockHndMembershipRepo) GetActiveFreezeTx(ctx context.Context, tx pgx.Tx, memberPackageID uuid.UUID) (*domain.PackageFreeze, error) {
	return nil, nil
}
func (m *mockHndMembershipRepo) UpdateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	return nil
}
func (m *mockHndMembershipRepo) GetTotalFrozenDays(ctx context.Context, memberPackageID uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockHndMembershipRepo) CreateTransferTx(ctx context.Context, tx pgx.Tx, transfer *domain.PackageTransfer) error {
	return nil
}
func (m *mockHndMembershipRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

func setupBillingHandlerRouter() (*chi.Mux, *mockHndBillingRepo, *security.JWTManager, uuid.UUID) {
	billingRepo := newMockHndBillingRepo()
	membershipRepo := &mockHndMembershipRepo{}
	userRepo := &mockHndUserRepo{users: make(map[uuid.UUID]*domain.User)}
	auditRepo := &mockHndAuditRepo{}
	paymentProv := service.NewMockPaymentProvider()

	branchID := uuid.New()
	billingRepo.branches[branchID] = "IST01"

	jwtMgr := security.NewJWTManager("test_secret_for_billing_handler_tests_key!")
	authMw := middleware.NewAuthMiddleware(jwtMgr)

	svc := service.NewBillingService(billingRepo, membershipRepo, userRepo, auditRepo, paymentProv)
	h := handler.NewBillingHandler(svc)

	r := chi.NewRouter()
	r.Route("/api/v1/billing", func(r chi.Router) {
		r.Use(authMw.Authenticate)

		r.Get("/invoices", h.ListInvoices)
		r.Get("/invoices/{id}", h.GetInvoice)
		r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleAccountant)).Post("/invoices", h.CreateInvoice)
		r.Post("/invoices/{id}/pay", h.PayInvoice)
		r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleAccountant)).Post("/auto-renewals/process", h.ProcessAutoRenewals)
		r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleAccountant)).Get("/auto-renewals", h.ListAutoRenewals)
	})

	return r, billingRepo, jwtMgr, branchID
}

func TestBillingHandler_CreateInvoice_RBAC(t *testing.T) {
	r, _, jwtMgr, branchID := setupBillingHandlerRouter()

	p1, _ := jwtMgr.GenerateTokenPair(&domain.User{ID: uuid.New(), Role: domain.RoleAccountant})
	accountantToken := p1.AccessToken
	p2, _ := jwtMgr.GenerateTokenPair(&domain.User{ID: uuid.New(), Role: domain.RoleMember})
	memberToken := p2.AccessToken

	invBody := map[string]interface{}{
		"member_id":    uuid.New(),
		"branch_id":    branchID,
		"amount_kurus": 150000,
	}
	jsonBody, _ := json.Marshal(invBody)

	// 1. Accountant -> 201 Created
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/invoices", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+accountantToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for accountant, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Member -> 403 Forbidden
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/invoices", bytes.NewBuffer(jsonBody))
	req2.Header.Set("Authorization", "Bearer "+memberToken)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for member, got %d", w2.Code)
	}
}

func TestBillingHandler_PayInvoice_Idempotent(t *testing.T) {
	r, repo, jwtMgr, branchID := setupBillingHandlerRouter()

	memberID := uuid.New()
	p, _ := jwtMgr.GenerateTokenPair(&domain.User{ID: memberID, Role: domain.RoleMember})
	memberToken := p.AccessToken

	invoice := &domain.Invoice{
		ID:            uuid.New(),
		InvoiceNumber: "FIT-2026-IST01-000001",
		MemberID:      memberID,
		BranchID:      branchID,
		AmountKurus:   200000,
		Status:        domain.InvoiceStatusPending,
		CreatedAt:     time.Now().UTC(),
	}
	repo.invoices[invoice.ID] = invoice

	payBody := map[string]interface{}{
		"card_token": "card_tok_valid_test",
	}
	jsonBody, _ := json.Marshal(payBody)

	// Payment 1 with Idempotency-Key
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/invoices/"+invoice.ID.String()+"/pay", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idemp_hnd_test_001")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp1 struct {
		Data domain.Payment `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp1)

	// Payment 2 with SAME Idempotency-Key
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/invoices/"+invoice.ID.String()+"/pay", bytes.NewBuffer(jsonBody))
	req2.Header.Set("Authorization", "Bearer "+memberToken)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "idemp_hnd_test_001")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on idempotent retry, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp2 struct {
		Data domain.Payment `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1.Data.ID != resp2.Data.ID {
		t.Errorf("expected same payment ID on idempotent request: %s vs %s", resp1.Data.ID, resp2.Data.ID)
	}
}
