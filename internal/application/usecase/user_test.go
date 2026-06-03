package usecase_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/boliev/pixie-bot/internal/application/usecase"
	"github.com/boliev/pixie-bot/internal/domain/credits"
	"github.com/boliev/pixie-bot/internal/domain/user"
)

// --- mocks ---

type mockUserRepo struct {
	users  map[int64]*user.User
	nextID int64
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[int64]*user.User)}
}

func (m *mockUserRepo) Create(ctx context.Context, u *user.User) (*user.User, error) {
	for _, existing := range m.users {
		if existing.TelegramUserID == u.TelegramUserID {
			return nil, user.ErrAlreadyExists
		}
	}

	m.nextID++

	u.ID = m.nextID

	m.users[u.TelegramUserID] = u

	return u, nil
}

func (m *mockUserRepo) FindByTelegramID(ctx context.Context, telegramUserID int64) (*user.User, error) {
	u, ok := m.users[telegramUserID]

	if !ok {
		return nil, user.ErrNotFound
	}

	return u, nil
}

func (m *mockUserRepo) AddCredits(ctx context.Context, userID int64, delta int) error {
	for _, u := range m.users {
		if u.ID == userID {
			if u.Credits+delta < 0 {
				return user.ErrInsufficientCredits
			}

			u.Credits += delta

			return nil
		}
	}

	return user.ErrNotFound
}

type mockCreditRepo struct {
	txs []*credits.CreditTransaction
}

func (m *mockCreditRepo) Create(ctx context.Context, tx *credits.CreditTransaction) (*credits.CreditTransaction, error) {
	m.txs = append(m.txs, tx)

	return tx, nil
}

func (m *mockCreditRepo) FindByExternalID(ctx context.Context, externalID string) (*credits.CreditTransaction, error) {
	for _, tx := range m.txs {
		if tx.ExternalID == externalID {
			return tx, nil
		}
	}

	return nil, nil
}

type noopTxManager struct{}

func (m *noopTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newUserUC(userRepo *mockUserRepo, creditRepo *mockCreditRepo) *usecase.UserUseCase {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	return usecase.NewUserUseCase(userRepo, creditRepo, &noopTxManager{}, log)
}

// --- tests ---

func TestRegisterUser_CreatesUserWithInitialCredits(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	uc := newUserUC(userRepo, creditRepo)

	u, err := uc.RegisterUser(context.Background(), usecase.RegisterUserRequest{
		TelegramUserID: 1001,
		FirstName:      "Alice",
	})
	if err != nil {
		t.Fatalf("RegisterUser error: %v", err)
	}

	if u.Credits != user.InitialCredits {
		t.Errorf("expected %d credits, got %d", user.InitialCredits, u.Credits)
	}

	if u.TelegramUserID != 1001 {
		t.Errorf("expected TelegramUserID=1001, got %d", u.TelegramUserID)
	}

	if len(creditRepo.txs) != 1 || creditRepo.txs[0].Type != credits.TypeBonus {
		t.Errorf("expected one bonus credit transaction")
	}
}

func TestRegisterUser_IdempotentForExistingUser(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	uc := newUserUC(userRepo, creditRepo)

	req := usecase.RegisterUserRequest{TelegramUserID: 1001, FirstName: "Alice"}

	u1, err := uc.RegisterUser(context.Background(), req)
	if err != nil {
		t.Fatalf("first RegisterUser error: %v", err)
	}

	u2, err := uc.RegisterUser(context.Background(), req)
	if err != nil {
		t.Fatalf("second RegisterUser error: %v", err)
	}

	if u1.ID != u2.ID {
		t.Errorf("expected same user ID, got %d and %d", u1.ID, u2.ID)
	}

	if len(creditRepo.txs) != 1 {
		t.Errorf("expected 1 credit transaction total, got %d", len(creditRepo.txs))
	}

	// Verify credits were not double-granted.
	stored, err := userRepo.FindByTelegramID(context.Background(), 1001)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	if stored.Credits != user.InitialCredits {
		t.Errorf("expected %d credits after double register, got %d", user.InitialCredits, stored.Credits)
	}
}

func TestGetBalance_ReturnsCurrentCredits(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	uc := newUserUC(userRepo, creditRepo)

	mustRegisterUser(t, userRepo, creditRepo, 2001)

	balance, err := uc.GetBalance(context.Background(), 2001)
	if err != nil {
		t.Fatalf("GetBalance error: %v", err)
	}

	if balance != user.InitialCredits {
		t.Errorf("expected balance %d, got %d", user.InitialCredits, balance)
	}
}
