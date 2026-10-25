package domain_test

import (
	"errors"
	"testing"

	"fitgiris/internal/domain"
)

func TestFSM_AllowedTransitions(t *testing.T) {
	validTransitions := []struct {
		from domain.MembershipStatus
		to   domain.MembershipStatus
	}{
		// From PENDING_PAYMENT
		{domain.StatusPendingPayment, domain.StatusActive},
		{domain.StatusPendingPayment, domain.StatusCancelled},

		// From ACTIVE
		{domain.StatusActive, domain.StatusFrozen},
		{domain.StatusActive, domain.StatusTransferred},
		{domain.StatusActive, domain.StatusSuspended},
		{domain.StatusActive, domain.StatusExpired},
		{domain.StatusActive, domain.StatusCancelled},

		// From FROZEN
		{domain.StatusFrozen, domain.StatusActive},
		{domain.StatusFrozen, domain.StatusCancelled},
		{domain.StatusFrozen, domain.StatusExpired},

		// From SUSPENDED
		{domain.StatusSuspended, domain.StatusActive},
		{domain.StatusSuspended, domain.StatusCancelled},
		{domain.StatusSuspended, domain.StatusExpired},
	}

	for _, tt := range validTransitions {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if !tt.from.CanTransitionTo(tt.to) {
				t.Errorf("Beklenen gecis basarisiz oldu: %s -> %s", tt.from, tt.to)
			}
			if err := domain.ValidateTransition(tt.from, tt.to); err != nil {
				t.Errorf("ValidateTransition hata donmemeliydi: %v", err)
			}
		})
	}
}

func TestFSM_DisallowedTransitions(t *testing.T) {
	invalidTransitions := []struct {
		from domain.MembershipStatus
		to   domain.MembershipStatus
	}{
		// Cannot transition from terminal states
		{domain.StatusCancelled, domain.StatusActive},
		{domain.StatusCancelled, domain.StatusPendingPayment},
		{domain.StatusCancelled, domain.StatusFrozen},
		{domain.StatusTransferred, domain.StatusActive},
		{domain.StatusTransferred, domain.StatusFrozen},
		{domain.StatusTransferred, domain.StatusCancelled},
		{domain.StatusExpired, domain.StatusActive},
		{domain.StatusExpired, domain.StatusFrozen},

		// Disallowed direct jumps
		{domain.StatusPendingPayment, domain.StatusFrozen},
		{domain.StatusPendingPayment, domain.StatusTransferred},
		{domain.StatusPendingPayment, domain.StatusSuspended},
		{domain.StatusFrozen, domain.StatusTransferred},
		{domain.StatusFrozen, domain.StatusSuspended},
		{domain.StatusSuspended, domain.StatusFrozen},
		{domain.StatusSuspended, domain.StatusTransferred},

		// Self transitions
		{domain.StatusActive, domain.StatusActive},
		{domain.StatusFrozen, domain.StatusFrozen},
		{domain.StatusCancelled, domain.StatusCancelled},
	}

	for _, tt := range invalidTransitions {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if tt.from.CanTransitionTo(tt.to) {
				t.Errorf("Gecersiz gecise izin verildi: %s -> %s", tt.from, tt.to)
			}
			err := domain.ValidateTransition(tt.from, tt.to)
			if err == nil {
				t.Fatalf("Gecersiz geciste hata donmeliydi: %s -> %s", tt.from, tt.to)
			}
			if !errors.Is(err, domain.ErrInvalidStateTransition) {
				t.Errorf("ErrInvalidStateTransition bekleniyordu, alinan: %v", err)
			}
		})
	}
}
