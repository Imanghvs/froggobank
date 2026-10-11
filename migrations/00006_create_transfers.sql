-- +goose Up

CREATE TABLE transfers (
    id UUID PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    source UUID NOT NULL CHECK (source <> '00000000-0000-0000-0000-000000000000'),
    destination UUID NOT NULL CHECK (destination <> '00000000-0000-0000-0000-000000000000'),
    currency currency_code NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT transfers_distinct_accounts CHECK (source <> destination),
    CONSTRAINT transfers_source_currency_fk
        FOREIGN KEY (source, currency) REFERENCES accounts (id, currency) ON DELETE RESTRICT,
    CONSTRAINT transfers_destination_currency_fk
        FOREIGN KEY (destination, currency) REFERENCES accounts (id, currency) ON DELETE RESTRICT
);

CREATE INDEX transfers_source_idx ON transfers (source);
CREATE INDEX transfers_destination_idx ON transfers (destination);

-- +goose Down

DROP TABLE transfers;
