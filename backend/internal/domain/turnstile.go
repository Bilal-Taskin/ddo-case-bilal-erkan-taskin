package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type TurnstileDirection string

const (
	DirectionIn  TurnstileDirection = "IN"
	DirectionOut TurnstileDirection = "OUT"
)

func (d TurnstileDirection) IsValid() bool {
	return d == DirectionIn || d == DirectionOut
}

type AccessStatus string

const (
	AccessGranted AccessStatus = "GRANTED"
	AccessDenied  AccessStatus = "DENIED"
)

// Anti-passback window is strictly 2 hours (120 minutes)
const AntiPassbackDuration = 2 * time.Hour

// Access decision reason codes
const (
	ReasonSuccess              = "SUCCESS"
	ReasonTurnstileNotFound    = "ERR_TURNSTILE_NOT_FOUND"
	ReasonTurnstileInactive    = "ERR_TURNSTILE_INACTIVE"
	ReasonMemberNotFound       = "ERR_MEMBER_NOT_FOUND"
	ReasonMemberInactive       = "ERR_MEMBER_INACTIVE"
	ReasonMembershipInactive   = "ERR_MEMBERSHIP_INACTIVE"
	ReasonMembershipExpired    = "ERR_MEMBERSHIP_EXPIRED"
	ReasonInsufficientEntries  = "ERR_INSUFFICIENT_ENTRIES"
	ReasonBranchAccessDenied   = "ERR_BRANCH_ACCESS_DENIED"
	ReasonAntiPassback         = "ERR_ANTI_PASSBACK"
	ReasonOfflineAnomaly       = "FLAGGED_OFFLINE_ANOMALY"
)

var (
	ErrTurnstileNotFound    = errors.New("turnike bulunamadi")
	ErrTurnstileInactive    = errors.New("turnike aktif degil")
	ErrAntiPassback         = errors.New("anti-passback ihlali: 2 saat icinde cikis yapilmadan tekrar girilemez")
	ErrTurnstileCodeExists  = errors.New("bu turnike kodu bu subede zaten mevcut")
)

// Turnstile represents a physical gate at a branch.
type Turnstile struct {
	ID        uuid.UUID          `json:"id"`
	BranchID  uuid.UUID          `json:"branch_id"`
	Code      string             `json:"code"`
	Name      string             `json:"name"`
	Direction TurnstileDirection `json:"direction"`
	IsActive  bool               `json:"is_active"`
	CreatedAt time.Time          `json:"created_at"`

	// Enriched fields
	BranchName string `json:"branch_name,omitempty"`
	BranchCode string `json:"branch_code,omitempty"`
}

// AccessLog represents an access attempt at a turnstile.
type AccessLog struct {
	ID            uuid.UUID          `json:"id"`
	MemberID      uuid.UUID          `json:"member_id"`
	TurnstileID   uuid.UUID          `json:"turnstile_id"`
	BranchID      uuid.UUID          `json:"branch_id"`
	Direction     TurnstileDirection `json:"direction"`
	Status        AccessStatus       `json:"status"`
	Reason        string             `json:"reason"`
	OfflineSynced bool               `json:"offline_synced"`
	CreatedAt     time.Time          `json:"created_at"`

	// Enriched fields
	MemberName    string `json:"member_name,omitempty"`
	MemberEmail   string `json:"member_email,omitempty"`
	TurnstileName string `json:"turnstile_name,omitempty"`
	TurnstileCode string `json:"turnstile_code,omitempty"`
	BranchName    string `json:"branch_name,omitempty"`
}

// AccessDecision is returned after evaluating a turnstile pass attempt.
type AccessDecision struct {
	Granted          bool               `json:"granted"`
	Direction        TurnstileDirection `json:"direction"`
	Status           AccessStatus       `json:"status"`
	Reason           string             `json:"reason"`
	MemberID         uuid.UUID          `json:"member_id"`
	TurnstileID      uuid.UUID          `json:"turnstile_id"`
	BranchID         uuid.UUID          `json:"branch_id"`
	RemainingEntries *int               `json:"remaining_entries,omitempty"`
	LatencyMs        int64              `json:"latency_ms"`
	Timestamp        time.Time          `json:"timestamp"`
}

// OfflineSyncItem is a single offline event received from a turnstile controller.
type OfflineSyncItem struct {
	TurnstileCode string             `json:"turnstile_code"`
	MemberID      uuid.UUID          `json:"member_id"`
	Direction     TurnstileDirection `json:"direction"`
	Status        AccessStatus       `json:"status"`
	Reason        string             `json:"reason"`
	OccurredAt    time.Time          `json:"occurred_at"`
}

// OfflineSyncResult reports the outcome of processing offline sync batches.
type OfflineSyncResult struct {
	TotalProcessed    int         `json:"total_processed"`
	AnomaliesDetected int         `json:"anomalies_detected"`
	SyncedLogs        []AccessLog `json:"synced_logs"`
}

// CheckAntiPassback determines whether an entry request violates anti-passback rule.
// Anti-passback rule: If the member previously entered (direction == IN) within 2 hours
// without an intervening exit (direction == OUT), entry must be blocked.
func CheckAntiPassback(lastDirection TurnstileDirection, lastAccessTime, requestTime time.Time) bool {
	if lastDirection != DirectionIn {
		return false
	}
	diff := requestTime.Sub(lastAccessTime)
	return diff >= 0 && diff < AntiPassbackDuration
}
