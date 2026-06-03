package ports

import (
	"context"

	"github.com/boliev/pixie-bot/internal/domain/credits"
	"github.com/boliev/pixie-bot/internal/domain/generation"
	"github.com/boliev/pixie-bot/internal/domain/payment"
	"github.com/boliev/pixie-bot/internal/domain/user"
)

type UserRepository interface {
	Create(ctx context.Context, u *user.User) (*user.User, error)
	FindByTelegramID(ctx context.Context, telegramUserID int64) (*user.User, error)
	AddCredits(ctx context.Context, userID int64, delta int) error
}

type CreditRepository interface {
	Create(ctx context.Context, tx *credits.CreditTransaction) (*credits.CreditTransaction, error)
	FindByExternalID(ctx context.Context, externalID string) (*credits.CreditTransaction, error)
}

type GenerationRepository interface {
	Create(ctx context.Context, g *generation.Generation, images []*generation.GenerationImage) (*generation.Generation, error)
	FindByID(ctx context.Context, id int64) (*generation.Generation, error)
	UpdateStatus(ctx context.Context, id int64, status generation.Status, errMsg string) error
}

type PaymentRepository interface {
	Create(ctx context.Context, p *payment.Payment) (*payment.Payment, error)
	FindByTelegramChargeID(ctx context.Context, chargeID string) (*payment.Payment, error)
	UpdateStatus(ctx context.Context, id int64, status payment.Status, telegramChargeID, providerChargeID string) error
}

type ImageEditor interface {
	EditImages(ctx context.Context, images [][]byte, prompt string) ([]byte, error)
}

type FileFetcher interface {
	DownloadFile(ctx context.Context, fileID string) ([]byte, error)
}

type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
