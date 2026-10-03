-- +goose Up

CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    currency VARCHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT accounts_currency_format
        CHECK (currency ~ '^[A-Z]{3}$')
);

-- +goose Down

DROP TABLE accounts;
