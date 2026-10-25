package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PackageKind defines whether a package is based on time duration or number of entries.
type PackageKind string

const (
	PackageKindDuration PackageKind = "DURATION"
	PackageKindSession  PackageKind = "SESSION"
)

func (k PackageKind) IsValid() bool {
	return k == PackageKindDuration || k == PackageKindSession
}

// PackageType represents an available gym membership package template.
type PackageType struct {
	ID             uuid.UUID   `json:"id"`
	Name           string      `json:"name"`
	Type           PackageKind `json:"type"`
	DurationMonths int         `json:"duration_months"`
	TotalEntries   int         `json:"total_entries"`
	PriceKurus     int64       `json:"price_kurus"`
	AllBranches    bool        `json:"all_branches"`
	MaxFreezeDays  int         `json:"max_freeze_days"`
	IsActive       bool        `json:"is_active"`
	CreatedAt      time.Time   `json:"created_at"`
}

// MembershipStatus represents the finite state machine states for a member's package.
type MembershipStatus string

const (
	StatusPendingPayment MembershipStatus = "PENDING_PAYMENT"
	StatusActive         MembershipStatus = "ACTIVE"
	StatusFrozen         MembershipStatus = "FROZEN"
	StatusTransferred    MembershipStatus = "TRANSFERRED"
	StatusSuspended      MembershipStatus = "SUSPENDED"
	StatusExpired        MembershipStatus = "EXPIRED"
	StatusCancelled      MembershipStatus = "CANCELLED"
)

// AllowedStatusTransitions defines the strict Finite State Machine (FSM) rules.
var AllowedStatusTransitions = map[MembershipStatus]map[MembershipStatus]bool{
	StatusPendingPayment: {
		StatusActive:    true,
		StatusCancelled: true,
	},
	StatusActive: {
		StatusFrozen:      true,
		StatusTransferred: true,
		StatusSuspended:   true,
		StatusExpired:     true,
		StatusCancelled:   true,
	},
	StatusFrozen: {
		StatusActive:    true,
		StatusCancelled: true,
		StatusExpired:   true,
	},
	StatusSuspended: {
		StatusActive:    true,
		StatusCancelled: true,
		StatusExpired:   true,
	},
	// Terminal states have no valid subsequent transitions
	StatusTransferred: {},
	StatusExpired:     {},
	StatusCancelled:   {},
}

// CanTransitionTo returns true if transitioning from s to next is permitted by the FSM.
func (s MembershipStatus) CanTransitionTo(next MembershipStatus) bool {
	allowed, exists := AllowedStatusTransitions[s]
	if !exists {
		return false
	}
	return allowed[next]
}

// ValidateTransition checks whether transitioning from current to next is allowed.
// Returns ErrInvalidStateTransition if disallowed.
func ValidateTransition(current, next MembershipStatus) error {
	if !current.CanTransitionTo(next) {
		return fmt.Errorf("%w: %s -> %s gecisi yasaktir", ErrInvalidStateTransition, current, next)
	}
	return nil
}

// MemberPackage represents a purchased gym membership instance.
type MemberPackage struct {
	ID                        uuid.UUID        `json:"id"`
	MemberID                  uuid.UUID        `json:"member_id"`
	PackageTypeID             uuid.UUID        `json:"package_type_id"`
	BranchID                  uuid.UUID        `json:"branch_id"`
	Status                    MembershipStatus `json:"status"`
	StartDate                 time.Time        `json:"start_date"`
	EndDate                   time.Time        `json:"end_date"`
	RemainingEntries          int              `json:"remaining_entries"`
	TotalEntries              int              `json:"total_entries"`
	PricePaidKurus            int64            `json:"price_paid_kurus"`
	AutoRenew                 bool             `json:"auto_renew"`
	CardToken                 *string          `json:"card_token,omitempty"`
	ConsecutiveFailedAttempts int              `json:"consecutive_failed_attempts"`
	NextRetryDate             *time.Time       `json:"next_retry_date,omitempty"`
	CreatedAt                 time.Time        `json:"created_at"`
	UpdatedAt                 time.Time        `json:"updated_at"`

	// Enriched fields from joins (optional)
	PackageName   string      `json:"package_name,omitempty"`
	PackageKind   PackageKind `json:"package_kind,omitempty"`
	AllBranches   bool        `json:"all_branches,omitempty"`
	MaxFreezeDays int         `json:"max_freeze_days,omitempty"`
	BranchName    string      `json:"branch_name,omitempty"`
	MemberName    string      `json:"member_name,omitempty"`
	MemberEmail   string      `json:"member_email,omitempty"`
}

// PackageFreeze records a temporary freeze of a membership package.
type PackageFreeze struct {
	ID                 uuid.UUID  `json:"id"`
	MemberPackageID    uuid.UUID  `json:"member_package_id"`
	FreezeStartDate    time.Time  `json:"freeze_start_date"`
	FreezeEndDate      time.Time  `json:"freeze_end_date"`
	ActualUnfreezeDate *time.Time `json:"actual_unfreeze_date,omitempty"`
	DaysFrozen         int        `json:"days_frozen"`
	Reason             *string    `json:"reason,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// PackageTransfer records ownership transfer of an active membership between users.
type PackageTransfer struct {
	ID              uuid.UUID `json:"id"`
	FromMemberID    uuid.UUID `json:"from_member_id"`
	ToMemberID      uuid.UUID `json:"to_member_id"`
	SourcePackageID uuid.UUID `json:"source_package_id"`
	NewPackageID    uuid.UUID `json:"new_package_id"`
	TransferredDays int       `json:"transferred_days"`
	CreatedAt       time.Time `json:"created_at"`
}

// UpgradeQuote contains calculations for prorated membership upgrade.
type UpgradeQuote struct {
	MemberPackageID     uuid.UUID   `json:"member_package_id"`
	CurrentPackageName  string      `json:"current_package_name"`
	NewPackageTypeID    uuid.UUID   `json:"new_package_type_id"`
	NewPackageName      string      `json:"new_package_name"`
	TotalDays           int         `json:"total_days"`
	RemainingDays       int         `json:"remaining_days"`
	PricePaidKurus      int64       `json:"price_paid_kurus"`
	RemainingValueKurus int64       `json:"remaining_value_kurus"`
	NewPackagePriceKurus int64      `json:"new_package_price_kurus"`
	AmountToPayKurus    int64       `json:"amount_to_pay_kurus"`
}

// Error codes and domain sentinel errors
const (
	ErrCodeInvalidStateTransition = "ERR_INVALID_STATE_TRANSITION"
	ErrCodeFreezeLimitExceeded    = "ERR_FREEZE_LIMIT_EXCEEDED"
	ErrCodeInsufficientEntries    = "ERR_INSUFFICIENT_ENTRIES"
	ErrCodePackageInactive        = "ERR_PACKAGE_INACTIVE"
)

var (
	ErrInvalidStateTransition = errors.New("gecersiz durum gecisi")
	ErrFreezeLimitExceeded    = errors.New("dondurma gun siniri asildi")
	ErrMembershipNotFound     = errors.New("uyelik bulunamadi")
	ErrPackageTypeNotFound    = errors.New("paket tipi bulunamadi")
	ErrPackageTypeInactive    = errors.New("paket tipi aktif degil")
	ErrCannotTransferToSelf   = errors.New("uyelik kendinize devredilemez")
	ErrMembershipNotActive    = errors.New("uyelik aktif durumda degil")
	ErrMembershipNotFrozen    = errors.New("uyelik dondurulmus durumda degil")
	ErrNoActiveFreeze         = errors.New("aktif dondurma kaydi bulunamadi")
	ErrSamePackageTypeUpgrade = errors.New("ayni paket tipine yukseltme yapilamaz")
	ErrInsufficientEntries    = errors.New("kalan giris hakki yetersiz")
	ErrMembershipExpired      = errors.New("uyelik suresi dolmus")
)
