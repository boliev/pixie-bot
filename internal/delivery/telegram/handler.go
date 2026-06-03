package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/boliev/pixie-bot/internal/application/usecase"
	"github.com/boliev/pixie-bot/internal/domain/generation"
	"github.com/boliev/pixie-bot/internal/domain/payment"
	tginfra "github.com/boliev/pixie-bot/internal/infrastructure/telegram"
)

type Handler struct {
	log            *slog.Logger
	bot            *tgbotapi.BotAPI
	tgClient       *tginfra.Client
	userUC         *usecase.UserUseCase
	generationUC   *usecase.GenerationUseCase
	paymentUC      *usecase.PaymentUseCase
	helpUC         *usecase.HelpUseCase
	mediaCollector *MediaGroupCollector
}

func NewHandler(
	log *slog.Logger,
	bot *tgbotapi.BotAPI,
	tgClient *tginfra.Client,
	userUC *usecase.UserUseCase,
	generationUC *usecase.GenerationUseCase,
	paymentUC *usecase.PaymentUseCase,
	helpUC *usecase.HelpUseCase,
) *Handler {
	return &Handler{
		log:            log,
		bot:            bot,
		tgClient:       tgClient,
		userUC:         userUC,
		generationUC:   generationUC,
		paymentUC:      paymentUC,
		helpUC:         helpUC,
		mediaCollector: NewMediaGroupCollector(),
	}
}

// Start begins long polling and blocks until ctx is cancelled.
func (h *Handler) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)

	u.Timeout = 60

	updates := h.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			h.bot.StopReceivingUpdates()

			return
		case update, ok := <-updates:
			if !ok {
				return
			}

			go h.handleUpdate(ctx, update)
		}
	}
}

func (h *Handler) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	switch {
	case update.Message != nil:
		h.handleMessage(ctx, update.Message)
	case update.CallbackQuery != nil:
		h.handleCallbackQuery(ctx, update.CallbackQuery)
	case update.PreCheckoutQuery != nil:
		h.handlePreCheckoutQuery(ctx, update.PreCheckoutQuery)
	}
}

func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg.SuccessfulPayment != nil {
		h.handleSuccessfulPayment(ctx, msg)

		return
	}

	if len(msg.Photo) > 0 {
		h.handlePhoto(ctx, msg)

		return
	}

	if msg.IsCommand() {
		h.handleCommand(ctx, msg)

		return
	}

	h.handleText(ctx, msg)
}

func (h *Handler) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		h.handleStart(ctx, msg)
	case "balance":
		h.handleBalance(ctx, msg)
	case "help":
		h.handleHelp(ctx, msg)
	default:
		h.sendText(ctx, msg.Chat.ID, "Неизвестная команда. Используйте /start, /balance или /help.")
	}
}

func (h *Handler) handleText(ctx context.Context, msg *tgbotapi.Message) {
	switch msg.Text {
	case "Баланс":
		h.handleBalance(ctx, msg)
	case "Пополнить":
		h.handleTopUp(ctx, msg)
	case "Помощь":
		h.handleHelp(ctx, msg)
	default:
		h.sendText(ctx, msg.Chat.ID,
			"Отправьте фото с описанием (caption) для редактирования изображения.\n"+
				"Используйте кнопки меню или /help для справки.")
	}
}

func (h *Handler) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	u, err := h.userUC.RegisterUser(ctx, usecase.RegisterUserRequest{
		TelegramUserID: msg.From.ID,
		Username:       msg.From.UserName,
		FirstName:      msg.From.FirstName,
		LastName:       msg.From.LastName,
	})
	if err != nil {
		h.log.Error("register user failed", "err", err, "tgUserID", msg.From.ID)

		h.sendText(ctx, msg.Chat.ID, "Произошла ошибка при регистрации. Попробуйте позже.")

		return
	}

	name := msg.From.FirstName

	if name == "" {
		name = msg.From.UserName
	}

	text := fmt.Sprintf(
		"Привет, %s!\n\nЯ помогу отредактировать ваши фото с помощью AI.\n\n"+
			"Ваш баланс: %d кредитов.\n"+
			"Одна генерация — 1 кредит (до 10 фото за раз).\n\n"+
			"Отправьте фото с описанием, чтобы начать!",
		name, u.Credits,
	)

	h.sendTextWithKeyboard(ctx, msg.Chat.ID, text, mainKeyboard())
}

func (h *Handler) handleBalance(ctx context.Context, msg *tgbotapi.Message) {
	balance, err := h.userUC.GetBalance(ctx, msg.From.ID)
	if err != nil {
		h.log.Error("get balance failed", "err", err)

		h.sendText(ctx, msg.Chat.ID, "Не удалось получить баланс. Попробуйте позже.")

		return
	}

	h.sendText(ctx, msg.Chat.ID, fmt.Sprintf("Ваш баланс: %d кредитов.", balance))
}

func (h *Handler) handleHelp(ctx context.Context, msg *tgbotapi.Message) {
	h.sendText(ctx, msg.Chat.ID, h.helpUC.GetHelpMessage())
}

func (h *Handler) handleTopUp(ctx context.Context, msg *tgbotapi.Message) {
	h.sendTextWithKeyboard(
		ctx, msg.Chat.ID,
		"Выберите пакет кредитов для пополнения:",
		topUpKeyboard(),
	)
}

func (h *Handler) handlePhoto(ctx context.Context, msg *tgbotapi.Message) {
	fileID := bestPhotoFileID(msg.Photo)

	if fileID == "" {
		return
	}

	if msg.MediaGroupID != "" {
		h.mediaCollector.Add(
			msg.Chat.ID, msg.From.ID,
			msg.MediaGroupID, fileID, msg.Caption,
			func(chatID, userID int64, fileIDs []string, prompt string) {
				h.processPhotos(ctx, chatID, userID, fileIDs, prompt)
			},
		)

		return
	}

	h.processPhotos(ctx, msg.Chat.ID, msg.From.ID, []string{fileID}, msg.Caption)
}

func (h *Handler) processPhotos(ctx context.Context, chatID, telegramUserID int64, fileIDs []string, prompt string) {
	if prompt == "" {
		h.sendText(ctx, chatID,
			"Пожалуйста, добавьте текстовое описание к фото (caption).\n"+
				"Что нужно сделать с изображением?")

		return
	}

	if len(fileIDs) > generation.MaxImages {
		h.sendText(ctx, chatID,
			fmt.Sprintf("За один раз можно обработать максимум %d фото. Пожалуйста, отправьте не более %d фото.",
				generation.MaxImages, generation.MaxImages))

		return
	}

	h.sendText(ctx, chatID, "Обрабатываю ваш запрос... ⏳")

	result, err := h.generationUC.StartGeneration(ctx, usecase.StartGenerationRequest{
		TelegramUserID: telegramUserID,
		FileIDs:        fileIDs,
		Prompt:         prompt,
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInsufficientCredits):
			h.sendTextWithKeyboard(ctx, chatID,
				"У вас недостаточно кредитов.\nНажмите «Пополнить», чтобы пополнить баланс.",
				topUpKeyboard())
		case errors.Is(err, generation.ErrEmptyPrompt):
			h.sendText(ctx, chatID, "Пожалуйста, добавьте описание к фото.")
		case errors.Is(err, generation.ErrTooManyImages):
			h.sendText(ctx, chatID, fmt.Sprintf("Максимум %d фото за один запрос.", generation.MaxImages))
		default:
			h.log.Error("start generation failed", "err", err, "chatID", chatID)

			h.sendText(ctx, chatID, "Произошла ошибка при обработке. Попробуйте ещё раз.")
		}

		return
	}

	if err = h.tgClient.SendPhoto(ctx, chatID, result.ImageData); err != nil {
		h.log.Error("send result photo failed", "err", err, "chatID", chatID)

		if refundErr := h.generationUC.FailGenerationAndRefund(ctx, result.GenerationID); refundErr != nil {
			h.log.Error("fail generation and refund failed", "err", refundErr)
		}

		h.sendText(ctx, chatID, "Не удалось отправить результат. Кредит возвращён.")

		return
	}

	if err = h.generationUC.CompleteGeneration(ctx, result.GenerationID); err != nil {
		h.log.Error("complete generation failed", "err", err)
	}
}

func (h *Handler) handleCallbackQuery(ctx context.Context, q *tgbotapi.CallbackQuery) {
	// Always acknowledge the callback immediately.
	if _, err := h.bot.Request(tgbotapi.NewCallback(q.ID, "")); err != nil {
		h.log.Error("answer callback query failed", "err", err)
	}

	if code, ok := strings.CutPrefix(q.Data, "buy:"); ok {
		productCode := payment.ProductCode(code)

		chatID := q.Message.Chat.ID

		h.sendInvoice(ctx, chatID, productCode)
	}
}

func (h *Handler) handlePreCheckoutQuery(_ context.Context, q *tgbotapi.PreCheckoutQuery) {
	// Must answer within 10 seconds — answer immediately before any DB work.
	pca := tgbotapi.PreCheckoutConfig{
		PreCheckoutQueryID: q.ID,
		OK:                 true,
	}

	if _, err := h.bot.Request(pca); err != nil {
		h.log.Error("answer pre_checkout_query failed", "err", err)
	}
}

func (h *Handler) handleSuccessfulPayment(ctx context.Context, msg *tgbotapi.Message) {
	sp := msg.SuccessfulPayment

	req := usecase.CompletePaymentRequest{
		TelegramUserID:          msg.From.ID,
		TelegramPaymentChargeID: sp.TelegramPaymentChargeID,
		ProviderPaymentChargeID: sp.ProviderPaymentChargeID,
		ProductCode:             payment.ProductCode(sp.InvoicePayload),
	}

	if err := h.paymentUC.CompletePayment(ctx, req); err != nil {
		h.log.Error("complete payment failed", "err", err, "chargeID", sp.TelegramPaymentChargeID)

		h.sendText(ctx, msg.Chat.ID, "Произошла ошибка при обработке платежа. Обратитесь в поддержку.")

		return
	}

	balance, err := h.userUC.GetBalance(ctx, msg.From.ID)
	if err != nil {
		h.log.Error("get balance after payment failed", "err", err)

		h.sendText(ctx, msg.Chat.ID, "Оплата прошла успешно!")

		return
	}

	h.sendText(ctx, msg.Chat.ID, fmt.Sprintf("Оплата прошла успешно!\nВаш баланс: %d кредитов.", balance))
}

func (h *Handler) sendInvoice(_ context.Context, chatID int64, productCode payment.ProductCode) {
	product, ok := payment.ProductCatalog[productCode]

	if !ok {
		h.log.Error("unknown product code", "code", productCode)

		return
	}

	invoice := tgbotapi.NewInvoice(
		chatID,
		product.Title,
		fmt.Sprintf("Пополнение баланса на %d кредитов для AI-редактирования фото.", product.Credits),
		string(productCode), // payload = product code; used in successful_payment handler
		"",                  // provider_token empty for Telegram Stars (XTR)
		"",                  // startParameter (not used)
		"XTR",
		[]tgbotapi.LabeledPrice{
			{Label: product.Title, Amount: product.AmountStars},
		},
	)

	if _, err := h.bot.Send(invoice); err != nil {
		h.log.Error("send invoice failed", "err", err, "chatID", chatID)
	}
}

func (h *Handler) sendText(_ context.Context, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)

	if _, err := h.bot.Send(msg); err != nil {
		h.log.Error("send message failed", "err", err, "chatID", chatID)
	}
}

func (h *Handler) sendTextWithKeyboard(_ context.Context, chatID int64, text string, keyboard any) {
	msg := tgbotapi.NewMessage(chatID, text)

	msg.ReplyMarkup = keyboard

	if _, err := h.bot.Send(msg); err != nil {
		h.log.Error("send message with keyboard failed", "err", err, "chatID", chatID)
	}
}

func mainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Баланс"),
			tgbotapi.NewKeyboardButton("Пополнить"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Помощь"),
		),
	)

	kb.ResizeKeyboard = true

	return kb
}

func topUpKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("5 кредитов — 50 ⭐", "buy:credits_5"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("10 кредитов — 90 ⭐", "buy:credits_10"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("25 кредитов — 200 ⭐", "buy:credits_25"),
		),
	)
}

func bestPhotoFileID(photos []tgbotapi.PhotoSize) string {
	if len(photos) == 0 {
		return ""
	}

	return photos[len(photos)-1].FileID
}
