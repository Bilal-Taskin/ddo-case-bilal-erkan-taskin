package domain_test

import (
	"testing"
	"time"

	"fitgiris/internal/domain"
)

func TestFormatInvoiceNumber(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		branch   string
		seq      int64
		expected string
	}{
		{"First invoice", 2026, "IST01", 1, "FIT-2026-IST01-000001"},
		{"Ninetieth invoice", 2026, "ANK02", 90, "FIT-2026-ANK02-000090"},
		{"High sequence", 2027, "IZM01", 123456, "FIT-2027-IZM01-123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.FormatInvoiceNumber(tt.year, tt.branch, tt.seq)
			if got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestCalculateNextRetry(t *testing.T) {
	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	// Attempt 1 fails -> Day 3
	next1, canRetry1 := domain.CalculateNextRetry(1, base)
	if !canRetry1 {
		t.Errorf("attempt 1 should allow retry")
	}
	if next1 != time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC) {
		t.Errorf("expected 2026-05-03, got %v", next1)
	}

	// Attempt 2 fails -> Day 5
	next2, canRetry2 := domain.CalculateNextRetry(2, next1)
	if !canRetry2 {
		t.Errorf("attempt 2 should allow retry")
	}
	if next2 != time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC) {
		t.Errorf("expected 2026-05-05, got %v", next2)
	}

	// Attempt 3 fails -> Day 7: exhausted
	next3, canRetry3 := domain.CalculateNextRetry(3, next2)
	if canRetry3 {
		t.Errorf("attempt 3 should NOT allow further retry (suspension due)")
	}
	if next3 != time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC) {
		t.Errorf("expected 2026-05-07, got %v", next3)
	}
}
