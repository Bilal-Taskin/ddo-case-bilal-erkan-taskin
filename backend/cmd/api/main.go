package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"fitgiris/internal/domain"
	"fitgiris/internal/handler"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/config"
	"fitgiris/internal/platform/database"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/platform/security"
	"fitgiris/internal/repository"
	"fitgiris/internal/service"
	"fitgiris/migrations"
)

func main() {
	// Structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()
	slog.Info("Uygulama baslatiliyor", "environment", cfg.Environment, "port", cfg.Port)

	// Context with cancel for startup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Database Pool
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Veritabani baglanti havuzu olusturulamadi", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("Veritabani baglantisi basarili")

	// Run Database Migrations automatically
	if err := database.RunMigrations(ctx, pool, migrations.FS, "."); err != nil {
		slog.Error("Veritabani migrasyonlari calistirilamadi", "error", err)
		os.Exit(1)
	}
	slog.Info("Veritabani migrasyonlari kontrol edildi ve guncellendi")

	// Automatic initial seeding (proje.md Section 3 requirement)
	var branchCount int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM branches").Scan(&branchCount)
	if branchCount == 0 {
		slog.Info("Ilk calisma tespit edildi, otomatik seed islemi baslatiliyor (5 sube, 20.000 uye, tohum=42)...")
		if err := database.SeedDatabase(ctx, pool, database.SeedOptions{
			SeedValue:   42,
			MemberCount: 20000,
			Reset:       false,
		}); err != nil {
			slog.Error("Otomatik seed islemi hatasi", "error", err)
		} else {
			slog.Info("Otomatik seed islemi basariyla tamamlandi")
		}
	}

	// Platform Services & Security
	jwtManager := security.NewJWTManager(cfg.JWTSecret)

	// Repositories
	userRepo := repository.NewPostgresUserRepository(pool)
	auditRepo := repository.NewPostgresAuditRepository(pool)
	idempotencyRepo := repository.NewPostgresIdempotencyRepository(pool)
	membershipRepo := repository.NewPostgresMembershipRepository(pool)
	bookingRepo := repository.NewPostgresBookingRepository(pool)
	turnstileRepo := repository.NewPostgresTurnstileRepository(pool)
	billingRepo := repository.NewPostgresBillingRepository(pool)
	ptRepo := repository.NewPostgresPTRepository(pool)
	reportRepo := repository.NewPostgresReportRepository(pool)

	// Platform Services & Mock Providers
	paymentProv := service.NewMockPaymentProvider()

	// Services
	authService := service.NewAuthService(userRepo, auditRepo, jwtManager)
	membershipService := service.NewMembershipService(membershipRepo, userRepo, auditRepo)
	bookingService := service.NewBookingService(bookingRepo, userRepo, auditRepo)
	turnstileService := service.NewTurnstileService(turnstileRepo, userRepo, auditRepo)
	billingService := service.NewBillingService(billingRepo, membershipRepo, userRepo, auditRepo, paymentProv)
	ptService := service.NewPTService(ptRepo, userRepo, auditRepo)
	reportService := service.NewReportService(reportRepo)

	// Middlewares
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	idempotencyMiddleware := middleware.NewIdempotencyMiddleware(idempotencyRepo)

	// Handlers
	healthHandler := handler.NewHealthHandler(pool)
	authHandler := handler.NewAuthHandler(authService)
	membershipHandler := handler.NewMembershipHandler(membershipService)
	bookingHandler := handler.NewBookingHandler(bookingService)
	turnstileHandler := handler.NewTurnstileHandler(turnstileService)
	billingHandler := handler.NewBillingHandler(billingService)
	ptHandler := handler.NewPTHandler(ptService)
	reportHandler := handler.NewReportHandler(reportService)

	// Router setup
	r := chi.NewRouter()

	// Base Middlewares
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))

	// CORS policy
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "Idempotency-Key"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Idempotency Middleware for mutating requests
	r.Use(idempotencyMiddleware.Handle)

	// Health Handler
	r.Get("/health", healthHandler.HealthCheck)

	// API v1 Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			response.JSON(w, http.StatusOK, map[string]string{"message": "FitGiris API v1 calisiyor"})
		})

		// Public Auth Routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)

			// Protected Current User Route
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware.Authenticate)
				r.Get("/me", authHandler.Me)
			})
		})

		// Package Types Routes
		r.Route("/packages", func(r chi.Router) {
			r.Get("/", membershipHandler.ListPackageTypes)
			r.Get("/{id}", membershipHandler.GetPackageType)

			// Manager Only Package Creation
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware.Authenticate)
				r.With(middleware.RequireRoles(domain.RoleManager)).Post("/", membershipHandler.CreatePackageType)
			})
		})

		// Membership Lifecycle Routes (Protected)
		r.Route("/memberships", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			r.Get("/", membershipHandler.List)
			r.Get("/{id}", membershipHandler.GetByID)
			r.Post("/purchase", membershipHandler.Purchase)
			r.Post("/{id}/freeze", membershipHandler.Freeze)
			r.Post("/{id}/unfreeze", membershipHandler.Unfreeze)
			r.Post("/{id}/transfer", membershipHandler.Transfer)
			r.Get("/{id}/upgrade-quote", membershipHandler.UpgradeQuote)
			r.Post("/{id}/upgrade", membershipHandler.Upgrade)
			r.Post("/{id}/cancel", membershipHandler.Cancel)
		})

		// Class & Schedule Management, Bookings, Waitlist (Protected)
		r.Route("/classes", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			// Schedules & Classes listing
			r.Get("/", bookingHandler.ListClasses)
			r.Get("/schedules", bookingHandler.ListSchedules)
			r.Get("/schedules/{id}", bookingHandler.GetSchedule)

			// Member personal bookings & waitlist
			r.With(middleware.RequireRoles(domain.RoleMember)).Get("/my-bookings", bookingHandler.ListMyBookings)
			r.With(middleware.RequireRoles(domain.RoleMember)).Get("/my-waitlist", bookingHandler.ListMyWaitlist)
			r.With(middleware.RequireRoles(domain.RoleMember)).Post("/schedules/{id}/book", bookingHandler.BookClass)
			r.With(middleware.RequireRoles(domain.RoleMember)).Post("/bookings/{id}/cancel", bookingHandler.CancelBooking)
			r.With(middleware.RequireRoles(domain.RoleMember)).Post("/waitlist/{id}/cancel", bookingHandler.CancelWaitlist)

			// Manager Only: Create class definition
			r.With(middleware.RequireRoles(domain.RoleManager)).Post("/", bookingHandler.CreateClass)

			// Manager & Trainer: Schedule sessions
			r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleTrainer)).Post("/schedules", bookingHandler.CreateSchedule)
			r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleTrainer, domain.RoleReception)).Get("/schedules/{id}/bookings", bookingHandler.ListScheduleBookings)
			r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleTrainer, domain.RoleReception)).Get("/schedules/{id}/waitlist", bookingHandler.ListScheduleWaitlist)

			// Attendance marking: Trainer, Reception, Manager
			r.With(middleware.RequireRoles(domain.RoleTrainer, domain.RoleReception, domain.RoleManager)).Post("/schedules/{id}/attendance", bookingHandler.MarkAttendance)
		})

		// Turnstile Access Control & Offline Sync (Protected)
		r.Route("/turnstiles", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			// Fast turnstile pass evaluation (Reception, Manager)
			r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Post("/access", turnstileHandler.EvaluateAccess)

			// Offline logs sync & anomaly check (Reception, Manager)
			r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Post("/sync-offline", turnstileHandler.SyncOffline)

			// Access log history
			r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Get("/logs", turnstileHandler.ListAccessLogs)

			// Turnstile gate definitions
			r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Get("/", turnstileHandler.ListTurnstiles)
			r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Get("/{id}", turnstileHandler.GetTurnstile)
			r.With(middleware.RequireRoles(domain.RoleManager)).Post("/", turnstileHandler.CreateTurnstile)
		})

		// Billing, Gapless Invoices & Auto-Renewal Queue (Protected)
		r.Route("/billing", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			// Invoices
			r.Get("/invoices", billingHandler.ListInvoices)
			r.Get("/invoices/{id}", billingHandler.GetInvoice)
			r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleAccountant)).Post("/invoices", billingHandler.CreateInvoice)
			r.Post("/invoices/{id}/pay", billingHandler.PayInvoice)

			// Auto-Renewal Queue & Retries
			r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleAccountant)).Post("/auto-renewals/process", billingHandler.ProcessAutoRenewals)
			r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleAccountant)).Get("/auto-renewals", billingHandler.ListAutoRenewals)
		})

		// Personal Training (PT) Routes (Protected)
		r.Route("/pt", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			r.Post("/packages", ptHandler.PurchasePackage)
			r.Get("/packages", ptHandler.ListPackages)
			r.Post("/appointments", ptHandler.BookAppointment)
			r.Get("/appointments", ptHandler.ListAppointments)
			r.Post("/appointments/{id}/cancel", ptHandler.CancelAppointment)
			r.With(middleware.RequireRoles(domain.RoleTrainer, domain.RoleManager)).Post("/appointments/{id}/complete", ptHandler.CompleteAppointment)
		})

		// Reports & Analytics Routes (Protected)
		r.Route("/reports", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleAccountant)).Get("/revenue", reportHandler.RevenueReport)
			r.With(middleware.RequireRoles(domain.RoleManager, domain.RoleReception)).Get("/occupancy", reportHandler.OccupancyReport)
			r.With(middleware.RequireRoles(domain.RoleManager)).Get("/trainers", reportHandler.TrainerReport)
		})

		// Protected RBAC Verification Routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			// Manager Only
			r.With(middleware.RequireRoles(domain.RoleManager)).Get("/manager/dashboard", func(w http.ResponseWriter, r *http.Request) {
				response.JSON(w, http.StatusOK, map[string]string{"access": "granted", "role": "MANAGER"})
			})

			// Trainer Only
			r.With(middleware.RequireRoles(domain.RoleTrainer)).Get("/trainer/schedule", func(w http.ResponseWriter, r *http.Request) {
				response.JSON(w, http.StatusOK, map[string]string{"access": "granted", "role": "TRAINER"})
			})

			// Reception & Manager
			r.With(middleware.RequireRoles(domain.RoleReception, domain.RoleManager)).Get("/reception/overview", func(w http.ResponseWriter, r *http.Request) {
				response.JSON(w, http.StatusOK, map[string]string{"access": "granted", "role": "RECEPTION_OR_MANAGER"})
			})

			// Accountant & Manager
			r.With(middleware.RequireRoles(domain.RoleAccountant, domain.RoleManager)).Get("/accountant/invoices", func(w http.ResponseWriter, r *http.Request) {
				response.JSON(w, http.StatusOK, map[string]string{"access": "granted", "role": "ACCOUNTANT_OR_MANAGER"})
			})

			// Member Only
			r.With(middleware.RequireRoles(domain.RoleMember)).Get("/member/portal", func(w http.ResponseWriter, r *http.Request) {
				response.JSON(w, http.StatusOK, map[string]string{"access": "granted", "role": "MEMBER"})
			})
		})
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server run loop with Graceful Shutdown
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("HTTP Sunucusu dinliyor", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		slog.Error("Sunucu baslatma hatasi", "error", err)
		os.Exit(1)
	case sig := <-shutdown:
		slog.Info("Kapanis sinyali alindi, sunucu durduruluyor", "signal", sig.String())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Sunucu graceful shutdown hatasi", "error", err)
			_ = server.Close()
		}
		slog.Info("Sunucu basariyla kapatildi")
	}
}
