package database

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations executes all SQL files found in the given filesystem in alphabetical order.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS, dir string) error {
	// Create schema_migrations table if not exists
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("schema_migrations tablosu olusturulamadi: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, dir)
	if err != nil {
		return fmt.Errorf("migrasyon dizini okunamadi: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, filename := range files {
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", filename).Scan(&exists)
		if err != nil {
			return fmt.Errorf("migrasyon kontrol sorgusu basarisiz (%s): %w", filename, err)
		}

		if exists {
			continue
		}

		var filePath string
		if dir == "" || dir == "." {
			filePath = filename
		} else {
			filePath = filepath.Join(dir, filename)
		}
		content, err := fs.ReadFile(migrationFS, filePath)
		if err != nil {
			return fmt.Errorf("migrasyon dosyasi okunamadi (%s): %w", filename, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migrasyon transaction baslatilamadi: %w", err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrasyon calistirma hatasi (%s): %w", filename, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrasyon kaydi eklenemedi (%s): %w", filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrasyon commit edilemedi (%s): %w", filename, err)
		}

		slog.Info("Migrasyon basariyla uygulandi", "version", filename)
	}

	return nil
}
