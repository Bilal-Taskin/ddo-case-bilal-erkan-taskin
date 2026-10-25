package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitgiris/internal/domain"
)

type AuditRepository interface {
	Append(ctx context.Context, log *domain.AuditLog) error
	List(ctx context.Context, limit, offset int) ([]domain.AuditLog, error)
}

type PostgresAuditRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAuditRepository(pool *pgxpool.Pool) *PostgresAuditRepository {
	return &PostgresAuditRepository{pool: pool}
}

func (r *PostgresAuditRepository) Append(ctx context.Context, log *domain.AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			id, user_id, action, entity, entity_id, old_values, new_values, ip_address, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, query,
		log.ID,
		log.UserID,
		log.Action,
		log.Entity,
		log.EntityID,
		log.OldValues,
		log.NewValues,
		log.IPAddress,
		log.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("audit log eklenemedi: %w", err)
	}

	return nil
}

func (r *PostgresAuditRepository) List(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, user_id, action, entity, entity_id, old_values, new_values, ip_address, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("audit loglar listelenemedi: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		if err := rows.Scan(
			&l.ID,
			&l.UserID,
			&l.Action,
			&l.Entity,
			&l.EntityID,
			&l.OldValues,
			&l.NewValues,
			&l.IPAddress,
			&l.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("audit log satiri okunamadi: %w", err)
		}
		logs = append(logs, l)
	}

	return logs, nil
}
