package domain_test

import (
	"testing"
	"time"

	"fitgiris/internal/domain"
)

func TestIsLateCancellation(t *testing.T) {
	startTime := time.Date(2026, 10, 26, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		cancelTime time.Time
		wantLate   bool
	}{
		{
			name:       "3 hours before start time - Free Cancel",
			cancelTime: startTime.Add(-3 * time.Hour),
			wantLate:   false,
		},
		{
			name:       "2 hours 30 mins before start time - Free Cancel",
			cancelTime: startTime.Add(-2*time.Hour - 30*time.Minute),
			wantLate:   false,
		},
		{
			name:       "Exactly 2 hours before start time - Free Cancel boundary",
			cancelTime: startTime.Add(-2 * time.Hour),
			wantLate:   false,
		},
		{
			name:       "1 hour 59 mins 59 secs before start time - Late Cancel",
			cancelTime: startTime.Add(-2*time.Hour + 1*time.Second),
			wantLate:   true,
		},
		{
			name:       "1 hour before start time - Late Cancel",
			cancelTime: startTime.Add(-1 * time.Hour),
			wantLate:   true,
		},
		{
			name:       "5 minutes before start time - Late Cancel",
			cancelTime: startTime.Add(-5 * time.Minute),
			wantLate:   true,
		},
		{
			name:       "At start time - Late Cancel",
			cancelTime: startTime,
			wantLate:   true,
		},
		{
			name:       "After start time - Late Cancel",
			cancelTime: startTime.Add(30 * time.Minute),
			wantLate:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.IsLateCancellation(startTime, tt.cancelTime)
			if got != tt.wantLate {
				t.Errorf("IsLateCancellation(%v, %v) = %v; want %v", startTime, tt.cancelTime, got, tt.wantLate)
			}
		})
	}
}

func TestBookingStatusValidation(t *testing.T) {
	validStatuses := []domain.BookingStatus{
		domain.BookingStatusConfirmed,
		domain.BookingStatusCancelled,
		domain.BookingStatusLateCancelled,
		domain.BookingStatusAttended,
		domain.BookingStatusNoShow,
	}

	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("BookingStatus %s should be valid", s)
		}
	}

	invalidStatus := domain.BookingStatus("UNKNOWN_STATUS")
	if invalidStatus.IsValid() {
		t.Errorf("BookingStatus %s should be invalid", invalidStatus)
	}
}

func TestScheduleStatusValidation(t *testing.T) {
	validStatuses := []domain.ScheduleStatus{
		domain.ScheduleStatusScheduled,
		domain.ScheduleStatusCancelled,
		domain.ScheduleStatusCompleted,
	}

	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("ScheduleStatus %s should be valid", s)
		}
	}

	invalidStatus := domain.ScheduleStatus("PENDING")
	if invalidStatus.IsValid() {
		t.Errorf("ScheduleStatus %s should be invalid", invalidStatus)
	}
}

func TestWaitlistStatusValidation(t *testing.T) {
	validStatuses := []domain.WaitlistStatus{
		domain.WaitlistStatusWaiting,
		domain.WaitlistStatusPromoted,
		domain.WaitlistStatusExpired,
		domain.WaitlistStatusCancelled,
	}

	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("WaitlistStatus %s should be valid", s)
		}
	}

	invalidStatus := domain.WaitlistStatus("INVALID")
	if invalidStatus.IsValid() {
		t.Errorf("WaitlistStatus %s should be invalid", invalidStatus)
	}
}
