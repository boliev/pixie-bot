-- +goose Up

CREATE TABLE IF NOT EXISTS users (
    id               BIGSERIAL PRIMARY KEY,
    telegram_user_id BIGINT    NOT NULL UNIQUE,
    username         TEXT,
    first_name       TEXT,
    last_name        TEXT,
    credits          INT       NOT NULL DEFAULT 0 CHECK (credits >= 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS credit_transactions (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    type        TEXT   NOT NULL CHECK (type IN ('bonus', 'debit', 'refund', 'purchase')),
    amount      INT    NOT NULL,
    reason      TEXT,
    external_id TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_credit_transactions_user_id    ON credit_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_credit_transactions_external_id ON credit_transactions(external_id)
    WHERE external_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS generations (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    prompt       TEXT   NOT NULL,
    status       TEXT   NOT NULL CHECK (status IN ('processing', 'done', 'failed')),
    images_count INT    NOT NULL CHECK (images_count BETWEEN 1 AND 10),
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_generations_user_id ON generations(user_id);

CREATE TABLE IF NOT EXISTS generation_images (
    id              BIGSERIAL PRIMARY KEY,
    generation_id   BIGINT NOT NULL REFERENCES generations(id) ON DELETE CASCADE,
    telegram_file_id TEXT   NOT NULL,
    position        INT    NOT NULL CHECK (position >= 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (generation_id, position)
);

CREATE TABLE IF NOT EXISTS payments (
    id                          BIGSERIAL PRIMARY KEY,
    user_id                     BIGINT NOT NULL REFERENCES users(id),
    telegram_payment_charge_id  TEXT UNIQUE,
    provider_payment_charge_id  TEXT,
    product_code                TEXT   NOT NULL,
    credits                     INT    NOT NULL,
    amount_stars                INT    NOT NULL,
    status                      TEXT   NOT NULL CHECK (status IN ('pending', 'paid', 'failed')),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);

-- +goose Down

DROP TABLE IF EXISTS generation_images;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS generations;
DROP TABLE IF EXISTS credit_transactions;
DROP TABLE IF EXISTS users;
