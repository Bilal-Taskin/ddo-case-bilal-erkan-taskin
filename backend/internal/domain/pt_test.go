package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPTPackage_CanBookAndDeduct(t *testing.T) {
	pkg := &PTPackage{
		ID:                uuid.New(),
		MemberID:          uuid.New(),
		TrainerID:         uuid.New(),
		TotalSessions:     10,
		RemainingSessions: 1,
		PriceKurus:        1000000,
		IsActive:          true,
		CreatedAt:         time.Now(),
	}

	if err := pkg.CanBook(); err != nil {
		t.Fatalf("expected package to be bookable: %v", err)
	}

	if err := pkg.DeductSession(); err != nil {
		t.Fatalf("expected session deduction to succeed: %v", err)
	}
	if pkg.RemainingSessions != 0 {
		t.Errorf("expected 0 remaining sessions, got %d", pkg.RemainingSessions)
	}

	// Try deducting again
	if err := pkg.DeductSession(); err != ErrPTNoSessionsRemaining {
		t.Errorf("expected ErrPTNoSessionsRemaining, got %v", err)
	}

	// Refund session
	pkg.RefundSession()
	if pkg.RemainingSessions != 1 {
		t.Errorf("expected 1 remaining session after refund, got %d", pkg.RemainingSessions)
	}
}
