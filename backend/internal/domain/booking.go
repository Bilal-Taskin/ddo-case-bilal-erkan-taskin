package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// CancellationDeadlineHours defines the threshold for free cancellation (2 hours before class).
const CancellationDeadlineHours = 2

// ScheduleStatus represents the status of a scheduled class.
type ScheduleStatus string

const (
	ScheduleStatusScheduled ScheduleStatus = "SCHEDULED"
	ScheduleStatusCancelled ScheduleStatus = "CANCELLED"
	ScheduleStatusCompleted ScheduleStatus = "COMPLETED"
)

func (s ScheduleStatus) IsValid() bool {
	return s == ScheduleStatusScheduled || s == ScheduleStatusCancelled || s == ScheduleStatusCompleted
}

// BookingStatus represents the status of a member's class booking.
type BookingStatus string

const (
	BookingStatusConfirmed     BookingStatus = "CONFIRMED"
	BookingStatusCancelled     BookingStatus = "CANCELLED"
	BookingStatusLateCancelled BookingStatus = "LATE_CANCELLED"
	BookingStatusAttended      BookingStatus = "ATTENDED"
	BookingStatusNoShow        BookingStatus = "NO_SHOW"
)

func (s BookingStatus) IsValid() bool {
	switch s {
	case BookingStatusConfirmed, BookingStatusCancelled, BookingStatusLateCancelled, BookingStatusAttended, BookingStatusNoShow:
		return true
	default:
		return false
	}
}

// WaitlistStatus represents the status of a waitlist entry.
type WaitlistStatus string

const (
	WaitlistStatusWaiting   WaitlistStatus = "WAITING"
	WaitlistStatusPromoted  WaitlistStatus = "PROMOTED"
	WaitlistStatusExpired   WaitlistStatus = "EXPIRED"
	WaitlistStatusCancelled WaitlistStatus = "CANCELLED"
)

func (s WaitlistStatus) IsValid() bool {
	switch s {
	case WaitlistStatusWaiting, WaitlistStatusPromoted, WaitlistStatusExpired, WaitlistStatusCancelled:
		return true
	default:
		return false
	}
}

// Common Domain Errors for Classes, Bookings and Waitlist
var (
	ErrClassNotFound            = errors.New("ders tanimi bulunamadi")
	ErrScheduleNotFound         = errors.New("ders seansi bulunamadi")
	ErrBookingNotFound          = errors.New("rezervasyon kaydi bulunamadi")
	ErrWaitlistNotFound         = errors.New("bekleme listesi kaydi bulunamadi")
	ErrClassAlreadyBooked       = errors.New("bu ders seansina zaten aktif rezervasyonunuz bulunmaktadir")
	ErrClassAlreadyInWaitlist   = errors.New("bu ders seansinin bekleme listesinde zaten kaydiniz bulunmaktadir")
	ErrScheduleTimeConflict     = errors.New("ayni saatte baska bir derse rezervasyonunuz bulunmaktadir")
	ErrSchedulePast             = errors.New("gecmis tarihli veya baslamis bir ders seansina rezervasyon yapilamaz")
	ErrScheduleCancelled        = errors.New("iptal edilmis ders seansina islem yapilamaz")
	ErrScheduleCompleted        = errors.New("tamamlanmis ders seansina islem yapilamaz")
	ErrBookingAlreadyCancelled  = errors.New("rezervasyon kaydi zaten iptal edilmis")
	ErrWaitlistAlreadyCancelled = errors.New("bekleme listesi kaydi zaten iptal edilmis")
	ErrCannotCancelPastClass    = errors.New("baslamis veya bitmis bir ders seansi icin iptal yapilamaz")
	ErrNoActiveMembership       = errors.New("ders rezervasyonu yapabilmek icin aktif bir uyeliginiz bulunmalidir")
	ErrBranchAccessDenied       = errors.New("uyelik paketiniz bu subedeki dersleri kapsamamaktadir")
	ErrInvalidAttendanceStatus  = errors.New("gecersiz yoklama durumu (sadece ATTENDED veya NO_SHOW olabilir)")
)

// Class represents a fitness class template.
type Class struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	DurationMinutes int       `json:"duration_minutes"`
	DefaultCapacity int       `json:"default_capacity"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

// ClassSchedule represents a scheduled occurrence of a class.
type ClassSchedule struct {
	ID          uuid.UUID      `json:"id"`
	ClassID     uuid.UUID      `json:"class_id"`
	BranchID    uuid.UUID      `json:"branch_id"`
	TrainerID   uuid.UUID      `json:"trainer_id"`
	StartTime   time.Time      `json:"start_time"`
	EndTime     time.Time      `json:"end_time"`
	Capacity    int            `json:"capacity"`
	BookedCount int            `json:"booked_count"`
	Status      ScheduleStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`

	// Enriched fields for API responses
	ClassName      string `json:"class_name,omitempty"`
	BranchName     string `json:"branch_name,omitempty"`
	TrainerName    string `json:"trainer_name,omitempty"`
	WaitlistCount  int    `json:"waitlist_count,omitempty"`
	AvailableSpots int    `json:"available_spots,omitempty"`
}

// ClassBooking represents an individual booking by a member for a scheduled class.
type ClassBooking struct {
	ID             uuid.UUID     `json:"id"`
	ScheduleID     uuid.UUID     `json:"schedule_id"`
	MemberID       uuid.UUID     `json:"member_id"`
	Status         BookingStatus `json:"status"`
	CancelledAt    *time.Time    `json:"cancelled_at,omitempty"`
	PenaltyApplied bool          `json:"penalty_applied"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`

	// Enriched fields
	ClassName   string    `json:"class_name,omitempty"`
	BranchName  string    `json:"branch_name,omitempty"`
	TrainerName string    `json:"trainer_name,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	MemberName  string    `json:"member_name,omitempty"`
	MemberEmail string    `json:"member_email,omitempty"`
}

// ClassWaitlist represents a member waiting for an available spot in a full class.
type ClassWaitlist struct {
	ID         uuid.UUID      `json:"id"`
	ScheduleID uuid.UUID      `json:"schedule_id"`
	MemberID   uuid.UUID      `json:"member_id"`
	Status     WaitlistStatus `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	PromotedAt *time.Time     `json:"promoted_at,omitempty"`

	// Enriched fields
	Position    int       `json:"position,omitempty"`
	ClassName   string    `json:"class_name,omitempty"`
	BranchName  string    `json:"branch_name,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	MemberName  string    `json:"member_name,omitempty"`
	MemberEmail string    `json:"member_email,omitempty"`
}

// BookingResult provides the outcome of a class booking attempt.
type BookingResult struct {
	IsWaitlisted     bool           `json:"is_waitlisted"`
	Booking          *ClassBooking  `json:"booking,omitempty"`
	Waitlist         *ClassWaitlist `json:"waitlist,omitempty"`
	WaitlistPosition int            `json:"waitlist_position,omitempty"`
	Message          string         `json:"message"`
}

// CancelBookingResult provides the outcome of a cancellation.
type CancelBookingResult struct {
	Booking          *ClassBooking `json:"booking"`
	IsLate           bool          `json:"is_late"`
	PenaltyApplied   bool          `json:"penalty_applied"`
	PromotedMemberID *uuid.UUID    `json:"promoted_member_id,omitempty"`
	Message          string        `json:"message"`
}

// IsLateCancellation checks if the cancellation occurs strictly less than 2 hours before the class starts.
// If cancelTime is after or less than 2 hours before startTime, returns true (late cancel).
// If cancelTime is 2 hours or more before startTime, returns false (timely cancel).
func IsLateCancellation(startTime, cancelTime time.Time) bool {
	return startTime.Sub(cancelTime) < CancellationDeadlineHours*time.Hour
}
