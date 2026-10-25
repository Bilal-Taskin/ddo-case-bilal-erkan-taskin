package service_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/service"
)

func TestCalculateUpgradeQuote_DurationPackage(t *testing.T) {
	pkgID := uuid.New()
	newPkgID := uuid.New()

	// 30 days total package: Jan 1 to Jan 31
	// Price paid: 300,000 kurus (3000 TL)
	startDate := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, time.January, 31, 0, 0, 0, 0, time.UTC)

	currentPkg := &domain.MemberPackage{
		ID:             pkgID,
		Status:         domain.StatusActive,
		PackageKind:    domain.PackageKindDuration,
		StartDate:      startDate,
		EndDate:        endDate,
		PricePaidKurus: 300000,
	}

	newPkgType := &domain.PackageType{
		ID:             newPkgID,
		Name:           "12 Aylik VIP",
		Type:           domain.PackageKindDuration,
		DurationMonths: 12,
		PriceKurus:     1200000, // 12,000 TL
		IsActive:       true,
	}

	// Case 1: Exactly halfway (15 days elapsed, 15 days remaining on Jan 16)
	asOfDate := time.Date(2025, time.January, 16, 0, 0, 0, 0, time.UTC)
	quote, err := service.CalculateUpgradeQuote(currentPkg, newPkgType, asOfDate)
	if err != nil {
		t.Fatalf("CalculateUpgradeQuote basarisiz: %v", err)
	}

	if quote.TotalDays != 30 {
		t.Errorf("TotalDays = %d; want 30", quote.TotalDays)
	}
	if quote.RemainingDays != 15 {
		t.Errorf("RemainingDays = %d; want 15", quote.RemainingDays)
	}
	// Remaining value: (15 * 300,000) / 30 = 150,000 kurus (1500 TL)
	if quote.RemainingValueKurus != 150000 {
		t.Errorf("RemainingValueKurus = %d; want 150000", quote.RemainingValueKurus)
	}
	// Amount to pay: 1,200,000 - 150,000 = 1,050,000 kurus (10500 TL)
	if quote.AmountToPayKurus != 1050000 {
		t.Errorf("AmountToPayKurus = %d; want 1050000", quote.AmountToPayKurus)
	}

	// Case 2: On last day (Jan 31, 0 days remaining)
	asOfDateEnd := time.Date(2025, time.January, 31, 0, 0, 0, 0, time.UTC)
	quoteEnd, err := service.CalculateUpgradeQuote(currentPkg, newPkgType, asOfDateEnd)
	if err != nil {
		t.Fatalf("CalculateUpgradeQuote on end date basarisiz: %v", err)
	}
	if quoteEnd.RemainingDays != 0 {
		t.Errorf("RemainingDays on end date = %d; want 0", quoteEnd.RemainingDays)
	}
	if quoteEnd.RemainingValueKurus != 0 {
		t.Errorf("RemainingValueKurus = %d; want 0", quoteEnd.RemainingValueKurus)
	}
	if quoteEnd.AmountToPayKurus != 1200000 {
		t.Errorf("AmountToPayKurus = %d; want 1200000", quoteEnd.AmountToPayKurus)
	}

	// Case 3: When remaining credit exceeds new package price (e.g. downgrading or cheap new promo)
	cheapPackage := &domain.PackageType{
		ID:             uuid.New(),
		Name:           "Giris Paketi",
		Type:           domain.PackageKindDuration,
		DurationMonths: 1,
		PriceKurus:     100000, // 100,000 kurus, but remaining value is 150,000 kurus
		IsActive:       true,
	}
	quoteCheap, err := service.CalculateUpgradeQuote(currentPkg, cheapPackage, asOfDate)
	if err != nil {
		t.Fatalf("CalculateUpgradeQuote cheap package basarisiz: %v", err)
	}
	// Difference cannot be negative; should floor at 0
	if quoteCheap.AmountToPayKurus != 0 {
		t.Errorf("AmountToPayKurus = %d; want 0", quoteCheap.AmountToPayKurus)
	}
}

func TestCalculateUpgradeQuote_SessionPackage(t *testing.T) {
	pkgID := uuid.New()
	newPkgID := uuid.New()

	// 10 session package for 100,000 kurus (1000 TL), 4 sessions remaining
	currentPkg := &domain.MemberPackage{
		ID:               pkgID,
		Status:           domain.StatusActive,
		PackageKind:      domain.PackageKindSession,
		StartDate:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:          time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
		TotalEntries:     10,
		RemainingEntries: 4,
		PricePaidKurus:   100000,
	}

	newPkgType := &domain.PackageType{
		ID:             newPkgID,
		Name:           "20 Girislik Paket",
		Type:           domain.PackageKindSession,
		DurationMonths: 6,
		TotalEntries:   20,
		PriceKurus:     180000,
		IsActive:       true,
	}

	asOfDate := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	quote, err := service.CalculateUpgradeQuote(currentPkg, newPkgType, asOfDate)
	if err != nil {
		t.Fatalf("CalculateUpgradeQuote session basarisiz: %v", err)
	}

	// Remaining value = (4 * 100,000) / 10 = 40,000 kurus
	if quote.RemainingValueKurus != 40000 {
		t.Errorf("RemainingValueKurus = %d; want 40000", quote.RemainingValueKurus)
	}
	// Difference to pay = 180,000 - 40,000 = 140,000 kurus
	if quote.AmountToPayKurus != 140000 {
		t.Errorf("AmountToPayKurus = %d; want 140000", quote.AmountToPayKurus)
	}
}
