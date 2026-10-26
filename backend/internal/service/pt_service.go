package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/repository"
)

type PTService interface {
	PurchasePackage(ctx context.Context, memberID, trainerID uuid.UUID, totalSessions int, priceKurus int64) (*domain.PTPackage, error)
	BookAppointment(ctx context.Context, memberID, ptPackageID, branchID uuid.UUID, startTime, endTime time.Time, notes string) (*domain.PTAppointment, error)
	CancelAppointment(ctx context.Context, memberID, appointmentID uuid.UUID) error
	CompleteAppointment(ctx context.Context, appointmentID uuid.UUID) error
	ListMemberPackages(ctx context.Context, memberID uuid.UUID) ([]domain.PTPackage, error)
	ListTrainerPackages(ctx context.Context, trainerID uuid.UUID) ([]domain.PTPackage, error)
	ListMemberAppointments(ctx context.Context, memberID uuid.UUID) ([]domain.PTAppointment, error)
	ListTrainerAppointments(ctx context.Context, trainerID uuid.UUID, from, to *time.Time) ([]domain.PTAppointment, error)
}

type ptService struct {
	ptRepo    repository.PTRepository
	userRepo  repository.UserRepository
	auditRepo repository.AuditRepository
}

func NewPTService(ptRepo repository.PTRepository, userRepo repository.UserRepository, auditRepo repository.AuditRepository) PTService {
	return &ptService{
		ptRepo:    ptRepo,
		userRepo:  userRepo,
		auditRepo: auditRepo,
	}
}

func (s *ptService) PurchasePackage(ctx context.Context, memberID, trainerID uuid.UUID, totalSessions int, priceKurus int64) (*domain.PTPackage, error) {
	if totalSessions <= 0 {
		return nil, errors.New("total_sessions must be greater than zero")
	}
	if priceKurus < 0 {
		return nil, errors.New("price_kurus cannot be negative")
	}

	// Verify trainer exists and has TRAINER role
	trainer, err := s.userRepo.GetByID(ctx, trainerID)
	if err != nil {
		return nil, err
	}
	if trainer.Role != domain.RoleTrainer {
		return nil, errors.New("specified user is not a trainer")
	}

	pkg := &domain.PTPackage{
		ID:                uuid.New(),
		MemberID:          memberID,
		TrainerID:         trainerID,
		TotalSessions:     totalSessions,
		RemainingSessions: totalSessions,
		PriceKurus:        priceKurus,
		IsActive:          true,
		CreatedAt:         time.Now(),
	}

	if err := s.ptRepo.CreatePackage(ctx, pkg); err != nil {
		return nil, err
	}

	newVals, _ := json.Marshal(map[string]interface{}{
		"trainer_id":     trainerID.String(),
		"total_sessions": totalSessions,
		"price_kurus":    priceKurus,
	})
	_ = s.auditRepo.Append(ctx, &domain.AuditLog{
		UserID:    &memberID,
		Action:    "PURCHASE_PT_PACKAGE",
		Entity:    "pt_packages",
		EntityID:  pkg.ID.String(),
		NewValues: newVals,
		CreatedAt: time.Now(),
	})

	return pkg, nil
}

func (s *ptService) BookAppointment(ctx context.Context, memberID, ptPackageID, branchID uuid.UUID, startTime, endTime time.Time, notes string) (*domain.PTAppointment, error) {
	if !endTime.After(startTime) {
		return nil, domain.ErrPTInvalidTime
	}

	tx, err := s.ptRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	if tx != nil {
		defer func() { _ = tx.Rollback(ctx) }()
	}

	// Lock package for update
	pkg, err := s.ptRepo.GetPackageForUpdateTx(ctx, tx, ptPackageID)
	if err != nil {
		return nil, err
	}

	if pkg.MemberID != memberID {
		return nil, errors.New("unauthorized: pt package belongs to another member")
	}

	// Check trainer schedule conflict first
	hasConflict, err := s.ptRepo.HasTrainerConflictTx(ctx, tx, pkg.TrainerID, startTime, endTime, nil)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, domain.ErrPTTrainerConflict
	}

	if err := pkg.DeductSession(); err != nil {
		return nil, err
	}

	// Update package
	if err := s.ptRepo.UpdatePackageTx(ctx, tx, pkg); err != nil {
		return nil, err
	}

	app := &domain.PTAppointment{
		ID:          uuid.New(),
		PTPackageID: ptPackageID,
		MemberID:    memberID,
		TrainerID:   pkg.TrainerID,
		BranchID:    branchID,
		StartTime:   startTime,
		EndTime:     endTime,
		Status:      domain.PTStatusConfirmed,
		Notes:       notes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.ptRepo.CreateAppointmentTx(ctx, tx, app); err != nil {
		return nil, err
	}

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}

	newVals, _ := json.Marshal(map[string]interface{}{
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
		"trainer_id": pkg.TrainerID.String(),
	})
	_ = s.auditRepo.Append(ctx, &domain.AuditLog{
		UserID:    &memberID,
		Action:    "BOOK_PT_APPOINTMENT",
		Entity:    "pt_appointments",
		EntityID:  app.ID.String(),
		NewValues: newVals,
		CreatedAt: time.Now(),
	})

	return app, nil
}

func (s *ptService) CancelAppointment(ctx context.Context, memberID, appointmentID uuid.UUID) error {
	tx, err := s.ptRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	if tx != nil {
		defer func() { _ = tx.Rollback(ctx) }()
	}

	app, err := s.ptRepo.GetAppointmentForUpdateTx(ctx, tx, appointmentID)
	if err != nil {
		return err
	}

	if app.MemberID != memberID {
		return errors.New("unauthorized: appointment belongs to another member")
	}

	if app.Status != domain.PTStatusConfirmed {
		return domain.ErrPTAlreadyFinished
	}

	app.Status = domain.PTStatusCancelled
	if err := s.ptRepo.UpdateAppointmentTx(ctx, tx, app); err != nil {
		return err
	}

	// Refund session to package
	pkg, err := s.ptRepo.GetPackageForUpdateTx(ctx, tx, app.PTPackageID)
	if err != nil {
		return err
	}
	pkg.RefundSession()
	if err := s.ptRepo.UpdatePackageTx(ctx, tx, pkg); err != nil {
		return err
	}

	if tx != nil {
		return tx.Commit(ctx)
	}
	return nil
}

func (s *ptService) CompleteAppointment(ctx context.Context, appointmentID uuid.UUID) error {
	tx, err := s.ptRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	if tx != nil {
		defer func() { _ = tx.Rollback(ctx) }()
	}

	app, err := s.ptRepo.GetAppointmentForUpdateTx(ctx, tx, appointmentID)
	if err != nil {
		return err
	}

	if app.Status != domain.PTStatusConfirmed {
		return domain.ErrPTAlreadyFinished
	}

	app.Status = domain.PTStatusCompleted
	if err := s.ptRepo.UpdateAppointmentTx(ctx, tx, app); err != nil {
		return err
	}

	if tx != nil {
		return tx.Commit(ctx)
	}
	return nil
}

func (s *ptService) ListMemberPackages(ctx context.Context, memberID uuid.UUID) ([]domain.PTPackage, error) {
	return s.ptRepo.ListPackagesByMember(ctx, memberID)
}

func (s *ptService) ListTrainerPackages(ctx context.Context, trainerID uuid.UUID) ([]domain.PTPackage, error) {
	return s.ptRepo.ListPackagesByTrainer(ctx, trainerID)
}

func (s *ptService) ListMemberAppointments(ctx context.Context, memberID uuid.UUID) ([]domain.PTAppointment, error) {
	return s.ptRepo.ListAppointmentsByMember(ctx, memberID)
}

func (s *ptService) ListTrainerAppointments(ctx context.Context, trainerID uuid.UUID, from, to *time.Time) ([]domain.PTAppointment, error) {
	return s.ptRepo.ListAppointmentsByTrainer(ctx, trainerID, from, to)
}
