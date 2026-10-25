package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitgiris/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	IsRefreshTokenValid(ctx context.Context, tokenHash string) (bool, uuid.UUID, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}

type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (
			id, email, password_hash, first_name, last_name, phone, role, home_branch_id, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.Phone,
		string(user.Role),
		user.HomeBranchID,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
			return domain.ErrUserAlreadyExists
		}
		return fmt.Errorf("kullanici olusturulamadi: %w", err)
	}

	return nil
}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT 
			id, email, password_hash, first_name, last_name, phone, role, home_branch_id, is_active, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u domain.User
	var roleStr string

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FirstName,
		&u.LastName,
		&u.Phone,
		&roleStr,
		&u.HomeBranchID,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("kullanici sorgulanamadi: %w", err)
	}

	u.Role = domain.UserRole(roleStr)
	return &u, nil
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT 
			id, email, password_hash, first_name, last_name, phone, role, home_branch_id, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u domain.User
	var roleStr string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FirstName,
		&u.LastName,
		&u.Phone,
		&roleStr,
		&u.HomeBranchID,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("kullanici sorgulanamadi: %w", err)
	}

	u.Role = domain.UserRole(roleStr)
	return &u, nil
}

func (r *PostgresUserRepository) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, false, NOW())
	`
	_, err := r.pool.Exec(ctx, query, userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("refresh token kaydedilemedi: %w", err)
	}
	return nil
}

func (r *PostgresUserRepository) IsRefreshTokenValid(ctx context.Context, tokenHash string) (bool, uuid.UUID, error) {
	query := `
		SELECT user_id, expires_at, revoked
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var userID uuid.UUID
	var expiresAt time.Time
	var revoked bool

	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(&userID, &expiresAt, &revoked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, uuid.Nil, nil
		}
		return false, uuid.Nil, err
	}

	if revoked || time.Now().After(expiresAt) {
		return false, userID, nil
	}

	return true, userID, nil
}

func (r *PostgresUserRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = true
		WHERE token_hash = $1
	`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	return err
}
