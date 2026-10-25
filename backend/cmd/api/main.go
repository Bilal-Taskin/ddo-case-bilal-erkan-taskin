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
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"fitgiris/internal/handler"
	"fitgiris/internal/platform/config"
	"fitgiris/internal/platform/database"
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

	// Router setup
	r := chi.NewRouter()

	// Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS policy
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "Idempotency-Key"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health Handler
	healthHandler := handler.NewHealthHandler(pool)
	r.Get("/health", healthHandler.HealthCheck)

	// API v1 placeholder
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"FitGiris API v1 calisiyor"}`))
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
