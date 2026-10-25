package domain

import (
	"time"

	"github.com/google/uuid"
)

type BranchRevenue struct {
	BranchID     uuid.UUID `json:"branch_id"`
	BranchCode   string    `json:"branch_code"`
	BranchName   string    `json:"branch_name"`
	RevenueKurus int64     `json:"revenue_kurus"`
	InvoiceCount int       `json:"invoice_count"`
}

type RevenueReport struct {
	StartDate         time.Time       `json:"start_date"`
	EndDate           time.Time       `json:"end_date"`
	TotalRevenueKurus int64           `json:"total_revenue_kurus"`
	TotalInvoices     int             `json:"total_invoices"`
	PaidInvoices      int             `json:"paid_invoices"`
	Branches          []BranchRevenue `json:"branches"`
}

type HourlyOccupancySlot struct {
	Hour            int `json:"hour"`
	Entries         int `json:"entries"`
	Exits           int `json:"exits"`
	EstimatedInside int `json:"estimated_inside"`
}

type HourlyOccupancyReport struct {
	BranchID   *uuid.UUID            `json:"branch_id,omitempty"`
	BranchName string                `json:"branch_name,omitempty"`
	Date       string                `json:"date"`
	PeakHour   int                   `json:"peak_hour"`
	PeakCount  int                   `json:"peak_count"`
	Slots      []HourlyOccupancySlot `json:"slots"`
}

type TrainerPerformance struct {
	TrainerID           uuid.UUID `json:"trainer_id"`
	TrainerName         string    `json:"trainer_name"`
	TotalClasses        int       `json:"total_classes"`
	TotalClassAttendees int       `json:"total_class_attendees"`
	TotalPTSessions     int       `json:"total_pt_sessions"`
	AverageOccupancyPct float64   `json:"average_occupancy_pct"`
}

type TrainerReport struct {
	StartDate time.Time            `json:"start_date"`
	EndDate   time.Time            `json:"end_date"`
	Trainers  []TrainerPerformance `json:"trainers"`
}
