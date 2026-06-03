package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/boliev/pixie-bot/internal/domain/user"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) (*user.User, error) {
	q := getQuerier(ctx, r.pool)

	err := q.QueryRow(ctx, `
		INSERT INTO users (telegram_user_id, username, first_name, last_name, credits, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 0, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, u.TelegramUserID, strPtr(u.Username), strPtr(u.FirstName), strPtr(u.LastName)).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, user.ErrAlreadyExists
		}

		return nil, fmt.Errorf("insert user: %w", err)
	}

	return u, nil
}

func (r *UserRepository) FindByTelegramID(ctx context.Context, telegramUserID int64) (*user.User, error) {
	q := getQuerier(ctx, r.pool)

	u := &user.User{}

	err := q.QueryRow(ctx, `
		SELECT id, telegram_user_id,
			COALESCE(username, ''), COALESCE(first_name, ''), COALESCE(last_name, ''),
			credits, created_at, updated_at
		FROM users
		WHERE telegram_user_id = $1
	`, telegramUserID).Scan(
		&u.ID, &u.TelegramUserID, &u.Username, &u.FirstName, &u.LastName,
		&u.Credits, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}

		return nil, fmt.Errorf("find user: %w", err)
	}

	return u, nil
}

func (r *UserRepository) AddCredits(ctx context.Context, userID int64, delta int) error {
	q := getQuerier(ctx, r.pool)

	tag, err := q.Exec(ctx,
		`UPDATE users SET credits = credits + $2, updated_at = NOW() WHERE id = $1`,
		userID, delta,
	)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23514" { // check_violation: credits >= 0
			return user.ErrInsufficientCredits
		}

		return fmt.Errorf("update credits: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return user.ErrNotFound
	}

	return nil
}
