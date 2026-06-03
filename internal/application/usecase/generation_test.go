package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/boliev/pixie-bot/internal/application/usecase"
	"github.com/boliev/pixie-bot/internal/domain/generation"
	"github.com/boliev/pixie-bot/internal/domain/user"
)

// --- additional mocks ---

type mockGenerationRepo struct {
	generations map[int64]*generation.Generation
	nextID      int64
}

func newMockGenerationRepo() *mockGenerationRepo {
	return &mockGenerationRepo{generations: make(map[int64]*generation.Generation)}
}

func (m *mockGenerationRepo) Create(ctx context.Context, g *generation.Generation, images []*generation.GenerationImage) (*generation.Generation, error) {
	m.nextID++

	g.ID = m.nextID

	m.generations[g.ID] = g

	return g, nil
}

func (m *mockGenerationRepo) FindByID(ctx context.Context, id int64) (*generation.Generation, error) {
	g, ok := m.generations[id]

	if !ok {
		return nil, generation.ErrNotFound
	}

	return g, nil
}

func (m *mockGenerationRepo) UpdateStatus(ctx context.Context, id int64, status generation.Status, errMsg string) error {
	g, ok := m.generations[id]

	if !ok {
		return generation.ErrNotFound
	}

	g.Status = status

	g.Error = errMsg

	return nil
}

type mockImageEditor struct {
	result []byte
	err    error
}

func (m *mockImageEditor) EditImages(ctx context.Context, images [][]byte, prompt string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.result, nil
}

type mockFileFetcher struct {
	data []byte
	err  error
}

func (m *mockFileFetcher) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.data, nil
}

func newGenerationUC(userRepo *mockUserRepo, creditRepo *mockCreditRepo, genRepo *mockGenerationRepo, editor *mockImageEditor, fetcher *mockFileFetcher) *usecase.GenerationUseCase {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	return usecase.NewGenerationUseCase(userRepo, creditRepo, genRepo, editor, fetcher, &noopTxManager{}, log)
}

func mustRegisterUser(t *testing.T, userRepo *mockUserRepo, creditRepo *mockCreditRepo, telegramID int64) {
	t.Helper()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	uc := usecase.NewUserUseCase(userRepo, creditRepo, &noopTxManager{}, log)

	_, err := uc.RegisterUser(context.Background(), usecase.RegisterUserRequest{TelegramUserID: telegramID})
	if err != nil {
		t.Fatalf("register user failed: %v", err)
	}
}

// --- tests ---

func TestStartGeneration_OnePhotoDebitsOneCredit(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	genRepo := newMockGenerationRepo()

	editor := &mockImageEditor{result: []byte("img")}

	fetcher := &mockFileFetcher{data: []byte("bytes")}

	mustRegisterUser(t, userRepo, creditRepo, 3001)

	uc := newGenerationUC(userRepo, creditRepo, genRepo, editor, fetcher)

	_, err := uc.StartGeneration(context.Background(), usecase.StartGenerationRequest{
		TelegramUserID: 3001,
		FileIDs:        []string{"file1"},
		Prompt:         "make it cool",
	})
	if err != nil {
		t.Fatalf("StartGeneration error: %v", err)
	}

	u, err := userRepo.FindByTelegramID(context.Background(), 3001)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	if u.Credits != user.InitialCredits-1 {
		t.Errorf("expected %d credits after generation, got %d", user.InitialCredits-1, u.Credits)
	}
}

func TestStartGeneration_TenPhotosDebitsOneCredit(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	genRepo := newMockGenerationRepo()

	editor := &mockImageEditor{result: []byte("img")}

	fetcher := &mockFileFetcher{data: []byte("bytes")}

	mustRegisterUser(t, userRepo, creditRepo, 3002)

	uc := newGenerationUC(userRepo, creditRepo, genRepo, editor, fetcher)

	fileIDs := make([]string, 10)

	for i := range fileIDs {
		fileIDs[i] = "file"
	}

	_, err := uc.StartGeneration(context.Background(), usecase.StartGenerationRequest{
		TelegramUserID: 3002,
		FileIDs:        fileIDs,
		Prompt:         "edit all",
	})
	if err != nil {
		t.Fatalf("StartGeneration error: %v", err)
	}

	u, err := userRepo.FindByTelegramID(context.Background(), 3002)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	if u.Credits != user.InitialCredits-1 {
		t.Errorf("expected %d credits after 10-photo generation, got %d", user.InitialCredits-1, u.Credits)
	}
}

func TestStartGeneration_ZeroPhotosReturnsError(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	genRepo := newMockGenerationRepo()

	editor := &mockImageEditor{result: []byte("img")}

	fetcher := &mockFileFetcher{data: []byte("bytes")}

	mustRegisterUser(t, userRepo, creditRepo, 3003)

	uc := newGenerationUC(userRepo, creditRepo, genRepo, editor, fetcher)

	_, err := uc.StartGeneration(context.Background(), usecase.StartGenerationRequest{
		TelegramUserID: 3003,
		FileIDs:        []string{},
		Prompt:         "edit",
	})

	if !errors.Is(err, generation.ErrNoImages) {
		t.Errorf("expected ErrNoImages, got %v", err)
	}
}

func TestStartGeneration_ElevenPhotosReturnsError(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	genRepo := newMockGenerationRepo()

	editor := &mockImageEditor{result: []byte("img")}

	fetcher := &mockFileFetcher{data: []byte("bytes")}

	mustRegisterUser(t, userRepo, creditRepo, 3004)

	uc := newGenerationUC(userRepo, creditRepo, genRepo, editor, fetcher)

	fileIDs := make([]string, 11)

	for i := range fileIDs {
		fileIDs[i] = "f"
	}

	_, err := uc.StartGeneration(context.Background(), usecase.StartGenerationRequest{
		TelegramUserID: 3004,
		FileIDs:        fileIDs,
		Prompt:         "edit",
	})

	if !errors.Is(err, generation.ErrTooManyImages) {
		t.Errorf("expected ErrTooManyImages, got %v", err)
	}
}

func TestStartGeneration_EmptyPromptReturnsError(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	genRepo := newMockGenerationRepo()

	editor := &mockImageEditor{result: []byte("img")}

	fetcher := &mockFileFetcher{data: []byte("bytes")}

	mustRegisterUser(t, userRepo, creditRepo, 3005)

	uc := newGenerationUC(userRepo, creditRepo, genRepo, editor, fetcher)

	_, err := uc.StartGeneration(context.Background(), usecase.StartGenerationRequest{
		TelegramUserID: 3005,
		FileIDs:        []string{"file1"},
		Prompt:         "",
	})

	if !errors.Is(err, generation.ErrEmptyPrompt) {
		t.Errorf("expected ErrEmptyPrompt, got %v", err)
	}
}

func TestFailGenerationAndRefund_RefundsCredit(t *testing.T) {
	userRepo := newMockUserRepo()

	creditRepo := &mockCreditRepo{}

	genRepo := newMockGenerationRepo()

	editor := &mockImageEditor{result: []byte("img")}

	fetcher := &mockFileFetcher{data: []byte("bytes")}

	mustRegisterUser(t, userRepo, creditRepo, 3006)

	uc := newGenerationUC(userRepo, creditRepo, genRepo, editor, fetcher)

	result, err := uc.StartGeneration(context.Background(), usecase.StartGenerationRequest{
		TelegramUserID: 3006,
		FileIDs:        []string{"file1"},
		Prompt:         "edit",
	})
	if err != nil {
		t.Fatalf("StartGeneration error: %v", err)
	}

	u, err := userRepo.FindByTelegramID(context.Background(), 3006)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	creditsAfterGeneration := u.Credits

	if err = uc.FailGenerationAndRefund(context.Background(), result.GenerationID); err != nil {
		t.Fatalf("FailGenerationAndRefund error: %v", err)
	}

	u, err = userRepo.FindByTelegramID(context.Background(), 3006)
	if err != nil {
		t.Fatalf("FindByTelegramID error: %v", err)
	}

	if u.Credits != creditsAfterGeneration+1 {
		t.Errorf("expected %d credits after refund, got %d", creditsAfterGeneration+1, u.Credits)
	}

	gen, err := genRepo.FindByID(context.Background(), result.GenerationID)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}

	if gen.Status != generation.StatusFailed {
		t.Errorf("expected generation status failed, got %s", gen.Status)
	}
}
