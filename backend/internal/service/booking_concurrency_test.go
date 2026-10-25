package service_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"fitgiris/internal/domain"
)

// TestConcurrency_50MembersCompeteForLast1Spot proves that when 50 members concurrently
// attempt to book the final available spot in a class:
// 1. Exactly ONE (1) member succeeds in getting a CONFIRMED booking.
// 2. The other 49 members are placed into the FIFO waitlist (WAITING).
// 3. The schedule's booked_count is strictly 1 (never exceeding capacity).
// 4. When the winning member cancels, the first waitlisted member is atomically promoted.
func TestConcurrency_50MembersCompeteForLast1Spot(t *testing.T) {
	svc, repo, _, _ := setupBookingService()
	ctx := context.Background()

	branchID := uuid.New()
	trainerID := uuid.New()

	class := &domain.Class{
		ID:              uuid.New(),
		Name:            "High-Intensity Spinning",
		DurationMinutes: 45,
		DefaultCapacity: 1,
		IsActive:        true,
	}
	_ = repo.CreateClass(ctx, class)

	// Create a class with Capacity = 1 and BookedCount = 0 (only 1 spot available!)
	startTime := time.Now().UTC().Add(48 * time.Hour)
	schedule := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class.ID,
		BranchID:    branchID,
		TrainerID:   trainerID,
		StartTime:   startTime,
		EndTime:     startTime.Add(45 * time.Minute),
		Capacity:    1,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedule)

	const totalCompetitors = 50
	memberIDs := make([]uuid.UUID, totalCompetitors)

	// Prepare 50 active member packages
	for i := 0; i < totalCompetitors; i++ {
		mID := uuid.New()
		memberIDs[i] = mID
		repo.memberPackages[mID] = &domain.MemberPackage{
			ID:          uuid.New(),
			MemberID:    mID,
			BranchID:    branchID,
			AllBranches: true,
			Status:      domain.StatusActive,
			PackageKind: domain.PackageKindDuration,
		}
	}

	type bookingOutcome struct {
		memberID     uuid.UUID
		isWaitlisted bool
		position     int
		err          error
	}

	outcomes := make(chan bookingOutcome, totalCompetitors)
	var confirmedCount int64
	var waitlistCount int64

	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	// Spin up 50 concurrent goroutines
	for i := 0; i < totalCompetitors; i++ {
		wg.Add(1)
		go func(mID uuid.UUID) {
			defer wg.Done()

			// Wait for barrier signal so all 50 goroutines fire at the exact same millisecond
			<-startBarrier

			res, err := svc.BookClass(ctx, mID, schedule.ID)
			if err != nil {
				outcomes <- bookingOutcome{memberID: mID, err: err}
				return
			}

			if !res.IsWaitlisted {
				atomic.AddInt64(&confirmedCount, 1)
				outcomes <- bookingOutcome{memberID: mID, isWaitlisted: false}
			} else {
				atomic.AddInt64(&waitlistCount, 1)
				outcomes <- bookingOutcome{memberID: mID, isWaitlisted: true, position: res.WaitlistPosition}
			}
		}(memberIDs[i])
	}

	// Release the barrier! All 50 goroutines execute concurrently
	close(startBarrier)
	wg.Wait()
	close(outcomes)

	// Process outcomes
	var confirmedMemberID uuid.UUID
	waitlistPositions := make(map[int]bool)

	for out := range outcomes {
		if out.err != nil {
			t.Fatalf("goroutine encountered unexpected error: %v", out.err)
		}
		if !out.isWaitlisted {
			confirmedMemberID = out.memberID
		} else {
			if out.position < 1 || out.position > totalCompetitors-1 {
				t.Errorf("invalid waitlist position: %d", out.position)
			}
			waitlistPositions[out.position] = true
		}
	}

	// VERIFICATION 1: Exactly 1 member got CONFIRMED
	if confirmedCount != 1 {
		t.Fatalf("RACE CONDITION DETECTED! Expected exactly 1 confirmed booking, got %d", confirmedCount)
	}

	// VERIFICATION 2: Exactly 49 members entered the WAITLIST
	if waitlistCount != 49 {
		t.Fatalf("Expected exactly 49 waitlisted members, got %d", waitlistCount)
	}

	// VERIFICATION 3: Schedule booked_count is strictly 1 and does not exceed capacity
	schedCheck, err := repo.GetScheduleByID(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("failed to get schedule: %v", err)
	}
	if schedCheck.BookedCount != 1 {
		t.Fatalf("Schedule booked_count is %d, expected strictly 1", schedCheck.BookedCount)
	}
	if schedCheck.BookedCount > schedCheck.Capacity {
		t.Fatalf("CAPACITY OVERFLOW! BookedCount %d > Capacity %d", schedCheck.BookedCount, schedCheck.Capacity)
	}

	t.Logf("PASS: 50 concurrent requests successfully processed. 1 CONFIRMED, 49 WAITLISTED. BookedCount: %d / %d", schedCheck.BookedCount, schedCheck.Capacity)

	// VERIFICATION 4: Test Cancellation & Automatic Waitlist Promotion
	// Winning member cancels
	winningBooking, err := repo.GetBookingByMemberAndSchedule(ctx, schedule.ID, confirmedMemberID)
	if err != nil {
		t.Fatalf("failed to find winning booking: %v", err)
	}

	cancelRes, err := svc.CancelBooking(ctx, confirmedMemberID, domain.RoleMember, winningBooking.ID)
	if err != nil {
		t.Fatalf("cancellation of winning booking failed: %v", err)
	}

	if cancelRes.PromotedMemberID == nil {
		t.Fatalf("Expected first waitlisted member to be promoted, but PromotedMemberID is nil")
	}

	// Verify the promoted member now has a CONFIRMED booking
	promotedBooking, err := repo.GetBookingByMemberAndSchedule(ctx, schedule.ID, *cancelRes.PromotedMemberID)
	if err != nil {
		t.Fatalf("promoted member booking not found: %v", err)
	}
	if promotedBooking.Status != domain.BookingStatusConfirmed {
		t.Fatalf("promoted member booking status is %s, expected CONFIRMED", promotedBooking.Status)
	}

	// Capacity must remain exactly 1 (the cancelled spot was claimed by the promoted member)
	schedAfterCancel, _ := repo.GetScheduleByID(ctx, schedule.ID)
	if schedAfterCancel.BookedCount != 1 {
		t.Fatalf("Schedule booked_count after promotion is %d, expected strictly 1", schedAfterCancel.BookedCount)
	}

	t.Logf("PASS: First waitlist member (%s) automatically promoted to CONFIRMED. Spot preserved at 1/1.", cancelRes.PromotedMemberID.String())
}

// TestConcurrency_MultipleSimultaneousCancellations verifies race conditions when multiple
// bookings are cancelled and waitlisted members are promoted concurrently.
func TestConcurrency_MultipleSimultaneousCancellations(t *testing.T) {
	svc, repo, _, _ := setupBookingService()
	ctx := context.Background()

	branchID := uuid.New()
	trainerID := uuid.New()

	class := &domain.Class{
		ID:              uuid.New(),
		Name:            "Pilates Reformer",
		DurationMinutes: 50,
		DefaultCapacity: 5,
		IsActive:        true,
	}
	_ = repo.CreateClass(ctx, class)

	startTime := time.Now().UTC().Add(72 * time.Hour)
	schedule := &domain.ClassSchedule{
		ID:          uuid.New(),
		ClassID:     class.ID,
		BranchID:    branchID,
		TrainerID:   trainerID,
		StartTime:   startTime,
		EndTime:     startTime.Add(50 * time.Minute),
		Capacity:    5,
		BookedCount: 0,
		Status:      domain.ScheduleStatusScheduled,
	}
	_ = repo.CreateSchedule(ctx, schedule)

	// Create 5 booked members and 5 waitlisted members
	var bookedIDs []uuid.UUID
	var bookingIDs []uuid.UUID

	for i := 0; i < 10; i++ {
		mID := uuid.New()
		repo.memberPackages[mID] = &domain.MemberPackage{
			ID:          uuid.New(),
			MemberID:    mID,
			BranchID:    branchID,
			AllBranches: true,
			Status:      domain.StatusActive,
			PackageKind: domain.PackageKindDuration,
		}

		res, err := svc.BookClass(ctx, mID, schedule.ID)
		if err != nil {
			t.Fatalf("booking member %d failed: %v", i, err)
		}

		if i < 5 {
			if res.IsWaitlisted {
				t.Fatalf("member %d should have been confirmed", i)
			}
			bookedIDs = append(bookedIDs, mID)
			bookingIDs = append(bookingIDs, res.Booking.ID)
		} else {
			if !res.IsWaitlisted {
				t.Fatalf("member %d should have been waitlisted", i)
			}
		}
	}

	// Concurrently cancel 3 of the 5 booked members
	const cancelCount = 3
	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	for i := 0; i < cancelCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startBarrier
			_, err := svc.CancelBooking(ctx, bookedIDs[idx], domain.RoleMember, bookingIDs[idx])
			if err != nil {
				t.Errorf("concurrent cancel %d failed: %v", idx, err)
			}
		}(i)
	}

	close(startBarrier)
	wg.Wait()

	// Verify that schedule booked_count remains 5 because 3 were promoted from the 5 waitlisted!
	schedAfter, err := repo.GetScheduleByID(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("failed to get schedule: %v", err)
	}
	if schedAfter.BookedCount != 5 {
		t.Errorf("expected booked_count to remain 5 after 3 promotions, got %d", schedAfter.BookedCount)
	}

	// Remaining waiting members should be exactly 5 - 3 = 2
	waitCount, err := repo.GetWaitlistCount(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("failed to get waitlist count: %v", err)
	}
	if waitCount != 2 {
		t.Errorf("expected 2 remaining in waitlist, got %d", waitCount)
	}

	fmt.Printf("PASS: Multiple concurrent cancellations promoted waitlist correctly. Remaining waitlist: %d\n", waitCount)
}
