package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	pginfra "github.com/boliev/pixie-bot/internal/infrastructure/postgres"
)

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func getQuerier(ctx context.Context, pool *pgxpool.Pool) querier {
	if tx, ok := ctx.Value(pginfra.TxContextKey).(pgx.Tx); ok {
		return tx
	}

	return pool
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
