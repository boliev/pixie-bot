# pixie-bot

Telegram-бот для AI-редактирования фотографий с монетизацией через Telegram Stars.

## Возможности

- Редактирование фото через OpenAI Images API (gpt-image-1)
- Поддержка до 10 фото за один запрос (album/media group)
- 3 бесплатных кредита для новых пользователей
- Пополнение баланса через Telegram Stars
- Идемпотентная обработка платежей
- Long polling (без webhook)

## Архитектура

```
cmd/bot/              — точка входа, wire зависимостей
internal/
  domain/             — сущности, ошибки (нет внешних зависимостей)
  application/        — use cases, port interfaces
  infrastructure/     — реализации: postgres, openai, telegram
  delivery/           — Telegram update handlers, HTTP /healthz
  config/ logger/ platform/
migrations/           — goose SQL-миграции
```

## Требования

- Go 1.22+
- PostgreSQL 16
- Docker Compose (для локальной БД)
- [goose](https://github.com/pressly/goose) для миграций: `go install github.com/pressly/goose/v3/cmd/goose@latest`

## Env-переменные

| Переменная | Обязательная | Описание |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | ✅ | Токен бота от @BotFather |
| `OPENAI_API_KEY` | ✅ | API-ключ OpenAI |
| `OPENAI_IMAGE_MODEL` | — | Модель (по умолчанию `gpt-image-1`) |
| `DATABASE_URL` | ✅ | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `APP_ENV` | — | `development` (text logs) или `production` (JSON logs) |
| `HTTP_ADDR` | — | Адрес HTTP-сервера (по умолчанию `:8080`) |

## Локальный запуск

```bash
# 1. Запустить PostgreSQL
docker compose up -d

# 2. Зависимости
go mod tidy

# 3. Миграции
export DATABASE_URL="postgres://pixie:pixie@localhost:5432/pixie_bot?sslmode=disable"
make migrate-up

# 4. Запуск бота
export TELEGRAM_BOT_TOKEN="your_token"
export OPENAI_API_KEY="your_key"
make run
```

## Как создать Telegram-бота

1. Напишите @BotFather в Telegram.
2. Отправьте `/newbot` и следуйте инструкциям.
3. Скопируйте токен в `TELEGRAM_BOT_TOKEN`.
4. Для тестирования платежей отправьте `/mybots` → выберите бота → **Payments** → подключите тестовый провайдер.

## Long polling

Бот использует исключительно long polling — ни webhook, ни публичный HTTPS не нужны.
Один экземпляр запускается как systemd-сервис на VPS.

> ⚠️ **Масштабирование:** in-memory media group collector работает только на одном инстансе.
> При горизонтальном масштабировании замените его на Redis-коллектор.

## Тестирование Telegram Stars

1. В @BotFather: `/mybots` → ваш бот → **Payments** → **Telegram Stars (Test Mode)**.
2. В тестовом режиме Stars списываются условно — реальных средств не тратится.
3. Отправьте боту «Пополнить», выберите пакет, завершите оплату.

## OpenAI API

Используется endpoint `POST /v1/images/edits` с multipart/form-data.

- Для `gpt-image-1`: поддержка нескольких изображений через поле `image[]`.
- Для `dall-e-2`: только одно изображение (`image`).
- Таймаут запроса: 120 секунд.

> **Важно:** проверьте актуальную документацию OpenAI перед деплоем — синтаксис `image[]`
> для множественных изображений зависит от конкретной модели.

## Деплой как systemd-сервис

```ini
# /etc/systemd/system/pixie-bot.service
[Unit]
Description=Pixie Bot
After=network.target

[Service]
Type=simple
User=pixie
WorkingDirectory=/opt/pixie-bot
EnvironmentFile=/opt/pixie-bot/.env
ExecStart=/opt/pixie-bot/pixie-bot
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# Сборка и деплой
go build -o pixie-bot ./cmd/bot/...
scp pixie-bot user@server:/opt/pixie-bot/

systemctl daemon-reload
systemctl enable pixie-bot
systemctl start pixie-bot
```

## Команды

```bash
make run          # запустить бота
make test         # юнит-тесты
make migrate-up   # применить миграции (нужен DATABASE_URL)
make migrate-down # откатить последнюю миграцию
make lint         # gofmt + go vet
make tidy         # go mod tidy
```

## Healthcheck

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```