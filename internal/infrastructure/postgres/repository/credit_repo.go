package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/boliev/pixie-bot/internal/domain/credits"
)

type CreditRepository struct {
	pool *pgxpool.Pool
}

func NewCreditRepository(pool *pgxpool.Pool) *CreditRepository {
	return &CreditRepository{pool: pool}
}

func (r *CreditRepository) Create(ctx context.Context, tx *credits.CreditTransaction) (*credits.CreditTransaction, error) {
	q := getQuerier(ctx, r.pool)

	err := q.QueryRow(ctx, `
		INSERT INTO credit_transactions (user_id, type, amount, reason, external_id, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, created_at
	`, tx.UserID, tx.Type, tx.Amount, strPtr(tx.Reason), strPtr(tx.ExternalID)).
		Scan(&tx.ID, &tx.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert credit tx: %w", err)
	}

	return tx, nil
}

func (r *CreditRepository) FindByExternalID(ctx context.Context, externalID string) (*credits.CreditTransaction, error) {
	q := getQuerier(ctx, r.pool)

	tx := &credits.CreditTransaction{}

	err := q.QueryRow(ctx, `
		SELECT id, user_id, type, amount, COALESCE(reason, ''), COALESCE(external_id, ''), created_at
		FROM credit_transactions
		WHERE external_id = $1
		LIMIT 1
	`, externalID).Scan(
		&tx.ID, &tx.UserID, &tx.Type, &tx.Amount, &tx.Reason, &tx.ExternalID, &tx.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("not found")
		}

		return nil, fmt.Errorf("find credit tx: %w", err)
	}

	return tx, nil
}
