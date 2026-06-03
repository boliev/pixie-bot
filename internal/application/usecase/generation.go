package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/boliev/pixie-bot/internal/application/ports"
	"github.com/boliev/pixie-bot/internal/domain/credits"
	"github.com/boliev/pixie-bot/internal/domain/generation"
	"github.com/boliev/pixie-bot/internal/domain/user"
)

var ErrInsufficientCredits = errors.New("insufficient credits")

type GenerationUseCase struct {
	userRepo       ports.UserRepository
	creditRepo     ports.CreditRepository
	generationRepo ports.GenerationRepository
	imageEditor    ports.ImageEditor
	fileFetcher    ports.FileFetcher
	txManager      ports.TxManager
	log            *slog.Logger
}

func NewGenerationUseCase(
	userRepo ports.UserRepository,
	creditRepo ports.CreditRepository,
	generationRepo ports.GenerationRepository,
	imageEditor ports.ImageEditor,
	fileFetcher ports.FileFetcher,
	txManager ports.TxManager,
	log *slog.Logger,
) *GenerationUseCase {
	return &GenerationUseCase{
		userRepo:       userRepo,
		creditRepo:     creditRepo,
		generationRepo: generationRepo,
		imageEditor:    imageEditor,
		fileFetcher:    fileFetcher,
		txManager:      txManager,
		log:            log,
	}
}

type StartGenerationRequest struct {
	TelegramUserID int64
	FileIDs        []string
	Prompt         string
}

type StartGenerationResult struct {
	GenerationID int64
	ImageData    []byte
}

func (uc *GenerationUseCase) StartGeneration(ctx context.Context, req StartGenerationRequest) (*StartGenerationResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, generation.ErrEmptyPrompt
	}

	if len(req.FileIDs) == 0 {
		return nil, generation.ErrNoImages
	}

	if len(req.FileIDs) > generation.MaxImages {
		return nil, generation.ErrTooManyImages
	}

	u, err := uc.userRepo.FindByTelegramID(ctx, req.TelegramUserID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	if u.Credits < 1 {
		return nil, ErrInsufficientCredits
	}

	var gen *generation.Generation

	txErr := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		if addErr := uc.userRepo.AddCredits(ctx, u.ID, -1); addErr != nil {
			if errors.Is(addErr, user.ErrInsufficientCredits) {
				return ErrInsufficientCredits
			}

			return fmt.Errorf("debit credit: %w", addErr)
		}

		if _, addErr := uc.creditRepo.Create(ctx, &credits.CreditTransaction{
			UserID: u.ID,
			Type:   credits.TypeDebit,
			Amount: -1,
			Reason: "image generation",
		}); addErr != nil {
			return fmt.Errorf("record debit tx: %w", addErr)
		}

		images := make([]*generation.GenerationImage, len(req.FileIDs))

		for i, fid := range req.FileIDs {
			images[i] = &generation.GenerationImage{
				TelegramFileID: fid,
				Position:       i,
			}
		}

		var createErr error

		gen, createErr = uc.generationRepo.Create(ctx, &generation.Generation{
			UserID:      u.ID,
			Prompt:      req.Prompt,
			Status:      generation.StatusProcessing,
			ImagesCount: len(req.FileIDs),
		}, images)
		if createErr != nil {
			return fmt.Errorf("create generation: %w", createErr)
		}

		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	// Download files and call OpenAI outside the transaction.
	imageBytes := make([][]byte, len(req.FileIDs))

	for i, fileID := range req.FileIDs {
		data, dlErr := uc.fileFetcher.DownloadFile(ctx, fileID)
		if dlErr != nil {
			uc.failAndRefund(context.WithoutCancel(ctx), u.ID, gen.ID, fmt.Sprintf("download failed: %v", dlErr))

			return nil, fmt.Errorf("download file: %w", dlErr)
		}

		imageBytes[i] = data
	}

	resultData, editErr := uc.imageEditor.EditImages(ctx, imageBytes, req.Prompt)
	if editErr != nil {
		uc.failAndRefund(context.WithoutCancel(ctx), u.ID, gen.ID, fmt.Sprintf("openai failed: %v", editErr))

		return nil, fmt.Errorf("edit images: %w", editErr)
	}

	return &StartGenerationResult{
		GenerationID: gen.ID,
		ImageData:    resultData,
	}, nil
}

func (uc *GenerationUseCase) CompleteGeneration(ctx context.Context, generationID int64) error {
	return uc.generationRepo.UpdateStatus(ctx, generationID, generation.StatusDone, "")
}

// FailGenerationAndRefund marks the generation failed and refunds the credit.
// It is idempotent: if the generation is not in processing state, it is a no-op.
func (uc *GenerationUseCase) FailGenerationAndRefund(ctx context.Context, generationID int64) error {
	return uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		gen, err := uc.generationRepo.FindByID(ctx, generationID)
		if err != nil {
			return fmt.Errorf("find generation: %w", err)
		}

		if gen.Status != generation.StatusProcessing {
			return nil
		}

		if err = uc.generationRepo.UpdateStatus(ctx, generationID, generation.StatusFailed, "telegram send failed"); err != nil {
			return fmt.Errorf("update status: %w", err)
		}

		if err = uc.userRepo.AddCredits(ctx, gen.UserID, 1); err != nil {
			return fmt.Errorf("refund credit: %w", err)
		}

		if _, err = uc.creditRepo.Create(ctx, &credits.CreditTransaction{
			UserID: gen.UserID,
			Type:   credits.TypeRefund,
			Amount: 1,
			Reason: "generation failed",
		}); err != nil {
			return fmt.Errorf("record refund tx: %w", err)
		}

		return nil
	})
}

func (uc *GenerationUseCase) failAndRefund(ctx context.Context, userID, genID int64, errMsg string) {
	err := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		if updateErr := uc.generationRepo.UpdateStatus(ctx, genID, generation.StatusFailed, errMsg); updateErr != nil {
			return updateErr
		}

		if addErr := uc.userRepo.AddCredits(ctx, userID, 1); addErr != nil {
			return addErr
		}

		_, createErr := uc.creditRepo.Create(ctx, &credits.CreditTransaction{
			UserID: userID,
			Type:   credits.TypeRefund,
			Amount: 1,
			Reason: "generation failed",
		})

		return createErr
	})
	if err != nil {
		uc.log.Error("failAndRefund failed", "userID", userID, "genID", genID, "err", err)
	}
}
