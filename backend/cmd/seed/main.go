package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"fitgiris/internal/platform/config"
	"fitgiris/internal/platform/database"
	"fitgiris/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	reset := flag.Bool("reset", false, "Mevcut verileri temizle ve demo verilerini bastan olustur")
	seedVal := flag.Int64("seed", 42, "Deterministik veri uretim tohumu")
	memberCount := flag.Int("members", 20000, "Olusturulacak rastgele uye sayisi")
	flag.Parse()

	cfg := config.Load()

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Veritabani baglantisi basarisiz", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Ensure migrations are run first
	if err := database.RunMigrations(ctx, pool, migrations.FS, "."); err != nil {
		slog.Error("Migrasyon hatasi", "error", err)
		os.Exit(1)
	}

	opts := database.SeedOptions{
		SeedValue:   *seedVal,
		MemberCount: *memberCount,
		Reset:       *reset,
	}

	if err := database.SeedDatabase(ctx, pool, opts); err != nil {
		slog.Error("Seed islemi basarisiz oldu", "error", err)
		os.Exit(1)
	}

	slog.Info("Seed islemi basariyla tamamlandi!")
	os.Exit(0)
}
