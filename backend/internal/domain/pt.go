package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPTNoSessionsRemaining = errors.New("ERR_PT_NO_SESSIONS_REMAINING")
	ErrPTTrainerConflict     = errors.New("ERR_PT_TRAINER_CONFLICT")
	ErrPTInvalidTime         = errors.New("ERR_PT_INVALID_TIME")
	ErrPTNotFound            = errors.New("ERR_PT_NOT_FOUND")
	ErrPTInvalidStatus       = errors.New("ERR_PT_INVALID_STATUS")
	ErrPTAlreadyFinished     = errors.New("ERR_PT_ALREADY_FINISHED")
)

const (
	PTStatusConfirmed = "CONFIRMED"
	PTStatusCompleted = "COMPLETED"
	PTStatusCancelled = "CANCELLED"
	PTStatusNoShow    = "NO_SHOW"
)

type PTPackage struct {
	ID                uuid.UUID `json:"id"`
	MemberID          uuid.UUID `json:"member_id"`
	TrainerID         uuid.UUID `json:"trainer_id"`
	TotalSessions     int       `json:"total_sessions"`
	RemainingSessions int       `json:"remaining_sessions"`
	PriceKurus        int64     `json:"price_kurus"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`

	// Enriched fields
	MemberName  string `json:"member_name,omitempty"`
	TrainerName string `json:"trainer_name,omitempty"`
}

type PTAppointment struct {
	ID          uuid.UUID `json:"id"`
	PTPackageID uuid.UUID `json:"pt_package_id"`
	MemberID    uuid.UUID `json:"member_id"`
	TrainerID   uuid.UUID `json:"trainer_id"`
	BranchID    uuid.UUID `json:"branch_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Status      string    `json:"status"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Enriched fields
	MemberName  string `json:"member_name,omitempty"`
	TrainerName string `json:"trainer_name,omitempty"`
	BranchName  string `json:"branch_name,omitempty"`
}

// CanBook checks if the package allows booking an appointment.
func (p *PTPackage) CanBook() error {
	if !p.IsActive {
		return errors.New("ERR_PT_PACKAGE_INACTIVE")
	}
	if p.RemainingSessions <= 0 {
		return ErrPTNoSessionsRemaining
	}
	return nil
}

// DeductSession reduces one session from remaining.
func (p *PTPackage) DeductSession() error {
	if err := p.CanBook(); err != nil {
		return err
	}
	p.RemainingSessions--
	return nil
}

// RefundSession restores one session.
func (p *PTPackage) RefundSession() {
	if p.RemainingSessions < p.TotalSessions {
		p.RemainingSessions++
	}
}
