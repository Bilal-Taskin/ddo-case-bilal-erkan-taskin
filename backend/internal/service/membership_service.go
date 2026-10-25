package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/dateutil"
	"fitgiris/internal/repository"
)

type MembershipService struct {
	repo      repository.MembershipRepository
	userRepo  repository.UserRepository
	auditRepo repository.AuditRepository
}

func NewMembershipService(
	repo repository.MembershipRepository,
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
) *MembershipService {
	return &MembershipService{
		repo:      repo,
		userRepo:  userRepo,
		auditRepo: auditRepo,
	}
}

// Package Types Management

type CreatePackageTypeInput struct {
	Name           string             `json:"name"`
	Type           domain.PackageKind `json:"type"`
	DurationMonths int                `json:"duration_months"`
	TotalEntries   int                `json:"total_entries"`
	PriceKurus     int64              `json:"price_kurus"`
	AllBranches    bool               `json:"all_branches"`
	MaxFreezeDays  int                `json:"max_freeze_days"`
	IsActive       *bool              `json:"is_active,omitempty"`
}

func (s *MembershipService) CreatePackageType(ctx context.Context, input CreatePackageTypeInput) (*domain.PackageType, error) {
	if input.Name == "" {
		return nil, errors.New("paket adi bos olamaz")
	}
	if !input.Type.IsValid() {
		return nil, errors.New("gecersiz paket tipi, DURATION veya SESSION olmalidir")
	}
	if input.DurationMonths <= 0 {
		return nil, errors.New("paket gecerlilik suresi en az 1 ay olmalidir")
	}
	if input.PriceKurus < 0 {
		return nil, errors.New("fiyat negatif olamaz")
	}
	if input.MaxFreezeDays < 0 {
		input.MaxFreezeDays = 30
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	pt := &domain.PackageType{
		ID:             uuid.New(),
		Name:           input.Name,
		Type:           input.Type,
		DurationMonths: input.DurationMonths,
		TotalEntries:   input.TotalEntries,
		PriceKurus:     input.PriceKurus,
		AllBranches:    input.AllBranches,
		MaxFreezeDays:  input.MaxFreezeDays,
		IsActive:       isActive,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.repo.CreatePackageType(ctx, pt); err != nil {
		return nil, err
	}
	return pt, nil
}

func (s *MembershipService) ListPackageTypes(ctx context.Context, activeOnly bool) ([]domain.PackageType, error) {
	return s.repo.ListPackageTypes(ctx, activeOnly)
}

func (s *MembershipService) GetPackageType(ctx context.Context, id uuid.UUID) (*domain.PackageType, error) {
	return s.repo.GetPackageTypeByID(ctx, id)
}

// Purchase Membership (Satış / Satın Alma)

type PurchasePackageInput struct {
	MemberID      uuid.UUID  `json:"member_id"`
	PackageTypeID uuid.UUID  `json:"package_type_id"`
	BranchID      uuid.UUID  `json:"branch_id"`
	StartDate     *time.Time `json:"start_date,omitempty"`
	AutoRenew     bool       `json:"auto_renew"`
	CardToken     *string    `json:"card_token,omitempty"`
}

func (s *MembershipService) PurchasePackage(
	ctx context.Context,
	input PurchasePackageInput,
	actorID uuid.UUID,
	actorRole domain.UserRole,
	ipAddress *string,
) (*domain.MemberPackage, error) {
	// Security check: normal MEMBER can only purchase for themselves
	if actorRole == domain.RoleMember && input.MemberID != actorID {
		return nil, domain.ErrForbidden
	}

	// Verify target member exists and is active
	member, err := s.userRepo.GetByID(ctx, input.MemberID)
	if err != nil {
		return nil, err
	}
	if !member.IsActive {
		return nil, domain.ErrInactiveUser
	}

	// Fetch package type
	pkgType, err := s.repo.GetPackageTypeByID(ctx, input.PackageTypeID)
	if err != nil {
		return nil, err
	}
	if !pkgType.IsActive {
		return nil, domain.ErrPackageTypeInactive
	}

	// Calculate Start and Clamped End Date
	startDate := time.Now().UTC()
	if input.StartDate != nil && !input.StartDate.IsZero() {
		startDate = input.StartDate.UTC()
	}
	startDate = dateutil.TruncateToDate(startDate)

	// Clamped Month Arithmetic: Prevents 31 Jan + 1 Month from overflowing into March
	endDate := dateutil.AddClampedMonths(startDate, pkgType.DurationMonths)

	totalEntries := 0
	remainingEntries := 0
	if pkgType.Type == domain.PackageKindSession {
		totalEntries = pkgType.TotalEntries
		remainingEntries = pkgType.TotalEntries
	}

	mp := &domain.MemberPackage{
		ID:                        uuid.New(),
		MemberID:                  input.MemberID,
		PackageTypeID:             pkgType.ID,
		BranchID:                  input.BranchID,
		Status:                    domain.StatusActive,
		StartDate:                 startDate,
		EndDate:                   endDate,
		RemainingEntries:          remainingEntries,
		TotalEntries:              totalEntries,
		PricePaidKurus:            pkgType.PriceKurus,
		AutoRenew:                 input.AutoRenew,
		CardToken:                 input.CardToken,
		ConsecutiveFailedAttempts: 0,
		NextRetryDate:             nil,
	}

	if err := s.repo.CreateMemberPackage(ctx, mp); err != nil {
		return nil, err
	}

	// Audit Log
	auditVal, _ := json.Marshal(map[string]interface{}{
		"member_id":        mp.MemberID,
		"package_type_id":  mp.PackageTypeID,
		"package_name":     pkgType.Name,
		"price_paid_kurus": mp.PricePaidKurus,
		"start_date":       mp.StartDate.Format("2006-01-02"),
		"end_date":         mp.EndDate.Format("2006-01-02"),
		"actor_id":         actorID,
	})
	_ = s.auditRepo.Append(ctx, &domain.AuditLog{
		UserID:    &actorID,
		Action:    "MEMBERSHIP_PURCHASE",
		Entity:    "member_packages",
		EntityID:  mp.ID.String(),
		NewValues: auditVal,
		IPAddress: ipAddress,
	})

	return mp, nil
}

// Freeze Membership (Dondurma)

type FreezeMembershipInput struct {
	MemberPackageID uuid.UUID `json:"member_package_id"`
	Days            int       `json:"days"`
	Reason          string    `json:"reason"`
}

func (s *MembershipService) FreezeMembership(
	ctx context.Context,
	input FreezeMembershipInput,
	actorID uuid.UUID,
	actorRole domain.UserRole,
	ipAddress *string,
) (*domain.MemberPackage, error) {
	if input.Days <= 0 {
		return nil, errors.New("dondurma suresi en az 1 gun olmalidir")
	}

	var updatedPackage *domain.MemberPackage

	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Pessimistic Lock row FOR UPDATE
		pkg, err := s.repo.GetMemberPackageForUpdate(ctx, tx, input.MemberPackageID)
		if err != nil {
			return err
		}

		// Security: Member can only freeze own package
		if actorRole == domain.RoleMember && pkg.MemberID != actorID {
			return domain.ErrForbidden
		}

		// FSM Check: Only ACTIVE can transition to FROZEN
		if err := domain.ValidateTransition(pkg.Status, domain.StatusFrozen); err != nil {
			return err
		}

		// Check Max Freeze Days limit per package
		alreadyFrozen, err := s.repo.GetTotalFrozenDays(ctx, pkg.ID)
		if err != nil {
			return err
		}

		if alreadyFrozen+input.Days > pkg.MaxFreezeDays {
			return fmt.Errorf("%w: maksimum %d gun, su ana kadar %d gun kullanildi, kalan hak %d gun",
				domain.ErrFreezeLimitExceeded, pkg.MaxFreezeDays, alreadyFrozen, pkg.MaxFreezeDays-alreadyFrozen)
		}

		today := dateutil.TruncateToDate(time.Now().UTC())
		freezeEnd := dateutil.AddDays(today, input.Days)

		// Create freeze record
		freeze := &domain.PackageFreeze{
			ID:                 uuid.New(),
			MemberPackageID:    pkg.ID,
			FreezeStartDate:    today,
			FreezeEndDate:      freezeEnd,
			ActualUnfreezeDate: nil,
			DaysFrozen:         input.Days,
			Reason:             &input.Reason,
			CreatedAt:          time.Now().UTC(),
		}
		if err := s.repo.CreateFreezeTx(ctx, tx, freeze); err != nil {
			return err
		}

		// Extend EndDate by the frozen days count
		oldEndDate := pkg.EndDate
		pkg.EndDate = dateutil.AddDays(pkg.EndDate, input.Days)
		pkg.Status = domain.StatusFrozen

		if err := s.repo.UpdateMemberPackageTx(ctx, tx, pkg); err != nil {
			return err
		}

		updatedPackage = pkg

		// Audit Log inside transaction
		auditVal, _ := json.Marshal(map[string]interface{}{
			"previous_status": domain.StatusActive,
			"new_status":      domain.StatusFrozen,
			"days_frozen":     input.Days,
			"old_end_date":    oldEndDate.Format("2006-01-02"),
			"new_end_date":    pkg.EndDate.Format("2006-01-02"),
			"reason":          input.Reason,
			"actor_id":        actorID,
		})
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &actorID,
			Action:    "MEMBERSHIP_FREEZE",
			Entity:    "member_packages",
			EntityID:  pkg.ID.String(),
			NewValues: auditVal,
			IPAddress: ipAddress,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedPackage, nil
}

// Unfreeze Membership (Erken Çözme)

func (s *MembershipService) UnfreezeMembership(
	ctx context.Context,
	memberPackageID uuid.UUID,
	actorID uuid.UUID,
	actorRole domain.UserRole,
	ipAddress *string,
) (*domain.MemberPackage, error) {
	var updatedPackage *domain.MemberPackage

	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Pessimistic Lock row FOR UPDATE
		pkg, err := s.repo.GetMemberPackageForUpdate(ctx, tx, memberPackageID)
		if err != nil {
			return err
		}

		if actorRole == domain.RoleMember && pkg.MemberID != actorID {
			return domain.ErrForbidden
		}

		// FSM Check: Only FROZEN can transition to ACTIVE
		if err := domain.ValidateTransition(pkg.Status, domain.StatusActive); err != nil {
			return err
		}

		// Find active freeze record
		freeze, err := s.repo.GetActiveFreezeTx(ctx, tx, pkg.ID)
		if err != nil {
			return err
		}

		today := dateutil.TruncateToDate(time.Now().UTC())
		var actualDays int
		if today.Before(freeze.FreezeStartDate) {
			actualDays = 0
		} else {
			actualDays = dateutil.DaysBetween(freeze.FreezeStartDate, today)
			if actualDays > freeze.DaysFrozen {
				actualDays = freeze.DaysFrozen
			}
		}

		unusedDays := freeze.DaysFrozen - actualDays
		oldEndDate := pkg.EndDate

		// Reclaim unused days: if member returned early, revoke the unused extension days
		if unusedDays > 0 {
			pkg.EndDate = dateutil.AddDays(pkg.EndDate, -unusedDays)
		}

		freeze.ActualUnfreezeDate = &today
		freeze.DaysFrozen = actualDays
		if err := s.repo.UpdateFreezeTx(ctx, tx, freeze); err != nil {
			return err
		}

		pkg.Status = domain.StatusActive
		if err := s.repo.UpdateMemberPackageTx(ctx, tx, pkg); err != nil {
			return err
		}

		updatedPackage = pkg

		// Audit Log
		auditVal, _ := json.Marshal(map[string]interface{}{
			"previous_status":    domain.StatusFrozen,
			"new_status":         domain.StatusActive,
			"actual_days_frozen": actualDays,
			"unused_days_reclaimed": unusedDays,
			"old_end_date":       oldEndDate.Format("2006-01-02"),
			"new_end_date":       pkg.EndDate.Format("2006-01-02"),
			"actor_id":           actorID,
		})
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &actorID,
			Action:    "MEMBERSHIP_UNFREEZE",
			Entity:    "member_packages",
			EntityID:  pkg.ID.String(),
			NewValues: auditVal,
			IPAddress: ipAddress,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedPackage, nil
}

// Transfer Membership (Devir)

type TransferMembershipInput struct {
	MemberPackageID uuid.UUID `json:"member_package_id"`
	ToMemberID      uuid.UUID `json:"to_member_id"`
	Reason          string    `json:"reason"`
}

func (s *MembershipService) TransferMembership(
	ctx context.Context,
	input TransferMembershipInput,
	actorID uuid.UUID,
	actorRole domain.UserRole,
	ipAddress *string,
) (*domain.MemberPackage, error) {
	var newPackage *domain.MemberPackage

	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Pessimistic Lock source package FOR UPDATE
		srcPkg, err := s.repo.GetMemberPackageForUpdate(ctx, tx, input.MemberPackageID)
		if err != nil {
			return err
		}

		if actorRole == domain.RoleMember && srcPkg.MemberID != actorID {
			return domain.ErrForbidden
		}

		// FSM Check: Only ACTIVE can be TRANSFERRED
		if err := domain.ValidateTransition(srcPkg.Status, domain.StatusTransferred); err != nil {
			return err
		}

		if srcPkg.MemberID == input.ToMemberID {
			return domain.ErrCannotTransferToSelf
		}

		// Verify target recipient member
		toMember, err := s.userRepo.GetByID(ctx, input.ToMemberID)
		if err != nil {
			return err
		}
		if !toMember.IsActive {
			return domain.ErrInactiveUser
		}

		today := dateutil.TruncateToDate(time.Now().UTC())
		remainingDays := dateutil.DaysBetween(today, srcPkg.EndDate)
		if remainingDays <= 0 {
			return domain.ErrMembershipExpired
		}

		remainingEntries := srcPkg.RemainingEntries
		if srcPkg.PackageKind == domain.PackageKindSession && remainingEntries <= 0 {
			return domain.ErrInsufficientEntries
		}

		// 1. Mark source package as TRANSFERRED (Terminal state)
		srcPkg.Status = domain.StatusTransferred
		if err := s.repo.UpdateMemberPackageTx(ctx, tx, srcPkg); err != nil {
			return err
		}

		// 2. Create new active package for recipient with remaining days/entries
		newPkgEndDate := dateutil.AddDays(today, remainingDays)
		newPkg := &domain.MemberPackage{
			ID:                        uuid.New(),
			MemberID:                  input.ToMemberID,
			PackageTypeID:             srcPkg.PackageTypeID,
			BranchID:                  srcPkg.BranchID,
			Status:                    domain.StatusActive,
			StartDate:                 today,
			EndDate:                   newPkgEndDate,
			RemainingEntries:          remainingEntries,
			TotalEntries:              remainingEntries,
			PricePaidKurus:            0, // Transferred without new payment
			AutoRenew:                 false,
			CardToken:                 nil,
			ConsecutiveFailedAttempts: 0,
			NextRetryDate:             nil,
		}
		if err := s.repo.CreateMemberPackageTx(ctx, tx, newPkg); err != nil {
			return err
		}

		// 3. Record transfer history
		transferRecord := &domain.PackageTransfer{
			ID:              uuid.New(),
			FromMemberID:    srcPkg.MemberID,
			ToMemberID:      input.ToMemberID,
			SourcePackageID: srcPkg.ID,
			NewPackageID:    newPkg.ID,
			TransferredDays: remainingDays,
			CreatedAt:       time.Now().UTC(),
		}
		if err := s.repo.CreateTransferTx(ctx, tx, transferRecord); err != nil {
			return err
		}

		newPackage = newPkg

		// Audit Log
		auditVal, _ := json.Marshal(map[string]interface{}{
			"source_package_id": srcPkg.ID,
			"new_package_id":    newPkg.ID,
			"from_member_id":    srcPkg.MemberID,
			"to_member_id":      input.ToMemberID,
			"transferred_days":  remainingDays,
			"reason":            input.Reason,
			"actor_id":          actorID,
		})
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &actorID,
			Action:    "MEMBERSHIP_TRANSFER",
			Entity:    "package_transfers",
			EntityID:  transferRecord.ID.String(),
			NewValues: auditVal,
			IPAddress: ipAddress,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return newPackage, nil
}

// Prorated Upgrade Quote & Execution (Orantılı Yükseltme)

func (s *MembershipService) GetUpgradeQuote(
	ctx context.Context,
	memberPackageID uuid.UUID,
	newPackageTypeID uuid.UUID,
) (*domain.UpgradeQuote, error) {
	pkg, err := s.repo.GetMemberPackageByID(ctx, memberPackageID)
	if err != nil {
		return nil, err
	}

	if pkg.Status != domain.StatusActive {
		return nil, domain.ErrMembershipNotActive
	}

	if pkg.PackageTypeID == newPackageTypeID {
		return nil, domain.ErrSamePackageTypeUpgrade
	}

	newPkgType, err := s.repo.GetPackageTypeByID(ctx, newPackageTypeID)
	if err != nil {
		return nil, err
	}
	if !newPkgType.IsActive {
		return nil, domain.ErrPackageTypeInactive
	}

	today := dateutil.TruncateToDate(time.Now().UTC())
	return CalculateUpgradeQuote(pkg, newPkgType, today)
}

func CalculateUpgradeQuote(
	pkg *domain.MemberPackage,
	newPkgType *domain.PackageType,
	asOfDate time.Time,
) (*domain.UpgradeQuote, error) {
	totalDays := dateutil.DaysBetween(pkg.StartDate, pkg.EndDate)
	if totalDays <= 0 {
		totalDays = 1
	}

	remainingDays := dateutil.DaysBetween(asOfDate, pkg.EndDate)
	if remainingDays < 0 {
		remainingDays = 0
	}
	if remainingDays > totalDays {
		remainingDays = totalDays
	}

	// Prorated Remaining Value Formula: floor((RemainingDays / TotalDays) * PricePaidKurus)
	var remainingValueKurus int64
	if pkg.PackageKind == domain.PackageKindSession && pkg.TotalEntries > 0 {
		remainingValueKurus = (int64(pkg.RemainingEntries) * pkg.PricePaidKurus) / int64(pkg.TotalEntries)
	} else {
		remainingValueKurus = (int64(remainingDays) * pkg.PricePaidKurus) / int64(totalDays)
	}

	if remainingValueKurus < 0 {
		remainingValueKurus = 0
	}
	if remainingValueKurus > pkg.PricePaidKurus {
		remainingValueKurus = pkg.PricePaidKurus
	}

	// Difference to pay: max(0, NewPackagePrice - RemainingValue)
	amountToPayKurus := newPkgType.PriceKurus - remainingValueKurus
	if amountToPayKurus < 0 {
		amountToPayKurus = 0
	}

	return &domain.UpgradeQuote{
		MemberPackageID:      pkg.ID,
		CurrentPackageName:   pkg.PackageName,
		NewPackageTypeID:     newPkgType.ID,
		NewPackageName:       newPkgType.Name,
		TotalDays:            totalDays,
		RemainingDays:        remainingDays,
		PricePaidKurus:       pkg.PricePaidKurus,
		RemainingValueKurus:  remainingValueKurus,
		NewPackagePriceKurus: newPkgType.PriceKurus,
		AmountToPayKurus:     amountToPayKurus,
	}, nil
}

func (s *MembershipService) UpgradeMembership(
	ctx context.Context,
	memberPackageID uuid.UUID,
	newPackageTypeID uuid.UUID,
	actorID uuid.UUID,
	actorRole domain.UserRole,
	ipAddress *string,
) (*domain.MemberPackage, *domain.UpgradeQuote, error) {
	var newPackage *domain.MemberPackage
	var quote *domain.UpgradeQuote

	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Lock current package FOR UPDATE
		pkg, err := s.repo.GetMemberPackageForUpdate(ctx, tx, memberPackageID)
		if err != nil {
			return err
		}

		if actorRole == domain.RoleMember && pkg.MemberID != actorID {
			return domain.ErrForbidden
		}

		if pkg.Status != domain.StatusActive {
			return domain.ErrMembershipNotActive
		}

		if pkg.PackageTypeID == newPackageTypeID {
			return domain.ErrSamePackageTypeUpgrade
		}

		newPkgType, err := s.repo.GetPackageTypeByID(ctx, newPackageTypeID)
		if err != nil {
			return err
		}
		if !newPkgType.IsActive {
			return domain.ErrPackageTypeInactive
		}

		today := dateutil.TruncateToDate(time.Now().UTC())
		q, err := CalculateUpgradeQuote(pkg, newPkgType, today)
		if err != nil {
			return err
		}
		quote = q

		// FSM transition: Mark old package as CANCELLED (replaced by upgrade)
		if err := domain.ValidateTransition(pkg.Status, domain.StatusCancelled); err != nil {
			return err
		}
		pkg.Status = domain.StatusCancelled
		if err := s.repo.UpdateMemberPackageTx(ctx, tx, pkg); err != nil {
			return err
		}

		// Calculate EndDate for new package using Clamped Date Arithmetic
		newEndDate := dateutil.AddClampedMonths(today, newPkgType.DurationMonths)

		totalEntries := 0
		remainingEntries := 0
		if newPkgType.Type == domain.PackageKindSession {
			totalEntries = newPkgType.TotalEntries
			remainingEntries = newPkgType.TotalEntries
		}

		newPkg := &domain.MemberPackage{
			ID:                        uuid.New(),
			MemberID:                  pkg.MemberID,
			PackageTypeID:             newPkgType.ID,
			BranchID:                  pkg.BranchID,
			Status:                    domain.StatusActive,
			StartDate:                 today,
			EndDate:                   newEndDate,
			RemainingEntries:          remainingEntries,
			TotalEntries:              totalEntries,
			PricePaidKurus:            newPkgType.PriceKurus,
			AutoRenew:                 pkg.AutoRenew,
			CardToken:                 pkg.CardToken,
			ConsecutiveFailedAttempts: 0,
			NextRetryDate:             nil,
		}

		if err := s.repo.CreateMemberPackageTx(ctx, tx, newPkg); err != nil {
			return err
		}
		newPackage = newPkg

		// Audit Log
		auditVal, _ := json.Marshal(map[string]interface{}{
			"action":                 "UPGRADE",
			"old_package_id":         pkg.ID,
			"new_package_id":         newPkg.ID,
			"new_package_name":       newPkgType.Name,
			"remaining_value_kurus":  quote.RemainingValueKurus,
			"amount_to_pay_kurus":    quote.AmountToPayKurus,
			"actor_id":               actorID,
		})
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &actorID,
			Action:    "MEMBERSHIP_UPGRADE",
			Entity:    "member_packages",
			EntityID:  newPkg.ID.String(),
			NewValues: auditVal,
			IPAddress: ipAddress,
		})

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return newPackage, quote, nil
}

// Cancel Membership (İptal)

func (s *MembershipService) CancelMembership(
	ctx context.Context,
	memberPackageID uuid.UUID,
	reason string,
	actorID uuid.UUID,
	actorRole domain.UserRole,
	ipAddress *string,
) error {
	return s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		pkg, err := s.repo.GetMemberPackageForUpdate(ctx, tx, memberPackageID)
		if err != nil {
			return err
		}

		if actorRole == domain.RoleMember && pkg.MemberID != actorID {
			return domain.ErrForbidden
		}

		// FSM Check
		if err := domain.ValidateTransition(pkg.Status, domain.StatusCancelled); err != nil {
			return err
		}

		oldStatus := pkg.Status
		pkg.Status = domain.StatusCancelled

		if err := s.repo.UpdateMemberPackageTx(ctx, tx, pkg); err != nil {
			return err
		}

		// Audit Log
		auditVal, _ := json.Marshal(map[string]interface{}{
			"previous_status": oldStatus,
			"new_status":      domain.StatusCancelled,
			"reason":          reason,
			"actor_id":        actorID,
		})
		_ = s.auditRepo.Append(ctx, &domain.AuditLog{
			UserID:    &actorID,
			Action:    "MEMBERSHIP_CANCEL",
			Entity:    "member_packages",
			EntityID:  pkg.ID.String(),
			NewValues: auditVal,
			IPAddress: ipAddress,
		})

		return nil
	})
}

// Read Operations

func (s *MembershipService) GetMembership(ctx context.Context, id uuid.UUID) (*domain.MemberPackage, error) {
	return s.repo.GetMemberPackageByID(ctx, id)
}

func (s *MembershipService) ListMemberships(
	ctx context.Context,
	memberID *uuid.UUID,
	status *domain.MembershipStatus,
	limit, offset int,
) ([]domain.MemberPackage, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListMemberPackages(ctx, memberID, status, limit, offset)
}
