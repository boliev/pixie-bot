package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/boliev/pixie-bot/internal/domain/payment"
)

type PaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

func (r *PaymentRepository) Create(ctx context.Context, p *payment.Payment) (*payment.Payment, error) {
	q := getQuerier(ctx, r.pool)

	err := q.QueryRow(ctx, `
		INSERT INTO payments (user_id, telegram_payment_charge_id, provider_payment_charge_id, product_code, credits, amount_stars, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, p.UserID,
		strPtr(p.TelegramPaymentChargeID),
		strPtr(p.ProviderPaymentChargeID),
		p.ProductCode, p.Credits, p.AmountStars, p.Status).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, payment.ErrAlreadyPaid
		}

		return nil, fmt.Errorf("insert payment: %w", err)
	}

	return p, nil
}

func (r *PaymentRepository) FindByTelegramChargeID(ctx context.Context, chargeID string) (*payment.Payment, error) {
	q := getQuerier(ctx, r.pool)

	p := &payment.Payment{}

	err := q.QueryRow(ctx, `
		SELECT id, user_id,
			COALESCE(telegram_payment_charge_id, ''),
			COALESCE(provider_payment_charge_id, ''),
			product_code, credits, amount_stars, status, created_at, updated_at
		FROM payments
		WHERE telegram_payment_charge_id = $1
	`, chargeID).Scan(
		&p.ID, &p.UserID, &p.TelegramPaymentChargeID, &p.ProviderPaymentChargeID,
		&p.ProductCode, &p.Credits, &p.AmountStars, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, payment.ErrNotFound
		}

		return nil, fmt.Errorf("find payment: %w", err)
	}

	return p, nil
}

func (r *PaymentRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status payment.Status,
	telegramChargeID, providerChargeID string,
) error {
	q := getQuerier(ctx, r.pool)

	tag, err := q.Exec(ctx, `
		UPDATE payments
		SET status = $2, telegram_payment_charge_id = $3, provider_payment_charge_id = $4, updated_at = NOW()
		WHERE id = $1
	`, id, status, strPtr(telegramChargeID), strPtr(providerChargeID))
	if err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return payment.ErrNotFound
	}

	return nil
}
