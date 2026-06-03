package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/boliev/pixie-bot/internal/domain/generation"
)

type GenerationRepository struct {
	pool *pgxpool.Pool
}

func NewGenerationRepository(pool *pgxpool.Pool) *GenerationRepository {
	return &GenerationRepository{pool: pool}
}

func (r *GenerationRepository) Create(
	ctx context.Context,
	g *generation.Generation,
	images []*generation.GenerationImage,
) (*generation.Generation, error) {
	q := getQuerier(ctx, r.pool)

	err := q.QueryRow(ctx, `
		INSERT INTO generations (user_id, prompt, status, images_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, g.UserID, g.Prompt, g.Status, g.ImagesCount).
		Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert generation: %w", err)
	}

	for _, img := range images {
		img.GenerationID = g.ID

		err = q.QueryRow(ctx, `
			INSERT INTO generation_images (generation_id, telegram_file_id, position, created_at)
			VALUES ($1, $2, $3, NOW())
			RETURNING id, created_at
		`, img.GenerationID, img.TelegramFileID, img.Position).
			Scan(&img.ID, &img.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert generation image: %w", err)
		}
	}

	return g, nil
}

func (r *GenerationRepository) FindByID(ctx context.Context, id int64) (*generation.Generation, error) {
	q := getQuerier(ctx, r.pool)

	g := &generation.Generation{}

	err := q.QueryRow(ctx, `
		SELECT id, user_id, prompt, status, images_count, COALESCE(error, ''), created_at, updated_at
		FROM generations
		WHERE id = $1
	`, id).Scan(
		&g.ID, &g.UserID, &g.Prompt, &g.Status, &g.ImagesCount, &g.Error, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, generation.ErrNotFound
		}

		return nil, fmt.Errorf("find generation: %w", err)
	}

	return g, nil
}

func (r *GenerationRepository) UpdateStatus(ctx context.Context, id int64, status generation.Status, errMsg string) error {
	q := getQuerier(ctx, r.pool)

	tag, err := q.Exec(ctx,
		`UPDATE generations SET status = $2, error = $3, updated_at = NOW() WHERE id = $1`,
		id, status, strPtr(errMsg),
	)
	if err != nil {
		return fmt.Errorf("update generation status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return generation.ErrNotFound
	}

	return nil
}
