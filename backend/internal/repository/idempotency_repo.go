package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitgiris/internal/domain"
)

type IdempotencyRepository interface {
	Get(ctx context.Context, key string) (*domain.IdempotencyRecord, error)
	Save(ctx context.Context, record *domain.IdempotencyRecord) error
}

type PostgresIdempotencyRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresIdempotencyRepository(pool *pgxpool.Pool) *PostgresIdempotencyRepository {
	return &PostgresIdempotencyRepository{pool: pool}
}

func (r *PostgresIdempotencyRepository) Get(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	query := `
		SELECT id, key, user_id, endpoint, status_code, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE key = $1 AND expires_at > NOW()
	`
	var rec domain.IdempotencyRecord
	err := r.pool.QueryRow(ctx, query, key).Scan(
		&rec.ID,
		&rec.Key,
		&rec.UserID,
		&rec.Endpoint,
		&rec.StatusCode,
		&rec.ResponseBody,
		&rec.CreatedAt,
		&rec.ExpiresAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // not found or expired
		}
		return nil, fmt.Errorf("idempotency anahtari sorgulanamadi: %w", err)
	}

	return &rec, nil
}

func (r *PostgresIdempotencyRepository) Save(ctx context.Context, record *domain.IdempotencyRecord) error {
	query := `
		INSERT INTO idempotency_keys (
			id, key, user_id, endpoint, status_code, response_body, created_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		ON CONFLICT (key) DO NOTHING
	`
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.ExpiresAt.IsZero() {
		record.ExpiresAt = record.CreatedAt.Add(24 * time.Hour)
	}

	_, err := r.pool.Exec(ctx, query,
		record.ID,
		record.Key,
		record.UserID,
		record.Endpoint,
		record.StatusCode,
		record.ResponseBody,
		record.CreatedAt,
		record.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("idempotency kaydi kaydedilemedi: %w", err)
	}

	return nil
}
