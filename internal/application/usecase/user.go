package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/boliev/pixie-bot/internal/application/ports"
	"github.com/boliev/pixie-bot/internal/domain/credits"
	"github.com/boliev/pixie-bot/internal/domain/user"
)

type UserUseCase struct {
	userRepo   ports.UserRepository
	creditRepo ports.CreditRepository
	txManager  ports.TxManager
	log        *slog.Logger
}

func NewUserUseCase(
	userRepo ports.UserRepository,
	creditRepo ports.CreditRepository,
	txManager ports.TxManager,
	log *slog.Logger,
) *UserUseCase {
	return &UserUseCase{
		userRepo:   userRepo,
		creditRepo: creditRepo,
		txManager:  txManager,
		log:        log,
	}
}

type RegisterUserRequest struct {
	TelegramUserID int64
	Username       string
	FirstName      string
	LastName       string
}

func (uc *UserUseCase) RegisterUser(ctx context.Context, req RegisterUserRequest) (*user.User, error) {
	existing, err := uc.userRepo.FindByTelegramID(ctx, req.TelegramUserID)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, user.ErrNotFound) {
		return nil, fmt.Errorf("find user: %w", err)
	}

	var created *user.User

	txErr := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		newUser := user.New(req.TelegramUserID, req.Username, req.FirstName, req.LastName)

		var createErr error

		created, createErr = uc.userRepo.Create(ctx, newUser)
		if createErr != nil {
			if errors.Is(createErr, user.ErrAlreadyExists) {
				// race: another request created the user between our find and create
				created, createErr = uc.userRepo.FindByTelegramID(ctx, req.TelegramUserID)

				return createErr
			}

			return fmt.Errorf("create user: %w", createErr)
		}

		if createErr = uc.userRepo.AddCredits(ctx, created.ID, user.InitialCredits); createErr != nil {
			return fmt.Errorf("add initial credits: %w", createErr)
		}

		if _, createErr = uc.creditRepo.Create(ctx, &credits.CreditTransaction{
			UserID: created.ID,
			Type:   credits.TypeBonus,
			Amount: user.InitialCredits,
			Reason: "welcome bonus",
		}); createErr != nil {
			return fmt.Errorf("record credit tx: %w", createErr)
		}

		created.Credits = user.InitialCredits

		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	return created, nil
}

func (uc *UserUseCase) GetBalance(ctx context.Context, telegramUserID int64) (int, error) {
	u, err := uc.userRepo.FindByTelegramID(ctx, telegramUserID)
	if err != nil {
		return 0, fmt.Errorf("find user: %w", err)
	}

	return u.Credits, nil
}
