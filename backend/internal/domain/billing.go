package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type InvoiceStatus string

const (
	InvoiceStatusPending   InvoiceStatus = "PENDING"
	InvoiceStatusPaid      InvoiceStatus = "PAID"
	InvoiceStatusRefunded  InvoiceStatus = "REFUNDED"
	InvoiceStatusCancelled InvoiceStatus = "CANCELLED"
)

func (s InvoiceStatus) IsValid() bool {
	return s == InvoiceStatusPending || s == InvoiceStatusPaid || s == InvoiceStatusRefunded || s == InvoiceStatusCancelled
}

type PaymentStatus string

const (
	PaymentStatusSuccess PaymentStatus = "SUCCESS"
	PaymentStatusFailed  PaymentStatus = "FAILED"
	PaymentStatusPending PaymentStatus = "PENDING"
)

type AutoRenewalStatus string

const (
	AutoRenewalPending   AutoRenewalStatus = "PENDING"
	AutoRenewalSuccess   AutoRenewalStatus = "SUCCESS"
	AutoRenewalFailed    AutoRenewalStatus = "FAILED"
	AutoRenewalAbandoned AutoRenewalStatus = "ABANDONED"
)

var (
	ErrInvoiceNotFound         = errors.New("fatura bulunamadi")
	ErrInvoiceAlreadyPaid      = errors.New("fatura zaten odenmis")
	ErrPaymentFailed           = errors.New("odeme islemi basarisiz oldu")
	ErrDuplicatePaymentKey     = errors.New("bu idempotency key ile daha once islem yapildi")
	ErrInvalidInvoiceAmount    = errors.New("fatura tutari 0'dan kucuk olamaz")
	ErrAutoRenewalNotFound     = errors.New("otomatik yenileme kaydi bulunamadi")
	ErrCardTokenRequired       = errors.New("otomatik yenileme icin kayitli kart tokeni zorunludur")
	ErrSuspensionThresholdMet  = errors.New("7 gun boyunca tahsil edilemedi, uyelik askiya alindi")
)

// InvoiceSequence tracks the gapless sequence value for a branch in a given year.
type InvoiceSequence struct {
	Year       int       `json:"year"`
	BranchID   uuid.UUID `json:"branch_id"`
	CurrentVal int64     `json:"current_val"`
}

// Invoice represents a financial tax receipt / invoice.
type Invoice struct {
	ID              uuid.UUID     `json:"id"`
	InvoiceNumber   string        `json:"invoice_number"`
	MemberID        uuid.UUID     `json:"member_id"`
	BranchID        uuid.UUID     `json:"branch_id"`
	MemberPackageID *uuid.UUID    `json:"member_package_id,omitempty"`
	Year            int           `json:"year"`
	SequenceNumber  int64         `json:"sequence_number"`
	AmountKurus     int64         `json:"amount_kurus"`
	TaxKurus        int64         `json:"tax_kurus"`
	Status          InvoiceStatus `json:"status"`
	PaymentMethod   string        `json:"payment_method"`
	CreatedAt       time.Time     `json:"created_at"`

	// Enriched fields
	MemberName  string `json:"member_name,omitempty"`
	MemberEmail string `json:"member_email,omitempty"`
	BranchName  string `json:"branch_name,omitempty"`
	BranchCode  string `json:"branch_code,omitempty"`
	PackageName string `json:"package_name,omitempty"`
}

// Payment records a payment attempt (or success) towards an invoice.
type Payment struct {
	ID             uuid.UUID     `json:"id"`
	InvoiceID      uuid.UUID     `json:"invoice_id"`
	MemberID       uuid.UUID     `json:"member_id"`
	AmountKurus    int64         `json:"amount_kurus"`
	Status         PaymentStatus `json:"status"`
	ProviderTxID   *string       `json:"provider_tx_id,omitempty"`
	ErrorCode      *string       `json:"error_code,omitempty"`
	ErrorMessage   *string       `json:"error_message,omitempty"`
	IdempotencyKey *string       `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
}

// AutoRenewalQueue represents a scheduled recurring payment attempt.
type AutoRenewalQueue struct {
	ID              uuid.UUID         `json:"id"`
	MemberPackageID uuid.UUID         `json:"member_package_id"`
	AttemptNumber   int               `json:"attempt_number"`
	ScheduledFor    time.Time         `json:"scheduled_for"`
	Status          AutoRenewalStatus `json:"status"`
	LastAttemptAt   *time.Time        `json:"last_attempt_at,omitempty"`
	ErrorReason     *string           `json:"error_reason,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`

	// Enriched fields
	MemberID    uuid.UUID `json:"member_id,omitempty"`
	MemberName  string    `json:"member_name,omitempty"`
	MemberEmail string    `json:"member_email,omitempty"`
	PackageName string    `json:"package_name,omitempty"`
	PriceKurus  int64     `json:"price_kurus,omitempty"`
	CardToken   *string   `json:"card_token,omitempty"`
}

// FormatInvoiceNumber builds the standard gapless invoice serial: FIT-{YEAR}-{BRANCH_CODE}-{SEQ_NO:06d}.
// Example: FIT-2026-KDK01-000001
func FormatInvoiceNumber(year int, branchCode string, sequenceNumber int64) string {
	return fmt.Sprintf("FIT-%04d-%s-%06d", year, branchCode, sequenceNumber)
}

// CalculateNextRetry returns the next scheduled date based on the 3-attempt + 7-day suspension calendar.
// - Attempt 1 failed -> next is attempt 2 at baseDate + 2 days (Day 3)
// - Attempt 2 failed -> next is attempt 3 at baseDate + 2 days (Day 5)
// - Attempt 3 failed -> at baseDate + 2 days (Day 7), no further retry; triggers suspension!
func CalculateNextRetry(attemptNumber int, fromDate time.Time) (time.Time, bool) {
	if attemptNumber < 3 {
		// Schedule next attempt 2 days later
		return fromDate.AddDate(0, 0, 2), true
	}
	// 3 attempts have been exhausted; day 7 triggers suspension
	return fromDate.AddDate(0, 0, 2), false
}
