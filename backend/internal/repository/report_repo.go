package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitgiris/internal/domain"
)

type ReportRepository interface {
	GetRevenueReport(ctx context.Context, from, to time.Time, branchID *uuid.UUID) (*domain.RevenueReport, error)
	GetHourlyOccupancy(ctx context.Context, branchID *uuid.UUID, date time.Time) (*domain.HourlyOccupancyReport, error)
	GetTrainerReport(ctx context.Context, from, to time.Time) (*domain.TrainerReport, error)
}

type postgresReportRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReportRepository(pool *pgxpool.Pool) ReportRepository {
	return &postgresReportRepository{pool: pool}
}

func (r *postgresReportRepository) GetRevenueReport(ctx context.Context, from, to time.Time, branchID *uuid.UUID) (*domain.RevenueReport, error) {
	// Summary totals
	summaryQuery := `
		SELECT 
			COALESCE(SUM(CASE WHEN status = 'PAID' THEN amount_kurus ELSE 0 END), 0) as total_rev,
			COUNT(id) as total_inv,
			COUNT(CASE WHEN status = 'PAID' THEN 1 END) as paid_inv
		FROM invoices
		WHERE created_at >= $1 AND created_at <= $2
	`
	args := []interface{}{from, to}
	if branchID != nil {
		summaryQuery += " AND branch_id = $3"
		args = append(args, *branchID)
	}

	report := &domain.RevenueReport{
		StartDate: from,
		EndDate:   to,
		Branches:  []domain.BranchRevenue{},
	}

	err := r.pool.QueryRow(ctx, summaryQuery, args...).Scan(
		&report.TotalRevenueKurus,
		&report.TotalInvoices,
		&report.PaidInvoices,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch revenue summary: %w", err)
	}

	// Branch breakdown
	branchQuery := `
		SELECT 
			b.id, b.code, b.name,
			COALESCE(SUM(CASE WHEN i.status = 'PAID' THEN i.amount_kurus ELSE 0 END), 0) as branch_rev,
			COUNT(i.id) as inv_count
		FROM branches b
		LEFT JOIN invoices i ON b.id = i.branch_id AND i.created_at >= $1 AND i.created_at <= $2
	`
	bArgs := []interface{}{from, to}
	if branchID != nil {
		branchQuery += " WHERE b.id = $3"
		bArgs = append(bArgs, *branchID)
	}
	branchQuery += " GROUP BY b.id, b.code, b.name ORDER BY branch_rev DESC"

	rows, err := r.pool.Query(ctx, branchQuery, bArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch branch revenue: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var br domain.BranchRevenue
		if err := rows.Scan(&br.BranchID, &br.BranchCode, &br.BranchName, &br.RevenueKurus, &br.InvoiceCount); err != nil {
			return nil, err
		}
		report.Branches = append(report.Branches, br)
	}

	return report, nil
}

func (r *postgresReportRepository) GetHourlyOccupancy(ctx context.Context, branchID *uuid.UUID, date time.Time) (*domain.HourlyOccupancyReport, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := `
		SELECT 
			EXTRACT(HOUR FROM created_at)::int as hr,
			COUNT(CASE WHEN direction = 'IN' THEN 1 END) as in_cnt,
			COUNT(CASE WHEN direction = 'OUT' THEN 1 END) as out_cnt
		FROM access_logs
		WHERE created_at >= $1 AND created_at < $2 AND status = 'GRANTED'
	`
	args := []interface{}{startOfDay, endOfDay}
	if branchID != nil {
		query += " AND branch_id = $3"
		args = append(args, *branchID)
	}
	query += " GROUP BY hr ORDER BY hr ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch hourly occupancy: %w", err)
	}
	defer rows.Close()

	hourMap := make(map[int]domain.HourlyOccupancySlot)
	for rows.Next() {
		var hr, inCnt, outCnt int
		if err := rows.Scan(&hr, &inCnt, &outCnt); err != nil {
			return nil, err
		}
		hourMap[hr] = domain.HourlyOccupancySlot{
			Hour:    hr,
			Entries: inCnt,
			Exits:   outCnt,
		}
	}

	report := &domain.HourlyOccupancyReport{
		BranchID: branchID,
		Date:     startOfDay.Format("2006-01-02"),
		Slots:    make([]domain.HourlyOccupancySlot, 24),
	}

	peakHour := 0
	peakCount := 0
	cumulativeInside := 0

	for h := 0; h < 24; h++ {
		slot := hourMap[h]
		slot.Hour = h
		cumulativeInside += (slot.Entries - slot.Exits)
		if cumulativeInside < 0 {
			cumulativeInside = 0
		}
		slot.EstimatedInside = cumulativeInside
		if slot.Entries > peakCount {
			peakCount = slot.Entries
			peakHour = h
		}
		report.Slots[h] = slot
	}

	report.PeakHour = peakHour
	report.PeakCount = peakCount

	if branchID != nil {
		var bName string
		_ = r.pool.QueryRow(ctx, "SELECT name FROM branches WHERE id = $1", *branchID).Scan(&bName)
		report.BranchName = bName
	}

	return report, nil
}

func (r *postgresReportRepository) GetTrainerReport(ctx context.Context, from, to time.Time) (*domain.TrainerReport, error) {
	query := `
		SELECT 
			u.id, 
			u.first_name || ' ' || u.last_name as trainer_name,
			COUNT(DISTINCT s.id) as total_classes,
			COUNT(DISTINCT b.id) as total_class_attendees,
			COUNT(DISTINCT pt.id) as total_pt_sessions,
			COALESCE(
				CASE WHEN SUM(s.capacity) > 0 
					THEN ROUND((COUNT(b.id)::numeric / SUM(s.capacity)::numeric) * 100, 2)
					ELSE 0 
				END, 
				0
			) as avg_occupancy
		FROM users u
		LEFT JOIN class_schedules s ON u.id = s.trainer_id AND s.start_time >= $1 AND s.end_time <= $2
		LEFT JOIN class_bookings b ON s.id = b.schedule_id AND b.status IN ('CONFIRMED', 'ATTENDED')
		LEFT JOIN pt_appointments pt ON u.id = pt.trainer_id AND pt.start_time >= $1 AND pt.end_time <= $2 AND pt.status IN ('CONFIRMED', 'COMPLETED')
		WHERE u.role = 'TRAINER'
		GROUP BY u.id, u.first_name, u.last_name
		ORDER BY total_classes DESC, total_pt_sessions DESC
	`

	rows, err := r.pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch trainer stats: %w", err)
	}
	defer rows.Close()

	report := &domain.TrainerReport{
		StartDate: from,
		EndDate:   to,
		Trainers:  []domain.TrainerPerformance{},
	}

	for rows.Next() {
		var tp domain.TrainerPerformance
		if err := rows.Scan(
			&tp.TrainerID, &tp.TrainerName, &tp.TotalClasses,
			&tp.TotalClassAttendees, &tp.TotalPTSessions, &tp.AverageOccupancyPct,
		); err != nil {
			return nil, err
		}
		report.Trainers = append(report.Trainers, tp)
	}

	return report, nil
}
