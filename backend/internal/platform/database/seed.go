package database

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/security"
)

type SeedOptions struct {
	SeedValue   int64
	MemberCount int
	Reset       bool
}

var (
	DemoManagerID    = uuid.MustParse("00000000-0000-0000-0001-000000000001")
	DemoAccountantID = uuid.MustParse("00000000-0000-0000-0001-000000000002")
	DemoReceptionID  = uuid.MustParse("00000000-0000-0000-0001-000000000003")
	DemoTrainerID    = uuid.MustParse("00000000-0000-0000-0001-000000000004")
	DemoMemberID     = uuid.MustParse("00000000-0000-0000-0001-000000000005")

	BranchBesiktasID = uuid.MustParse("00000000-0000-0000-0002-000000000001")
	BranchKadikoyID  = uuid.MustParse("00000000-0000-0000-0002-000000000002")
	BranchMaslakID   = uuid.MustParse("00000000-0000-0000-0002-000000000003")
	BranchCankayaID  = uuid.MustParse("00000000-0000-0000-0002-000000000004")
	BranchAlsancakID = uuid.MustParse("00000000-0000-0000-0002-000000000005")

	PkgMonthlyID = uuid.MustParse("00000000-0000-0000-0003-000000000001")
	PkgYearlyID  = uuid.MustParse("00000000-0000-0000-0003-000000000002")
	PkgSessionID = uuid.MustParse("00000000-0000-0000-0003-000000000003")
	PkgGold6M    = uuid.MustParse("00000000-0000-0000-0003-000000000004")
)

func SeedDatabase(ctx context.Context, pool *pgxpool.Pool, opts SeedOptions) error {
	start := time.Now()
	slog.Info("Seed islemi baslatildi", "seed", opts.SeedValue, "member_count", opts.MemberCount, "reset", opts.Reset)

	if opts.Reset {
		slog.Info("Veritabani tablolari sifirlaniyor (TRUNCATE CASCADE)...")
		truncateSQL := `
			TRUNCATE TABLE 
				idempotency_keys, audit_logs, pt_appointments, pt_packages, 
				access_logs, class_waitlist, class_bookings, class_schedules, classes, 
				payments, invoices, invoice_sequences, auto_renewal_queue, 
				package_transfers, package_freezes, member_packages, package_types, 
				refresh_tokens, turnstiles, users, branches 
			RESTART IDENTITY CASCADE;
		`
		if _, err := pool.Exec(ctx, truncateSQL); err != nil {
			return fmt.Errorf("truncate hatasi: %w", err)
		}
		slog.Info("Tablolar temizlendi")
	}

	// 1. Seed Branches
	if err := seedBranches(ctx, pool); err != nil {
		return fmt.Errorf("branches seed hatasi: %w", err)
	}

	// 2. Seed Package Types
	if err := seedPackageTypes(ctx, pool); err != nil {
		return fmt.Errorf("package_types seed hatasi: %w", err)
	}

	// 3. Seed Turnstiles
	if err := seedTurnstiles(ctx, pool); err != nil {
		return fmt.Errorf("turnstiles seed hatasi: %w", err)
	}

	// 4. Seed Demo Users (Pre-compute hash for Password123!)
	demoHash, err := security.HashPassword("Password123!")
	if err != nil {
		return fmt.Errorf("parola hashleme hatasi: %w", err)
	}

	if err := seedDemoUsers(ctx, pool, demoHash); err != nil {
		return fmt.Errorf("demo users seed hatasi: %w", err)
	}

	// 5. Seed Demo Member Package & Turnstile Pass & PT
	if err := seedDemoData(ctx, pool); err != nil {
		return fmt.Errorf("demo data seed hatasi: %w", err)
	}

	// 6. Seed Classes & Schedules
	if err := seedClassesAndSchedules(ctx, pool); err != nil {
		return fmt.Errorf("classes & schedules seed hatasi: %w", err)
	}

	// 7. Seed 20,000 Members (Deterministic PRNG)
	if opts.MemberCount > 0 {
		if err := seedMembersBatch(ctx, pool, opts.SeedValue, opts.MemberCount, demoHash); err != nil {
			return fmt.Errorf("members batch seed hatasi: %w", err)
		}
	}

	slog.Info("Seed islemi basariyla tamamlandi!", "sure", time.Since(start).String())
	return nil
}

func seedBranches(ctx context.Context, pool *pgxpool.Pool) error {
	branches := []struct {
		ID      uuid.UUID
		Code    string
		Name    string
		Address string
		Phone   string
	}{
		{BranchBesiktasID, "IST01", "FitGiriş Beşiktaş Ana Şube", "Sinanpaşa Mah. Şair Nedim Cad. No:12 Beşiktaş/İstanbul", "+90 212 555 0101"},
		{BranchKadikoyID, "IST02", "FitGiriş Kadıköy Şube", "Caferağa Mah. Moda Cad. No:45 Kadıköy/İstanbul", "+90 216 555 0102"},
		{BranchMaslakID, "IST03", "FitGiriş Maslak Şube", "Maslak Mah. Büyükdere Cad. No:120 Sarıyer/İstanbul", "+90 212 555 0103"},
		{BranchCankayaID, "ANK01", "FitGiriş Çankaya Şube", "Tunalı Hilmi Cad. No:88 Çankaya/Ankara", "+90 312 555 0104"},
		{BranchAlsancakID, "IZM01", "FitGiriş Alsancak Şube", "Kültür Mah. Atatürk Cad. No:64 Konak/İzmir", "+90 232 555 0105"},
	}

	for _, b := range branches {
		query := `
			INSERT INTO branches (id, code, name, address, phone, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, address = EXCLUDED.address, phone = EXCLUDED.phone
		`
		if _, err := pool.Exec(ctx, query, b.ID, b.Code, b.Name, b.Address, b.Phone); err != nil {
			return err
		}

		// Initialize invoice_sequences for current year
		currentYear := time.Now().Year()
		seqQuery := `
			INSERT INTO invoice_sequences (year, branch_id, current_val)
			VALUES ($1, $2, 0)
			ON CONFLICT (year, branch_id) DO NOTHING
		`
		if _, err := pool.Exec(ctx, seqQuery, currentYear, b.ID); err != nil {
			return err
		}
	}
	slog.Info("5 Şube ve Fatura Sayaçları hazırlandı")
	return nil
}

func seedPackageTypes(ctx context.Context, pool *pgxpool.Pool) error {
	packages := []struct {
		ID             uuid.UUID
		Name           string
		Type           string
		DurationMonths int
		TotalEntries   int
		PriceKurus     int64
		AllBranches    bool
		MaxFreezeDays  int
	}{
		{PkgMonthlyID, "Aylık Standart Üyelik", "DURATION", 1, 0, 150000, false, 15},
		{PkgYearlyID, "Yıllık VIP Üyelik", "DURATION", 12, 0, 1400000, true, 30},
		{PkgSessionID, "10 Girişlik Seans Paketi", "SESSION", 12, 10, 180000, false, 0},
		{PkgGold6M, "6 Aylık Gold Üyelik", "DURATION", 6, 0, 750000, true, 30},
	}

	for _, p := range packages {
		query := `
			INSERT INTO package_types (id, name, type, duration_months, total_entries, price_kurus, all_branches, max_freeze_days, is_active, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, NOW())
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, price_kurus = EXCLUDED.price_kurus
		`
		if _, err := pool.Exec(ctx, query, p.ID, p.Name, p.Type, p.DurationMonths, p.TotalEntries, p.PriceKurus, p.AllBranches, p.MaxFreezeDays); err != nil {
			return err
		}
	}
	slog.Info("Standart Paket Tipleri hazırlandı")
	return nil
}

func seedTurnstiles(ctx context.Context, pool *pgxpool.Pool) error {
	branchIDs := []struct {
		ID   uuid.UUID
		Code string
	}{
		{BranchBesiktasID, "IST01"},
		{BranchKadikoyID, "IST02"},
		{BranchMaslakID, "IST03"},
		{BranchCankayaID, "ANK01"},
		{BranchAlsancakID, "IZM01"},
	}

	for _, b := range branchIDs {
		// IN Turnstile
		inID := uuid.NewSHA1(b.ID, []byte("IN"))
		queryIn := `
			INSERT INTO turnstiles (id, branch_id, code, name, direction, is_active, created_at)
			VALUES ($1, $2, $3, $4, 'IN', true, NOW())
			ON CONFLICT (id) DO NOTHING
		`
		if _, err := pool.Exec(ctx, queryIn, inID, b.ID, b.Code+"-T-IN", b.Code+" Giriş Kapısı"); err != nil {
			return err
		}

		// OUT Turnstile
		outID := uuid.NewSHA1(b.ID, []byte("OUT"))
		queryOut := `
			INSERT INTO turnstiles (id, branch_id, code, name, direction, is_active, created_at)
			VALUES ($1, $2, $3, $4, 'OUT', true, NOW())
			ON CONFLICT (id) DO NOTHING
		`
		if _, err := pool.Exec(ctx, queryOut, outID, b.ID, b.Code+"-T-OUT", b.Code+" Çıkış Kapısı"); err != nil {
			return err
		}
	}
	slog.Info("Şube Giriş/Çıkış Turnikeleri tanımlandı")
	return nil
}

func seedDemoUsers(ctx context.Context, pool *pgxpool.Pool, hash string) error {
	users := []struct {
		ID        uuid.UUID
		Email     string
		FirstName string
		LastName  string
		Role      string
		BranchID  uuid.UUID
	}{
		{DemoManagerID, "manager@fitgiris.com", "Ahmet", "Yönetici", "MANAGER", BranchBesiktasID},
		{DemoAccountantID, "accountant@fitgiris.com", "Ayşe", "Muhasebe", "ACCOUNTANT", BranchBesiktasID},
		{DemoReceptionID, "reception@fitgiris.com", "Canan", "Resepsiyon", "RECEPTION", BranchBesiktasID},
		{DemoTrainerID, "trainer@fitgiris.com", "Murat", "Eğitmen", "TRAINER", BranchBesiktasID},
		{DemoMemberID, "member@fitgiris.com", "Bilal", "Üye", "MEMBER", BranchBesiktasID},
	}

	for _, u := range users {
		query := `
			INSERT INTO users (id, email, password_hash, first_name, last_name, phone, role, home_branch_id, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, '+90 555 111 2233', $6, $7, true, NOW(), NOW())
			ON CONFLICT (id) DO UPDATE SET password_hash = EXCLUDED.password_hash, role = EXCLUDED.role
		`
		if _, err := pool.Exec(ctx, query, u.ID, u.Email, hash, u.FirstName, u.LastName, u.Role, u.BranchID); err != nil {
			return err
		}
	}
	slog.Info("5 Rol için Demo Hesapları oluşturuldu (Parola: Password123!)")
	return nil
}

func seedDemoData(ctx context.Context, pool *pgxpool.Pool) error {
	// Member package for demo member
	pkgID := uuid.MustParse("00000000-0000-0000-0004-000000000001")
	now := time.Now()
	startDate := now.AddDate(0, 0, -5)
	endDate := now.AddDate(1, 0, 0)

	pkgQuery := `
		INSERT INTO member_packages (
			id, member_id, package_type_id, branch_id, status, start_date, end_date, 
			remaining_entries, total_entries, price_paid_kurus, auto_renew, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, 'ACTIVE', $5, $6, 0, 0, 1400000, true, NOW(), NOW()
		) ON CONFLICT (id) DO NOTHING
	`
	if _, err := pool.Exec(ctx, pkgQuery, pkgID, DemoMemberID, PkgYearlyID, BranchBesiktasID, startDate, endDate); err != nil {
		return err
	}

	// PT Package for demo member with Murat Eğitmen
	ptPkgID := uuid.MustParse("00000000-0000-0000-0005-000000000001")
	ptPkgQuery := `
		INSERT INTO pt_packages (
			id, member_id, trainer_id, total_sessions, remaining_sessions, price_kurus, is_active, created_at
		) VALUES (
			$1, $2, $3, 10, 8, 800000, true, NOW()
		) ON CONFLICT (id) DO NOTHING
	`
	if _, err := pool.Exec(ctx, ptPkgQuery, ptPkgID, DemoMemberID, DemoTrainerID); err != nil {
		return err
	}

	// Sample PT Appointment
	ptAppID := uuid.MustParse("00000000-0000-0000-0005-000000000002")
	appStart := now.Add(48 * time.Hour).Truncate(time.Hour)
	appEnd := appStart.Add(time.Hour)
	ptAppQuery := `
		INSERT INTO pt_appointments (
			id, pt_package_id, member_id, trainer_id, branch_id, start_time, end_time, status, notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, 'CONFIRMED', 'Kuvvet ve Kondisyon', NOW(), NOW()
		) ON CONFLICT (id) DO NOTHING
	`
	if _, err := pool.Exec(ctx, ptAppQuery, ptAppID, ptPkgID, DemoMemberID, DemoTrainerID, BranchBesiktasID, appStart, appEnd); err != nil {
		return err
	}

	// Sample Paid Invoice for Demo Member
	invID := uuid.MustParse("00000000-0000-0000-0006-000000000001")
	invQuery := `
		INSERT INTO invoices (
			id, invoice_number, member_id, branch_id, member_package_id, year, sequence_number, 
			amount_kurus, tax_kurus, status, payment_method, created_at
		) VALUES (
			$1, 'FIT-2026-IST01-000001', $2, $3, $4, 2026, 1, 1400000, 280000, 'PAID', 'CREDIT_CARD', NOW()
		) ON CONFLICT (id) DO NOTHING
	`
	if _, err := pool.Exec(ctx, invQuery, invID, DemoMemberID, BranchBesiktasID, pkgID); err != nil {
		return err
	}

	// Update sequence counter to 1
	_ = pool.QueryRow(ctx, "UPDATE invoice_sequences SET current_val = 1 WHERE year = 2026 AND branch_id = $1", BranchBesiktasID)

	slog.Info("Demo Üye Aktif Paketi, PT Randevusu ve Faturası hazırlandı")
	return nil
}

func seedClassesAndSchedules(ctx context.Context, pool *pgxpool.Pool) error {
	classes := []struct {
		ID       uuid.UUID
		Name     string
		Duration int
		Capacity int
	}{
		{uuid.MustParse("00000000-0000-0000-0007-000000000001"), "Spinning Yüksek Tempo", 50, 20},
		{uuid.MustParse("00000000-0000-0000-0007-000000000002"), "Pilates Reformer Core", 50, 12},
		{uuid.MustParse("00000000-0000-0000-0007-000000000003"), "CrossFit WOD", 60, 15},
		{uuid.MustParse("00000000-0000-0000-0007-000000000004"), "Vinyasa Akış Yoga", 60, 25},
		{uuid.MustParse("00000000-0000-0000-0007-000000000005"), "HIIT Yağ Yakımı", 45, 20},
	}

	for _, c := range classes {
		query := `
			INSERT INTO classes (id, name, description, duration_minutes, default_capacity, is_active, created_at)
			VALUES ($1, $2, $3, $4, $5, true, NOW())
			ON CONFLICT (id) DO NOTHING
		`
		if _, err := pool.Exec(ctx, query, c.ID, c.Name, c.Name, c.Duration, c.Capacity); err != nil {
			return err
		}
	}

	// Create sample schedules for today and tomorrow
	now := time.Now()
	schedule1ID := uuid.MustParse("00000000-0000-0000-0008-000000000001")
	schStart1 := time.Date(now.Year(), now.Month(), now.Day(), 18, 30, 0, 0, now.Location())
	schEnd1 := schStart1.Add(50 * time.Minute)

	schQuery := `
		INSERT INTO class_schedules (
			id, class_id, branch_id, trainer_id, start_time, end_time, capacity, booked_count, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, 20, 1, 'SCHEDULED', NOW(), NOW()
		) ON CONFLICT (id) DO NOTHING
	`
	_, _ = pool.Exec(ctx, schQuery, schedule1ID, classes[0].ID, BranchBesiktasID, DemoTrainerID, schStart1, schEnd1)

	// Booking for demo member
	bookQuery := `
		INSERT INTO class_bookings (
			id, schedule_id, member_id, status, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, 'CONFIRMED', NOW(), NOW()
		) ON CONFLICT (schedule_id, member_id) DO NOTHING
	`
	_, _ = pool.Exec(ctx, bookQuery, schedule1ID, DemoMemberID)

	slog.Info("Grup Dersleri ve Örnek Seans Programı hazırlandı")
	return nil
}

func seedMembersBatch(ctx context.Context, pool *pgxpool.Pool, seed int64, totalMembers int, defaultHash string) error {
	slog.Info("20.000 Üye ve Üyelik Paketleri toplu ekleniyor (PRNG Seed)...", "seed", seed, "count", totalMembers)

	rng := rand.New(rand.NewSource(seed))

	firstNames := []string{
		"Mehmet", "Mustafa", "Ahmet", "Ali", "Hüseyin", "Hasan", "İbrahim", "İsmail", "Osman", "Halil",
		"Fatma", "Ayşe", "Emine", "Hatice", "Zeynep", "Elif", "Merve", "Büşra", "Esra", "Seda",
		"Can", "Burak", "Emre", "Cem", "Oğuz", "Deniz", "Ege", "Alper", "Tolga", "Kaan",
		"Selin", "Ece", "Melis", "Gamze", "Gizem", "İrem", "Duygu", "Pınar", "Tuğba", "Hande",
	}

	lastNames := []string{
		"Yılmaz", "Kaya", "Demir", "Çelik", "Şahin", "Yıldız", "Yıldırım", "Öztürk", "Aydın", "Özdemir",
		"Arslan", "Doğan", "Kılıç", "Aslan", "Çetin", "Kara", "Koç", "Kurt", "Özkan", "Şimşek",
		"Polat", "Korkmaz", "Güneş", "Aksoy", "Yalçın", "Erdoğan", "Bulut", "Keskin", "Ünal", "Turan",
	}

	branches := []uuid.UUID{BranchBesiktasID, BranchKadikoyID, BranchMaslakID, BranchCankayaID, BranchAlsancakID}
	packageIDs := []uuid.UUID{PkgMonthlyID, PkgYearlyID, PkgSessionID, PkgGold6M}

	batchSize := 1000
	totalBatches := (totalMembers + batchSize - 1) / batchSize

	now := time.Now()

	for b := 0; b < totalBatches; b++ {
		startIdx := b*batchSize + 1
		endIdx := (b + 1) * batchSize
		if endIdx > totalMembers {
			endIdx = totalMembers
		}

		userRows := make([][]interface{}, 0, endIdx-startIdx+1)
		pkgRows := make([][]interface{}, 0, endIdx-startIdx+1)

		for i := startIdx; i <= endIdx; i++ {
			userID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(fmt.Sprintf("fitgiris_member_%05d", i)))
			email := fmt.Sprintf("member_%05d@fitgiris.com", i)
			fName := firstNames[rng.Intn(len(firstNames))]
			lName := lastNames[rng.Intn(len(lastNames))]
			phone := fmt.Sprintf("+90 5%02d %03d %04d", rng.Intn(50)+30, rng.Intn(900)+100, rng.Intn(9000)+1000)
			branchID := branches[rng.Intn(len(branches))]

			userRows = append(userRows, []interface{}{
				userID, email, defaultHash, fName, lName, phone, string(domain.RoleMember), branchID, true, now, now,
			})

			// Generate Membership Package
			pkgUUID := uuid.NewMD5(userID, []byte("pkg"))
			selectedPkgID := packageIDs[rng.Intn(len(packageIDs))]

			// 70% ACTIVE, 15% EXPIRED, 10% FROZEN, 5% SUSPENDED
			prob := rng.Intn(100)
			status := "ACTIVE"
			startDate := now.AddDate(0, -rng.Intn(6), 0)
			endDate := now.AddDate(0, rng.Intn(6)+1, 0)

			if prob >= 70 && prob < 85 {
				status = "EXPIRED"
				startDate = now.AddDate(-1, -rng.Intn(6), 0)
				endDate = now.AddDate(0, -rng.Intn(2)-1, 0)
			} else if prob >= 85 && prob < 95 {
				status = "FROZEN"
			} else if prob >= 95 {
				status = "SUSPENDED"
			}

			pricePaid := int64(150000)
			if selectedPkgID == PkgYearlyID {
				pricePaid = 1400000
			} else if selectedPkgID == PkgGold6M {
				pricePaid = 750000
			}

			pkgRows = append(pkgRows, []interface{}{
				pkgUUID, userID, selectedPkgID, branchID, status, startDate, endDate, 0, 0, pricePaid, true, now, now,
			})
		}

		// Insert Users via pgx.CopyFrom
		_, err := pool.CopyFrom(
			ctx,
			pgx.Identifier{"users"},
			[]string{"id", "email", "password_hash", "first_name", "last_name", "phone", "role", "home_branch_id", "is_active", "created_at", "updated_at"},
			pgx.CopyFromRows(userRows),
		)
		if err != nil {
			return fmt.Errorf("batch %d users insert hatasi: %w", b+1, err)
		}

		// Insert Member Packages via pgx.CopyFrom
		_, err = pool.CopyFrom(
			ctx,
			pgx.Identifier{"member_packages"},
			[]string{"id", "member_id", "package_type_id", "branch_id", "status", "start_date", "end_date", "remaining_entries", "total_entries", "price_paid_kurus", "auto_renew", "created_at", "updated_at"},
			pgx.CopyFromRows(pkgRows),
		)
		if err != nil {
			return fmt.Errorf("batch %d member_packages insert hatasi: %w", b+1, err)
		}
	}

	slog.Info("20.000 Üye ve Üyelik Paketleri basariyla olusturuldu")
	return nil
}
