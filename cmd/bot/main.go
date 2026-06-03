package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/boliev/pixie-bot/internal/application/usecase"
	"github.com/boliev/pixie-bot/internal/config"
	httpdelivery "github.com/boliev/pixie-bot/internal/delivery/http"
	tgdelivery "github.com/boliev/pixie-bot/internal/delivery/telegram"
	openaiinfra "github.com/boliev/pixie-bot/internal/infrastructure/openai"
	pginfra "github.com/boliev/pixie-bot/internal/infrastructure/postgres"
	"github.com/boliev/pixie-bot/internal/infrastructure/postgres/repository"
	tginfra "github.com/boliev/pixie-bot/internal/infrastructure/telegram"
	"github.com/boliev/pixie-bot/internal/logger"
	"github.com/boliev/pixie-bot/internal/platform/db"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	log := logger.New(cfg.AppEnv)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", "err", err)

		return err
	}

	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)

	creditRepo := repository.NewCreditRepository(pool)

	generationRepo := repository.NewGenerationRepository(pool)

	paymentRepo := repository.NewPaymentRepository(pool)

	txManager := pginfra.NewTxManager(pool)

	imageEditor := openaiinfra.NewImageEditor(cfg.OpenAIAPIKey, cfg.OpenAIImageModel)

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		log.Error("failed to create telegram bot", "err", err)

		return err
	}

	log.Info("telegram bot authorised", "username", bot.Self.UserName)

	tgClient := tginfra.NewClient(bot)

	userUC := usecase.NewUserUseCase(userRepo, creditRepo, txManager, log)

	generationUC := usecase.NewGenerationUseCase(userRepo, creditRepo, generationRepo, imageEditor, tgClient, txManager, log)

	paymentUC := usecase.NewPaymentUseCase(userRepo, creditRepo, paymentRepo, txManager, log)

	helpUC := usecase.NewHelpUseCase()

	tgHandler := tgdelivery.NewHandler(log, bot, tgClient, userUC, generationUC, paymentUC, helpUC)

	httpHandler := httpdelivery.NewHandler(pool)

	r := chi.NewRouter()

	r.Get("/healthz", httpHandler.Healthz)

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}

	go func() {
		log.Info("http server starting", "addr", cfg.HTTPAddr)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server error", "err", err)
		}
	}()

	log.Info("bot starting long polling")

	tgHandler.Start(ctx)

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error("http server shutdown error", "err", err)
	}

	log.Info("shutdown complete")

	return nil
}
