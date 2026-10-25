package domain_test

import (
	"testing"
	"time"

	"fitgiris/internal/domain"
)

func TestCheckAntiPassback(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		lastDirection  domain.TurnstileDirection
		lastAccessTime time.Time
		requestTime    time.Time
		expectViolation bool
	}{
		{
			name:           "Recent IN within 10 minutes violates anti-passback",
			lastDirection:  domain.DirectionIn,
			lastAccessTime: now.Add(-10 * time.Minute),
			requestTime:    now,
			expectViolation: true,
		},
		{
			name:           "Recent IN within 119 minutes violates anti-passback",
			lastDirection:  domain.DirectionIn,
			lastAccessTime: now.Add(-119 * time.Minute),
			requestTime:    now,
			expectViolation: true,
		},
		{
			name:           "IN exactly 2 hours ago does not violate anti-passback",
			lastDirection:  domain.DirectionIn,
			lastAccessTime: now.Add(-2 * time.Hour),
			requestTime:    now,
			expectViolation: false,
		},
		{
			name:           "IN 3 hours ago does not violate anti-passback",
			lastDirection:  domain.DirectionIn,
			lastAccessTime: now.Add(-3 * time.Hour),
			requestTime:    now,
			expectViolation: false,
		},
		{
			name:           "Recent OUT within 10 minutes does not violate anti-passback",
			lastDirection:  domain.DirectionOut,
			lastAccessTime: now.Add(-10 * time.Minute),
			requestTime:    now,
			expectViolation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.CheckAntiPassback(tt.lastDirection, tt.lastAccessTime, tt.requestTime)
			if got != tt.expectViolation {
				t.Fatalf("expected violation %v, got %v", tt.expectViolation, got)
			}
		})
	}
}

func TestTurnstileDirection_IsValid(t *testing.T) {
	if !domain.DirectionIn.IsValid() {
		t.Errorf("expected IN to be valid")
	}
	if !domain.DirectionOut.IsValid() {
		t.Errorf("expected OUT to be valid")
	}
	if domain.TurnstileDirection("SIDEWAYS").IsValid() {
		t.Errorf("expected SIDEWAYS to be invalid")
	}
}
