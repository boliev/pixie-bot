package usecase_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/boliev/pixie-bot/internal/application/usecase"
	"github.com/boliev/pixie-bot/internal/domain/payment"
	"github.com/boliev/pixie-bot/internal/domain/user"
)

type mockPaymentRepo struct {
	payments map[string]*payment.Payment
	nextID   int64
}

func newMockPaymentRepo() *mockPaymentRepo {
	return &mockPaymentRepo{payments: make(map[string]*payment.Payment)}
}

func (m *mockPaymentRepo) Create(ctx context.Context, p *payment.Payment) (*payment.Payment, error) {
	m.nextID++

	p.ID = m.nextID

	m.payments[p.TelegramPaymentChargeID] = p

	return p, nil
}

func (m *mockPaymentRepo) FindByTelegramChargeID(ctx context.Context, chargeID string) (*payment.Payment, error) {
	p, ok := m.payments[chargeID]

	if !ok {
		return nil, payment.ErrNotFound
	}

	return p, nil
}

func (m *mockPaymentRepo) UpdateStatus(ctx context.Context, id int64, status payment.Status, telegramChargeID, providerChargeID string) error {
	for _, p := range m.payments {
		if p.ID == id {
			p.Status = status

			return nil
		}
	}

	return payment.ErrNotFound
}

func newPaymentUC(userRepo *mockUserRepo, creditRepo *mockCreditRepo, paymentRepo *mockPaymentRepo) *usecase.PaymentUseCase {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	return usecase.NewPaymentUseCase(userRepo, creditRepo, paymentRepo, &noopTxManager{}, log)
}

func TestCompletePayment_CreditsAreAdded(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	paymentRepo := newMockPaymentRepo()

	mustRegisterUser(t, userRepo, creditRepo, 4001)

	u, err := userRepo.FindByTelegramID(context.Background(), 4001)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	initialCredits := u.Credits

	uc := newPaymentUC(userRepo, creditRepo, paymentRepo)

	err = uc.CompletePayment(context.Background(), usecase.CompletePaymentRequest{
		TelegramUserID:          4001,
		TelegramPaymentChargeID: "charge_abc",
		ProviderPaymentChargeID: "provider_xyz",
		ProductCode:             payment.Product5Credits,
	})
	if err != nil {
		t.Fatalf("CompletePayment error: %v", err)
	}

	u, err = userRepo.FindByTelegramID(context.Background(), 4001)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	product := payment.ProductCatalog[payment.Product5Credits]

	if u.Credits != initialCredits+product.Credits {
		t.Errorf("expected %d credits, got %d", initialCredits+product.Credits, u.Credits)
	}
}

func TestCompletePayment_IdempotentByChargeID(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	paymentRepo := newMockPaymentRepo()

	mustRegisterUser(t, userRepo, creditRepo, 4002)

	uc := newPaymentUC(userRepo, creditRepo, paymentRepo)

	req := usecase.CompletePaymentRequest{
		TelegramUserID:          4002,
		TelegramPaymentChargeID: "charge_dup",
		ProviderPaymentChargeID: "provider_dup",
		ProductCode:             payment.Product10Credits,
	}

	if err := uc.CompletePayment(context.Background(), req); err != nil {
		t.Fatalf("first CompletePayment error: %v", err)
	}

	u, err := userRepo.FindByTelegramID(context.Background(), 4002)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	creditsAfterFirst := u.Credits

	if err := uc.CompletePayment(context.Background(), req); err != nil {
		t.Fatalf("second CompletePayment error: %v", err)
	}

	u, err = userRepo.FindByTelegramID(context.Background(), 4002)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	if u.Credits != creditsAfterFirst {
		t.Errorf("credits changed on duplicate payment: before=%d after=%d", creditsAfterFirst, u.Credits)
	}

	expectedCreditTxCount := 2 // 1 bonus + 1 purchase

	if len(creditRepo.txs) != expectedCreditTxCount {
		t.Errorf("expected %d credit transactions, got %d", expectedCreditTxCount, len(creditRepo.txs))
	}
}

func TestCompletePayment_InsufficientCreditsConstraint(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	paymentRepo := newMockPaymentRepo()

	mustRegisterUser(t, userRepo, creditRepo, 4003)

	uc := newPaymentUC(userRepo, creditRepo, paymentRepo)

	err := uc.CompletePayment(context.Background(), usecase.CompletePaymentRequest{
		TelegramUserID:          4003,
		TelegramPaymentChargeID: "charge_ok",
		ProductCode:             payment.Product25Credits,
	})
	if err != nil {
		t.Fatalf("CompletePayment error: %v", err)
	}

	u, err := userRepo.FindByTelegramID(context.Background(), 4003)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	product := payment.ProductCatalog[payment.Product25Credits]

	if u.Credits != user.InitialCredits+product.Credits {
		t.Errorf("expected %d credits, got %d", user.InitialCredits+product.Credits, u.Credits)
	}
}
