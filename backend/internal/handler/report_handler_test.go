package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/handler"
	"fitgiris/internal/service"
)

type mockReportRepo struct{}

func (m *mockReportRepo) GetRevenueReport(ctx context.Context, from, to time.Time, branchID *uuid.UUID) (*domain.RevenueReport, error) {
	return &domain.RevenueReport{
		StartDate:         from,
		EndDate:           to,
		TotalRevenueKurus: 125000000,
		TotalInvoices:     50,
		PaidInvoices:      48,
		Branches: []domain.BranchRevenue{
			{
				BranchID:     uuid.New(),
				BranchCode:   "IST01",
				BranchName:   "Beşiktaş",
				RevenueKurus: 75000000,
				InvoiceCount: 30,
			},
		},
	}, nil
}

func (m *mockReportRepo) GetHourlyOccupancy(ctx context.Context, branchID *uuid.UUID, date time.Time) (*domain.HourlyOccupancyReport, error) {
	return &domain.HourlyOccupancyReport{
		Date:      date.Format("2006-01-02"),
		PeakHour:  19,
		PeakCount: 45,
		Slots: []domain.HourlyOccupancySlot{
			{Hour: 19, Entries: 45, Exits: 10, EstimatedInside: 35},
		},
	}, nil
}

func (m *mockReportRepo) GetTrainerReport(ctx context.Context, from, to time.Time) (*domain.TrainerReport, error) {
	return &domain.TrainerReport{
		StartDate: from,
		EndDate:   to,
		Trainers: []domain.TrainerPerformance{
			{
				TrainerID:           uuid.New(),
				TrainerName:         "Murat Eğitmen",
				TotalClasses:        20,
				TotalClassAttendees: 380,
				TotalPTSessions:     15,
				AverageOccupancyPct: 95.0,
			},
		},
	}, nil
}

func TestReportHandler_Endpoints(t *testing.T) {
	svc := service.NewReportService(&mockReportRepo{})
	h := handler.NewReportHandler(svc)

	r := chi.NewRouter()
	r.Get("/reports/revenue", h.RevenueReport)
	r.Get("/reports/occupancy", h.OccupancyReport)
	r.Get("/reports/trainers", h.TrainerReport)

	// Test Revenue Report
	req := httptest.NewRequest("GET", "/reports/revenue?from=2026-01-01&to=2026-02-01", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for revenue report, got %d", rec.Code)
	}

	var revEnv struct {
		Data domain.RevenueReport `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &revEnv)
	revReport := revEnv.Data
	if revReport.TotalRevenueKurus != 125000000 {
		t.Fatalf("unexpected revenue: %d", revReport.TotalRevenueKurus)
	}

	// Test Occupancy Report
	req = httptest.NewRequest("GET", "/reports/occupancy?date=2026-01-15", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for occupancy report, got %d", rec.Code)
	}

	var occEnv struct {
		Data domain.HourlyOccupancyReport `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &occEnv)
	occReport := occEnv.Data
	if occReport.PeakHour != 19 || occReport.PeakCount != 45 {
		t.Fatalf("unexpected occupancy peak: %+v", occReport)
	}

	// Test Trainer Report
	req = httptest.NewRequest("GET", "/reports/trainers", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for trainer report, got %d", rec.Code)
	}

	var trEnv struct {
		Data domain.TrainerReport `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &trEnv)
	trReport := trEnv.Data
	if len(trReport.Trainers) != 1 || trReport.Trainers[0].AverageOccupancyPct != 95.0 {
		t.Fatalf("unexpected trainer report: %+v", trReport)
	}
}
