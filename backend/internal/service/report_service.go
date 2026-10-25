package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/repository"
)

type ReportService interface {
	GetRevenueReport(ctx context.Context, from, to time.Time, branchID *uuid.UUID) (*domain.RevenueReport, error)
	GetHourlyOccupancy(ctx context.Context, branchID *uuid.UUID, date time.Time) (*domain.HourlyOccupancyReport, error)
	GetTrainerReport(ctx context.Context, from, to time.Time) (*domain.TrainerReport, error)
}

type reportService struct {
	reportRepo repository.ReportRepository
}

func NewReportService(reportRepo repository.ReportRepository) ReportService {
	return &reportService{reportRepo: reportRepo}
}

func (s *reportService) GetRevenueReport(ctx context.Context, from, to time.Time, branchID *uuid.UUID) (*domain.RevenueReport, error) {
	if from.IsZero() {
		from = time.Now().AddDate(0, -1, 0) // default 1 month ago
	}
	if to.IsZero() {
		to = time.Now()
	}
	return s.reportRepo.GetRevenueReport(ctx, from, to, branchID)
}

func (s *reportService) GetHourlyOccupancy(ctx context.Context, branchID *uuid.UUID, date time.Time) (*domain.HourlyOccupancyReport, error) {
	if date.IsZero() {
		date = time.Now()
	}
	return s.reportRepo.GetHourlyOccupancy(ctx, branchID, date)
}

func (s *reportService) GetTrainerReport(ctx context.Context, from, to time.Time) (*domain.TrainerReport, error) {
	if from.IsZero() {
		from = time.Now().AddDate(0, -1, 0)
	}
	if to.IsZero() {
		to = time.Now()
	}
	return s.reportRepo.GetTrainerReport(ctx, from, to)
}
