package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/boliev/pixie-bot/internal/application/ports"
	"github.com/boliev/pixie-bot/internal/domain/credits"
	"github.com/boliev/pixie-bot/internal/domain/payment"
)

type PaymentUseCase struct {
	userRepo    ports.UserRepository
	creditRepo  ports.CreditRepository
	paymentRepo ports.PaymentRepository
	txManager   ports.TxManager
	log         *slog.Logger
}

func NewPaymentUseCase(
	userRepo ports.UserRepository,
	creditRepo ports.CreditRepository,
	paymentRepo ports.PaymentRepository,
	txManager ports.TxManager,
	log *slog.Logger,
) *PaymentUseCase {
	return &PaymentUseCase{
		userRepo:    userRepo,
		creditRepo:  creditRepo,
		paymentRepo: paymentRepo,
		txManager:   txManager,
		log:         log,
	}
}

type CompletePaymentRequest struct {
	TelegramUserID          int64
	TelegramPaymentChargeID string
	ProviderPaymentChargeID string
	ProductCode             payment.ProductCode
}

func (uc *PaymentUseCase) CompletePayment(ctx context.Context, req CompletePaymentRequest) error {
	product, ok := payment.ProductCatalog[req.ProductCode]

	if !ok {
		return fmt.Errorf("unknown product: %s", req.ProductCode)
	}

	u, err := uc.userRepo.FindByTelegramID(ctx, req.TelegramUserID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	return uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		existing, findErr := uc.paymentRepo.FindByTelegramChargeID(ctx, req.TelegramPaymentChargeID)
		if findErr == nil {
			if existing.Status == payment.StatusPaid {
				return nil // idempotent: already processed
			}

			if updateErr := uc.paymentRepo.UpdateStatus(ctx, existing.ID, payment.StatusPaid,
				req.TelegramPaymentChargeID, req.ProviderPaymentChargeID); updateErr != nil {
				return fmt.Errorf("update payment status: %w", updateErr)
			}
		} else if !errors.Is(findErr, payment.ErrNotFound) {
			return fmt.Errorf("find payment: %w", findErr)
		} else {
			p := &payment.Payment{
				UserID:                  u.ID,
				TelegramPaymentChargeID: req.TelegramPaymentChargeID,
				ProviderPaymentChargeID: req.ProviderPaymentChargeID,
				ProductCode:             req.ProductCode,
				Credits:                 product.Credits,
				AmountStars:             product.AmountStars,
				Status:                  payment.StatusPaid,
			}

			if _, createErr := uc.paymentRepo.Create(ctx, p); createErr != nil {
				return fmt.Errorf("create payment: %w", createErr)
			}
		}

		if addErr := uc.userRepo.AddCredits(ctx, u.ID, product.Credits); addErr != nil {
			return fmt.Errorf("add credits: %w", addErr)
		}

		if _, createErr := uc.creditRepo.Create(ctx, &credits.CreditTransaction{
			UserID:     u.ID,
			Type:       credits.TypePurchase,
			Amount:     product.Credits,
			Reason:     fmt.Sprintf("purchase %s", req.ProductCode),
			ExternalID: req.TelegramPaymentChargeID,
		}); createErr != nil {
			return fmt.Errorf("record credit tx: %w", createErr)
		}

		return nil
	})
}
