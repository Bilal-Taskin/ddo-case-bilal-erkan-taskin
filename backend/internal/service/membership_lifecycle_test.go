package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/dateutil"
	"fitgiris/internal/service"
)

// mockMembershipRepo simulates PostgresMembershipRepository in memory
type mockMembershipRepo struct {
	mu             sync.Mutex
	packageTypes   map[uuid.UUID]*domain.PackageType
	memberships    map[uuid.UUID]*domain.MemberPackage
	freezes        map[uuid.UUID]*domain.PackageFreeze
	transfers      map[uuid.UUID]*domain.PackageTransfer
	packageFreezes map[uuid.UUID][]*domain.PackageFreeze
}

func newMockMembershipRepo() *mockMembershipRepo {
	return &mockMembershipRepo{
		packageTypes:   make(map[uuid.UUID]*domain.PackageType),
		memberships:    make(map[uuid.UUID]*domain.MemberPackage),
		freezes:        make(map[uuid.UUID]*domain.PackageFreeze),
		transfers:      make(map[uuid.UUID]*domain.PackageTransfer),
		packageFreezes: make(map[uuid.UUID][]*domain.PackageFreeze),
	}
}

func (m *mockMembershipRepo) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(nil)
}

func (m *mockMembershipRepo) CreatePackageType(ctx context.Context, pt *domain.PackageType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packageTypes[pt.ID] = pt
	return nil
}

func (m *mockMembershipRepo) GetPackageTypeByID(ctx context.Context, id uuid.UUID) (*domain.PackageType, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.packageTypes[id]
	if !ok {
		return nil, domain.ErrPackageTypeNotFound
	}
	copyPt := *pt
	return &copyPt, nil
}

func (m *mockMembershipRepo) ListPackageTypes(ctx context.Context, activeOnly bool) ([]domain.PackageType, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.PackageType
	for _, pt := range m.packageTypes {
		if !activeOnly || pt.IsActive {
			list = append(list, *pt)
		}
	}
	return list, nil
}

func (m *mockMembershipRepo) CreateMemberPackage(ctx context.Context, mp *domain.MemberPackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memberships[mp.ID] = mp
	return nil
}

func (m *mockMembershipRepo) CreateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	m.memberships[mp.ID] = mp
	return nil
}

func (m *mockMembershipRepo) GetMemberPackageByID(ctx context.Context, id uuid.UUID) (*domain.MemberPackage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mp, ok := m.memberships[id]
	if !ok {
		return nil, domain.ErrMembershipNotFound
	}
	copyMp := *mp
	if pt, exists := m.packageTypes[copyMp.PackageTypeID]; exists {
		copyMp.PackageName = pt.Name
		copyMp.PackageKind = pt.Type
		copyMp.MaxFreezeDays = pt.MaxFreezeDays
	}
	return &copyMp, nil
}

func (m *mockMembershipRepo) GetMemberPackageForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.MemberPackage, error) {
	mp, ok := m.memberships[id]
	if !ok {
		return nil, domain.ErrMembershipNotFound
	}
	copyMp := *mp
	if pt, exists := m.packageTypes[copyMp.PackageTypeID]; exists {
		copyMp.PackageName = pt.Name
		copyMp.PackageKind = pt.Type
		copyMp.MaxFreezeDays = pt.MaxFreezeDays
	}
	return &copyMp, nil
}

func (m *mockMembershipRepo) ListMemberPackages(ctx context.Context, memberID *uuid.UUID, status *domain.MembershipStatus, limit, offset int) ([]domain.MemberPackage, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []domain.MemberPackage
	for _, mp := range m.memberships {
		if memberID != nil && mp.MemberID != *memberID {
			continue
		}
		if status != nil && mp.Status != *status {
			continue
		}
		list = append(list, *mp)
	}
	return list, len(list), nil
}

func (m *mockMembershipRepo) UpdateMemberPackageTx(ctx context.Context, tx pgx.Tx, mp *domain.MemberPackage) error {
	m.memberships[mp.ID] = mp
	return nil
}

func (m *mockMembershipRepo) CreateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	m.freezes[freeze.ID] = freeze
	m.packageFreezes[freeze.MemberPackageID] = append(m.packageFreezes[freeze.MemberPackageID], freeze)
	return nil
}

func (m *mockMembershipRepo) GetActiveFreezeTx(ctx context.Context, tx pgx.Tx, memberPackageID uuid.UUID) (*domain.PackageFreeze, error) {
	freezes := m.packageFreezes[memberPackageID]
	for i := len(freezes) - 1; i >= 0; i-- {
		if freezes[i].ActualUnfreezeDate == nil {
			return freezes[i], nil
		}
	}
	return nil, domain.ErrNoActiveFreeze
}

func (m *mockMembershipRepo) UpdateFreezeTx(ctx context.Context, tx pgx.Tx, freeze *domain.PackageFreeze) error {
	m.freezes[freeze.ID] = freeze
	return nil
}

func (m *mockMembershipRepo) GetTotalFrozenDays(ctx context.Context, memberPackageID uuid.UUID) (int, error) {
	var total int
	for _, f := range m.packageFreezes[memberPackageID] {
		total += f.DaysFrozen
	}
	return total, nil
}

func (m *mockMembershipRepo) CreateTransferTx(ctx context.Context, tx pgx.Tx, transfer *domain.PackageTransfer) error {
	m.transfers[transfer.ID] = transfer
	return nil
}

func TestMembershipService_ClampedPurchaseDates(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	auditRepo := newMockAuditRepo()
	memRepo := newMockMembershipRepo()

	svc := service.NewMembershipService(memRepo, userRepo, auditRepo)

	member := &domain.User{
		ID:        uuid.New(),
		Email:     "clamp@fitgiris.com",
		FirstName: "Deneme",
		LastName:  "Tarih",
		Role:      domain.RoleMember,
		IsActive:  true,
	}
	_ = userRepo.Create(ctx, member)

	pkgType1M, _ := svc.CreatePackageType(ctx, service.CreatePackageTypeInput{
		Name:           "1 Aylik",
		Type:           domain.PackageKindDuration,
		DurationMonths: 1,
		PriceKurus:     100000,
	})

	ip := "127.0.0.1"

	// 1. Purchase on 31 Jan 2024 (leap year) -> MUST be 29 Feb 2024
	jan31_2024 := time.Date(2024, time.January, 31, 0, 0, 0, 0, time.UTC)
	pkg2024, err := svc.PurchasePackage(ctx, service.PurchasePackageInput{
		MemberID:      member.ID,
		PackageTypeID: pkgType1M.ID,
		StartDate:     &jan31_2024,
	}, member.ID, domain.RoleMember, &ip)
	if err != nil {
		t.Fatalf("Purchase 2024 failed: %v", err)
	}
	expected2024 := time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC)
	if !pkg2024.EndDate.Equal(expected2024) {
		t.Fatalf("Clamped Date Leap Year Error: want %v, got %v", expected2024, pkg2024.EndDate)
	}

	// 2. Purchase on 31 Jan 2025 (non-leap year) -> MUST be 28 Feb 2025
	jan31_2025 := time.Date(2025, time.January, 31, 0, 0, 0, 0, time.UTC)
	pkg2025, err := svc.PurchasePackage(ctx, service.PurchasePackageInput{
		MemberID:      member.ID,
		PackageTypeID: pkgType1M.ID,
		StartDate:     &jan31_2025,
	}, member.ID, domain.RoleMember, &ip)
	if err != nil {
		t.Fatalf("Purchase 2025 failed: %v", err)
	}
	expected2025 := time.Date(2025, time.February, 28, 0, 0, 0, 0, time.UTC)
	if !pkg2025.EndDate.Equal(expected2025) {
		t.Fatalf("Clamped Date Non-Leap Year Error: want %v, got %v", expected2025, pkg2025.EndDate)
	}
}

func TestMembershipService_CompleteLifecycle(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	auditRepo := newMockAuditRepo()
	memRepo := newMockMembershipRepo()

	svc := service.NewMembershipService(memRepo, userRepo, auditRepo)

	// Create test users
	member1 := &domain.User{
		ID:        uuid.New(),
		Email:     "ahmet@fitgiris.com",
		FirstName: "Ahmet",
		LastName:  "Yılmaz",
		Role:      domain.RoleMember,
		IsActive:  true,
	}
	_ = userRepo.Create(ctx, member1)

	member2 := &domain.User{
		ID:        uuid.New(),
		Email:     "mehmet@fitgiris.com",
		FirstName: "Mehmet",
		LastName:  "Demir",
		Role:      domain.RoleMember,
		IsActive:  true,
	}
	_ = userRepo.Create(ctx, member2)

	branchID := uuid.New()

	// 1. Create Package Type: 3-Month Standard with max 30 freeze days
	pkgType3M, err := svc.CreatePackageType(ctx, service.CreatePackageTypeInput{
		Name:           "3 Aylik Standart",
		Type:           domain.PackageKindDuration,
		DurationMonths: 3,
		PriceKurus:     300000, // 3000 TL
		AllBranches:    false,
		MaxFreezeDays:  30,
	})
	if err != nil {
		t.Fatalf("CreatePackageType basarisiz: %v", err)
	}

	// Create 12-Month VIP package for upgrade test
	pkgTypeVIP, err := svc.CreatePackageType(ctx, service.CreatePackageTypeInput{
		Name:           "12 Aylik VIP",
		Type:           domain.PackageKindDuration,
		DurationMonths: 12,
		PriceKurus:     1200000, // 12,000 TL
		AllBranches:    true,
		MaxFreezeDays:  60,
	})
	if err != nil {
		t.Fatalf("CreatePackageType VIP basarisiz: %v", err)
	}

	// 2. Purchase starting TODAY (valid and active now)
	today := dateutil.TruncateToDate(time.Now().UTC())
	ip := "127.0.0.1"
	purchasedPkg, err := svc.PurchasePackage(ctx, service.PurchasePackageInput{
		MemberID:      member1.ID,
		PackageTypeID: pkgType3M.ID,
		BranchID:      branchID,
		StartDate:     &today,
	}, member1.ID, domain.RoleMember, &ip)

	if err != nil {
		t.Fatalf("PurchasePackage basarisiz: %v", err)
	}

	if purchasedPkg.Status != domain.StatusActive {
		t.Fatalf("Yeni alinan paket ACTIVE olmali, alinan: %s", purchasedPkg.Status)
	}

	expectedEnd := dateutil.AddClampedMonths(today, 3)
	if !purchasedPkg.EndDate.Equal(expectedEnd) {
		t.Fatalf("EndDate hatasi: beklenen %v, alinan %v", expectedEnd, purchasedPkg.EndDate)
	}

	// 3. Freeze Package: Freeze for 10 days
	frozenPkg, err := svc.FreezeMembership(ctx, service.FreezeMembershipInput{
		MemberPackageID: purchasedPkg.ID,
		Days:            10,
		Reason:          "Kaza",
	}, member1.ID, domain.RoleMember, &ip)

	if err != nil {
		t.Fatalf("FreezeMembership basarisiz: %v", err)
	}
	if frozenPkg.Status != domain.StatusFrozen {
		t.Fatalf("Dondurulan paket FROZEN olmali, alinan: %s", frozenPkg.Status)
	}

	// EndDate should be extended by 10 days
	expectedExtendedEnd := dateutil.AddDays(expectedEnd, 10)
	if !frozenPkg.EndDate.Equal(expectedExtendedEnd) {
		t.Fatalf("Bitis tarihi 10 gun uzamaliydi, beklenen: %v, alinan: %v", expectedExtendedEnd, frozenPkg.EndDate)
	}

	// 4. Disallowed FSM Transition: Cannot freeze an already FROZEN package!
	_, err = svc.FreezeMembership(ctx, service.FreezeMembershipInput{
		MemberPackageID: purchasedPkg.ID,
		Days:            5,
		Reason:          "Tekrar dondurma",
	}, member1.ID, domain.RoleMember, &ip)

	if err == nil || !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("FROZEN paketin tekrar dondurulmasi ErrInvalidStateTransition ile reddedilmeliydi, alinan: %v", err)
	}

	// 5. Unfreeze early: Unfreeze package
	unfrozenPkg, err := svc.UnfreezeMembership(ctx, purchasedPkg.ID, member1.ID, domain.RoleMember, &ip)
	if err != nil {
		t.Fatalf("UnfreezeMembership basarisiz: %v", err)
	}
	if unfrozenPkg.Status != domain.StatusActive {
		t.Fatalf("Cozulen paket ACTIVE olmali, alinan: %s", unfrozenPkg.Status)
	}

	// 6. Test Max Freeze Days limit enforcement
	// Attempt to freeze for 35 days (max is 30)
	_, err = svc.FreezeMembership(ctx, service.FreezeMembershipInput{
		MemberPackageID: purchasedPkg.ID,
		Days:            35,
		Reason:          "Uzun tatil",
	}, member1.ID, domain.RoleMember, &ip)

	if err == nil || !errors.Is(err, domain.ErrFreezeLimitExceeded) {
		t.Fatalf("Maksimum dondurma limiti asildiginda ErrFreezeLimitExceeded bekleniyordu, alinan: %v", err)
	}

	// 7. Prorated Upgrade Quote
	quote, err := svc.GetUpgradeQuote(ctx, purchasedPkg.ID, pkgTypeVIP.ID)
	if err != nil {
		t.Fatalf("GetUpgradeQuote basarisiz: %v", err)
	}
	if quote.NewPackagePriceKurus != 1200000 {
		t.Errorf("NewPackagePriceKurus = %d; want 1200000", quote.NewPackagePriceKurus)
	}

	// 8. Transfer Membership to Member2
	transferredPkg, err := svc.TransferMembership(ctx, service.TransferMembershipInput{
		MemberPackageID: purchasedPkg.ID,
		ToMemberID:      member2.ID,
		Reason:          "Sehir degisikligi",
	}, member1.ID, domain.RoleMember, &ip)

	if err != nil {
		t.Fatalf("TransferMembership basarisiz: %v", err)
	}

	if transferredPkg.MemberID != member2.ID {
		t.Fatalf("Yeni paket alici uye ID'sine (%s) ait olmali, alinan: %s", member2.ID, transferredPkg.MemberID)
	}
	if transferredPkg.Status != domain.StatusActive {
		t.Fatalf("Yeni transfer paketi ACTIVE olmali, alinan: %s", transferredPkg.Status)
	}

	// Check original package status in repo: MUST be TRANSFERRED
	origPkg, err := memRepo.GetMemberPackageByID(ctx, purchasedPkg.ID)
	if err != nil {
		t.Fatalf("Orijinal paket bulunamadi: %v", err)
	}
	if origPkg.Status != domain.StatusTransferred {
		t.Fatalf("Orijinal paket TRANSFERRED olmali, alinan: %s", origPkg.Status)
	}

	// 9. Disallowed FSM Transition from terminal state TRANSFERRED
	// Cannot cancel or freeze a TRANSFERRED package!
	err = svc.CancelMembership(ctx, origPkg.ID, "Iptal denemesi", member1.ID, domain.RoleMember, &ip)
	if err == nil || !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("TRANSFERRED paket iptal edilememeli, ErrInvalidStateTransition bekleniyordu, alinan: %v", err)
	}

	// 10. Cancel active transferredPkg
	err = svc.CancelMembership(ctx, transferredPkg.ID, "Vazgecti", member2.ID, domain.RoleMember, &ip)
	if err != nil {
		t.Fatalf("CancelMembership basarisiz: %v", err)
	}

	cancelledPkg, _ := memRepo.GetMemberPackageByID(ctx, transferredPkg.ID)
	if cancelledPkg.Status != domain.StatusCancelled {
		t.Fatalf("Paket CANCELLED olmali, alinan: %s", cancelledPkg.Status)
	}
}
