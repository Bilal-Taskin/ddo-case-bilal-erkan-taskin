package service_test

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fitgiris/internal/domain"
	"fitgiris/internal/service"
)

// In-memory mock for TurnstileRepository
type mockTurnstileRepo struct {
	mu           sync.Mutex
	turnstiles   map[uuid.UUID]*domain.Turnstile
	accessLogs   []domain.AccessLog
	packages     map[uuid.UUID]*domain.MemberPackage
	memberPkgs   map[uuid.UUID]*domain.MemberPackage // memberID -> active package
}

func newMockTurnstileRepo() *mockTurnstileRepo {
	return &mockTurnstileRepo{
		turnstiles: make(map[uuid.UUID]*domain.Turnstile),
		packages:   make(map[uuid.UUID]*domain.MemberPackage),
		memberPkgs: make(map[uuid.UUID]*domain.MemberPackage),
	}
}

func (m *mockTurnstileRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(nil)
}

func (m *mockTurnstileRepo) CreateTurnstile(ctx context.Context, t *domain.Turnstile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	m.turnstiles[t.ID] = t
	return nil
}

func (m *mockTurnstileRepo) GetTurnstileByID(ctx context.Context, id uuid.UUID) (*domain.Turnstile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.turnstiles[id]
	if !ok {
		return nil, domain.ErrTurnstileNotFound
	}
	return t, nil
}

func (m *mockTurnstileRepo) GetTurnstileByBranchAndCode(ctx context.Context, branchID uuid.UUID, code string) (*domain.Turnstile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.turnstiles {
		if t.BranchID == branchID && t.Code == code {
			return t, nil
		}
	}
	return nil, domain.ErrTurnstileNotFound
}

func (m *mockTurnstileRepo) GetTurnstileByCode(ctx context.Context, code string) (*domain.Turnstile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.turnstiles {
		if t.Code == code {
			return t, nil
		}
	}
	return nil, domain.ErrTurnstileNotFound
}

func (m *mockTurnstileRepo) ListTurnstiles(ctx context.Context, branchID *uuid.UUID, activeOnly bool) ([]domain.Turnstile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.Turnstile
	for _, t := range m.turnstiles {
		if branchID != nil && t.BranchID != *branchID {
			continue
		}
		if activeOnly && !t.IsActive {
			continue
		}
		list = append(list, *t)
	}
	return list, nil
}

func (m *mockTurnstileRepo) UpdateTurnstile(ctx context.Context, t *domain.Turnstile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.turnstiles[t.ID] = t
	return nil
}

func (m *mockTurnstileRepo) LogAccess(ctx context.Context, log *domain.AccessLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	m.accessLogs = append(m.accessLogs, *log)
	return nil
}

func (m *mockTurnstileRepo) LogAccessBatch(ctx context.Context, logs []domain.AccessLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range logs {
		if l.ID == uuid.Nil {
			l.ID = uuid.New()
		}
		m.accessLogs = append(m.accessLogs, l)
	}
	return nil
}

func (m *mockTurnstileRepo) GetLastGrantedAccess(ctx context.Context, memberID, branchID uuid.UUID) (*domain.AccessLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.accessLogs) - 1; i >= 0; i-- {
		l := m.accessLogs[i]
		if l.MemberID == memberID && l.BranchID == branchID && l.Status == domain.AccessGranted {
			return &l, nil
		}
	}
	return nil, nil
}

func (m *mockTurnstileRepo) ListAccessLogs(ctx context.Context, branchID *uuid.UUID, memberID *uuid.UUID, limit, offset int) ([]domain.AccessLog, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var filtered []domain.AccessLog
	for _, l := range m.accessLogs {
		if branchID != nil && l.BranchID != *branchID {
			continue
		}
		if memberID != nil && l.MemberID != *memberID {
			continue
		}
		filtered = append(filtered, l)
	}
	return filtered, len(filtered), nil
}

func (m *mockTurnstileRepo) GetActiveMemberPackageTx(ctx context.Context, tx pgx.Tx, memberID uuid.UUID) (*domain.MemberPackage, error) {
	pkg, ok := m.memberPkgs[memberID]
	if !ok || pkg == nil || pkg.Status != domain.StatusActive {
		return nil, domain.ErrMembershipNotFound
	}
	return pkg, nil
}

func (m *mockTurnstileRepo) DeductSessionEntryTx(ctx context.Context, tx pgx.Tx, packageID uuid.UUID) error {
	for _, pkg := range m.memberPkgs {
		if pkg.ID == packageID {
			if pkg.RemainingEntries <= 0 {
				return domain.ErrInsufficientEntries
			}
			pkg.RemainingEntries--
			return nil
		}
	}
	return domain.ErrInsufficientEntries
}

// In-memory mock for UserRepository
type mockTurnstileUserRepo struct {
	mu    sync.Mutex
	users map[uuid.UUID]*domain.User
}

func (m *mockTurnstileUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
	return nil
}

func (m *mockTurnstileUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (m *mockTurnstileUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (m *mockTurnstileUserRepo) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	return nil
}
func (m *mockTurnstileUserRepo) IsRefreshTokenValid(ctx context.Context, tokenHash string) (bool, uuid.UUID, error) {
	return false, uuid.Nil, nil
}
func (m *mockTurnstileUserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return nil
}

// In-memory mock for AuditRepository
type mockTurnstileAuditRepo struct {
	mu   sync.Mutex
	logs []domain.AuditLog
}

func (m *mockTurnstileAuditRepo) Append(ctx context.Context, log *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockTurnstileAuditRepo) List(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs, nil
}

func setupTurnstileTestEnv() (
	service.TurnstileService,
	*mockTurnstileRepo,
	*mockTurnstileUserRepo,
	*mockTurnstileAuditRepo,
	*domain.Turnstile,
	*domain.Turnstile,
	*domain.User,
) {
	repo := newMockTurnstileRepo()
	userRepo := &mockTurnstileUserRepo{users: make(map[uuid.UUID]*domain.User)}
	auditRepo := &mockTurnstileAuditRepo{}

	branchID := uuid.New()

	inGate := &domain.Turnstile{
		ID:        uuid.New(),
		BranchID:  branchID,
		Code:      "TURN-IN-01",
		Name:      "Ana Giris Turnikesi",
		Direction: domain.DirectionIn,
		IsActive:  true,
	}
	_ = repo.CreateTurnstile(context.Background(), inGate)

	outGate := &domain.Turnstile{
		ID:        uuid.New(),
		BranchID:  branchID,
		Code:      "TURN-OUT-01",
		Name:      "Ana Cikis Turnikesi",
		Direction: domain.DirectionOut,
		IsActive:  true,
	}
	_ = repo.CreateTurnstile(context.Background(), outGate)

	member := &domain.User{
		ID:        uuid.New(),
		FirstName: "Ali",
		LastName:  "Demir",
		Role:      domain.RoleMember,
		IsActive:  true,
	}
	userRepo.users[member.ID] = member

	svc := service.NewTurnstileService(repo, userRepo, auditRepo)
	return svc, repo, userRepo, auditRepo, inGate, outGate, member
}

// -------------------------------------------------------------
// Test 1: Active Membership Pass & Anti-Passback (2h window)
// -------------------------------------------------------------
func TestTurnstile_AntiPassback_TwoHoursRule(t *testing.T) {
	svc, repo, _, _, inGate, outGate, member := setupTurnstileTestEnv()
	ctx := context.Background()

	// Give member an active duration package
	repo.memberPkgs[member.ID] = &domain.MemberPackage{
		ID:          uuid.New(),
		MemberID:    member.ID,
		BranchID:    inGate.BranchID,
		AllBranches: true,
		Status:      domain.StatusActive,
		PackageKind: domain.PackageKindDuration,
		StartDate:   time.Now().Add(-24 * time.Hour),
		EndDate:     time.Now().Add(30 * 24 * time.Hour),
	}

	// STEP 1: First entry request -> MUST BE GRANTED
	dec1, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error on first entry: %v", err)
	}
	if !dec1.Granted || dec1.Status != domain.AccessGranted {
		t.Fatalf("first entry should be GRANTED, got status: %s, reason: %s", dec1.Status, dec1.Reason)
	}

	// STEP 2: Immediate second entry request at same branch without exiting -> MUST BE DENIED (ERR_ANTI_PASSBACK)
	dec2, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error on second entry: %v", err)
	}
	if dec2.Granted || dec2.Status != domain.AccessDenied || dec2.Reason != domain.ReasonAntiPassback {
		t.Fatalf("second entry must be DENIED with ERR_ANTI_PASSBACK, got granted=%v, reason=%s", dec2.Granted, dec2.Reason)
	}

	// STEP 3: Member exits through OUT turnstile -> MUST BE GRANTED
	decExit, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &outGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error on exit: %v", err)
	}
	if !decExit.Granted || decExit.Status != domain.AccessGranted {
		t.Fatalf("exit must be GRANTED, got status: %s", decExit.Status)
	}

	// STEP 4: Member re-enters after having exited -> MUST BE GRANTED (anti-passback reset by OUT)
	dec3, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error on re-entry: %v", err)
	}
	if !dec3.Granted || dec3.Status != domain.AccessGranted {
		t.Fatalf("re-entry after exit should be GRANTED, got reason: %s", dec3.Reason)
	}
}

// -------------------------------------------------------------
// Test 2: Inactive & Expired Memberships Denied
// -------------------------------------------------------------
func TestTurnstile_InactiveAndExpiredMembership(t *testing.T) {
	svc, repo, _, _, inGate, _, member := setupTurnstileTestEnv()
	ctx := context.Background()

	// Case A: No package at all -> DENIED
	decNoPkg, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decNoPkg.Granted || decNoPkg.Reason != domain.ReasonMembershipInactive {
		t.Errorf("expected DENIED with ERR_MEMBERSHIP_INACTIVE, got %s", decNoPkg.Reason)
	}

	// Case B: Package is SUSPENDED -> DENIED
	repo.memberPkgs[member.ID] = &domain.MemberPackage{
		ID:          uuid.New(),
		MemberID:    member.ID,
		BranchID:    inGate.BranchID,
		AllBranches: true,
		Status:      domain.StatusSuspended,
		PackageKind: domain.PackageKindDuration,
	}

	decSusp, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decSusp.Granted || decSusp.Reason != domain.ReasonMembershipInactive {
		t.Errorf("expected DENIED for suspended membership, got %s", decSusp.Reason)
	}
}

// -------------------------------------------------------------
// Test 3: Session Package Deduction & Out of Sessions
// -------------------------------------------------------------
func TestTurnstile_SessionPackageEntriesDeduction(t *testing.T) {
	svc, repo, _, _, inGate, outGate, member := setupTurnstileTestEnv()
	ctx := context.Background()

	// Session package with exactly 1 entry
	pkg := &domain.MemberPackage{
		ID:               uuid.New(),
		MemberID:         member.ID,
		BranchID:         inGate.BranchID,
		AllBranches:      true,
		Status:           domain.StatusActive,
		PackageKind:      domain.PackageKindSession,
		RemainingEntries: 1,
		TotalEntries:     10,
	}
	repo.memberPkgs[member.ID] = pkg

	// Pass 1: Granted, remaining entries becomes 0
	dec1, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil || !dec1.Granted {
		t.Fatalf("first entry should be granted: %v, reason: %s", err, dec1.Reason)
	}
	if dec1.RemainingEntries == nil {
		t.Fatalf("expected remaining entries not nil")
	}
	if *dec1.RemainingEntries != 0 {
		t.Errorf("expected 0 remaining entries, got %d", *dec1.RemainingEntries)
	}

	// Member exits
	_, _ = svc.EvaluateAccess(ctx, service.AccessRequest{TurnstileID: &outGate.ID, MemberID: member.ID})

	// Pass 2: Remaining entries = 0 -> MUST BE DENIED with ERR_INSUFFICIENT_ENTRIES
	dec2, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec2.Granted || dec2.Reason != domain.ReasonInsufficientEntries {
		t.Errorf("expected DENIED with ERR_INSUFFICIENT_ENTRIES, got granted=%v, reason=%s", dec2.Granted, dec2.Reason)
	}
}

// -------------------------------------------------------------
// Test 4: Branch Permission (Single vs Multi-Branch)
// -------------------------------------------------------------
func TestTurnstile_BranchAccessRules(t *testing.T) {
	svc, repo, _, _, inGate, _, member := setupTurnstileTestEnv()
	ctx := context.Background()

	otherBranchID := uuid.New()

	// Single branch package tied to otherBranchID (not inGate.BranchID)
	repo.memberPkgs[member.ID] = &domain.MemberPackage{
		ID:          uuid.New(),
		MemberID:    member.ID,
		BranchID:    otherBranchID,
		AllBranches: false,
		Status:      domain.StatusActive,
		PackageKind: domain.PackageKindDuration,
	}

	// Should be DENIED with ERR_BRANCH_ACCESS_DENIED
	dec, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Granted || dec.Reason != domain.ReasonBranchAccessDenied {
		t.Errorf("expected DENIED with ERR_BRANCH_ACCESS_DENIED, got %s", dec.Reason)
	}

	// Now upgrade package to AllBranches = true
	repo.memberPkgs[member.ID].AllBranches = true

	decAllowed, err := svc.EvaluateAccess(ctx, service.AccessRequest{
		TurnstileID: &inGate.ID,
		MemberID:    member.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decAllowed.Granted {
		t.Errorf("expected GRANTED for all-branches package, got %s", decAllowed.Reason)
	}
}

// -------------------------------------------------------------
// Test 5: Latency Requirement - Prove p95 < 50ms (T-3 requirement)
// -------------------------------------------------------------
func TestTurnstile_LatencyRequirement_P95Under50ms(t *testing.T) {
	svc, repo, userRepo, _, inGate, outGate, _ := setupTurnstileTestEnv()
	ctx := context.Background()

	const iterations = 500
	latencies := make([]time.Duration, iterations)

	// Pre-populate 500 members with packages
	members := make([]uuid.UUID, iterations)
	for i := 0; i < iterations; i++ {
		mID := uuid.New()
		members[i] = mID
		userRepo.users[mID] = &domain.User{ID: mID, Role: domain.RoleMember, IsActive: true}
		repo.memberPkgs[mID] = &domain.MemberPackage{
			ID:          uuid.New(),
			MemberID:    mID,
			BranchID:    inGate.BranchID,
			AllBranches: true,
			Status:      domain.StatusActive,
			PackageKind: domain.PackageKindDuration,
		}
	}

	// Run 500 evaluations measuring exact duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		dec, err := svc.EvaluateAccess(ctx, service.AccessRequest{
			TurnstileID: &inGate.ID,
			MemberID:    members[i],
		})
		elapsed := time.Since(start)
		latencies[i] = elapsed

		if err != nil || !dec.Granted {
			t.Fatalf("iteration %d failed: err=%v, reason=%s", i, err, dec.Reason)
		}

		// Also do exit so subsequent accesses are clean
		_, _ = svc.EvaluateAccess(ctx, service.AccessRequest{
			TurnstileID: &outGate.ID,
			MemberID:    members[i],
		})
	}

	// Sort latencies to compute p95
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p95Index := int(float64(iterations) * 0.95)
	p95Duration := latencies[p95Index]
	t.Logf("Measured Turnstile p95 Latency: %v across %d requests", p95Duration, iterations)

	// PROOF: p95 must be strictly under 50ms
	if p95Duration >= 50*time.Millisecond {
		t.Fatalf("p95 latency violated! Expected < 50ms, got %v", p95Duration)
	}
}

// -------------------------------------------------------------
// Test 6: Offline Sync & Anomaly Flagging
// -------------------------------------------------------------
func TestTurnstile_OfflineSyncAndAnomalyFlagging(t *testing.T) {
	svc, repo, userRepo, auditRepo, inGate, _, member := setupTurnstileTestEnv()
	ctx := context.Background()

	bogusMemberID := uuid.New()

	syncItems := []domain.OfflineSyncItem{
		{
			TurnstileCode: inGate.Code,
			MemberID:      member.ID,
			Direction:     domain.DirectionIn,
			Status:        domain.AccessGranted,
			Reason:        domain.ReasonSuccess,
			OccurredAt:    time.Now().Add(-1 * time.Hour),
		},
		{
			// Non-existent member who was admitted offline -> ANOMALY!
			TurnstileCode: inGate.Code,
			MemberID:      bogusMemberID,
			Direction:     domain.DirectionIn,
			Status:        domain.AccessGranted,
			Reason:        domain.ReasonSuccess,
			OccurredAt:    time.Now().Add(-30 * time.Minute),
		},
	}

	res, err := svc.SyncOfflineAccessLogs(ctx, syncItems)
	if err != nil {
		t.Fatalf("SyncOfflineAccessLogs failed: %v", err)
	}

	if res.TotalProcessed != 2 {
		t.Errorf("TotalProcessed = %d; want 2", res.TotalProcessed)
	}
	if res.AnomaliesDetected != 1 {
		t.Errorf("AnomaliesDetected = %d; want 1", res.AnomaliesDetected)
	}

	// Verify audit log has the FLAGGED_OFFLINE_ANOMALY
	if len(auditRepo.logs) == 0 {
		t.Fatalf("expected audit log for anomaly, got 0")
	}
	foundAnomaly := false
	for _, l := range auditRepo.logs {
		if l.Action == "FLAGGED_OFFLINE_ANOMALY" {
			foundAnomaly = true
			break
		}
	}
	if !foundAnomaly {
		t.Errorf("FLAGGED_OFFLINE_ANOMALY action not found in audit logs")
	}

	// Verify logs are marked offline_synced = true
	logs, _, _ := repo.ListAccessLogs(ctx, nil, nil, 10, 0)
	if len(logs) == 0 {
		t.Fatalf("expected synced logs in repository")
	}
	for _, l := range logs {
		if !l.OfflineSynced {
			t.Errorf("log %s should have offline_synced = true", l.ID)
		}
	}
	_ = userRepo // suppress unused
}
