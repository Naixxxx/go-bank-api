CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS accounts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    number TEXT NOT NULL UNIQUE,
    currency TEXT NOT NULL DEFAULT 'RUB' CHECK (currency = 'RUB'),
    balance_kopecks BIGINT NOT NULL DEFAULT 0 CHECK (balance_kopecks >= 0),
    is_blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cards (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    number_encrypted BYTEA NOT NULL,
    expiry_encrypted BYTEA NOT NULL,
    cvv_hash TEXT NOT NULL,
    number_hmac TEXT NOT NULL UNIQUE,
    expiry_hmac TEXT NOT NULL,
    last4 TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    related_account_id BIGINT REFERENCES accounts(id),
    amount_kopecks BIGINT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('deposit','withdraw','transfer_in','transfer_out','credit_issue','credit_payment','credit_penalty','card_payment')),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS credits (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    principal_kopecks BIGINT NOT NULL CHECK (principal_kopecks > 0),
    annual_rate NUMERIC(8,4) NOT NULL,
    months INT NOT NULL CHECK (months > 0),
    annuity_payment_kopecks BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','closed','overdue')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payment_schedules (
    id BIGSERIAL PRIMARY KEY,
    credit_id BIGINT NOT NULL REFERENCES credits(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    due_date DATE NOT NULL,
    payment_kopecks BIGINT NOT NULL CHECK (payment_kopecks > 0),
    principal_part_kopecks BIGINT NOT NULL DEFAULT 0,
    interest_part_kopecks BIGINT NOT NULL DEFAULT 0,
    penalty_kopecks BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','paid','overdue')),
    paid_at TIMESTAMPTZ,
    UNIQUE (credit_id, due_date)
);

CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_cards_user_id ON cards(user_id);
CREATE INDEX IF NOT EXISTS idx_cards_account_id ON cards(account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_account_id_created_at ON transactions(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_credits_user_id ON credits(user_id);
CREATE INDEX IF NOT EXISTS idx_schedules_due_status ON payment_schedules(due_date, status);
