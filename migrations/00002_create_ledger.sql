-- +goose Up

CREATE TYPE currency_code AS ENUM ('EUR', 'USD', 'GBP');

ALTER TABLE accounts DROP CONSTRAINT accounts_currency_format;
ALTER TABLE accounts ALTER COLUMN currency TYPE currency_code USING currency::currency_code;

-- The composite key ensures postings use the referenced account's currency.
ALTER TABLE accounts ADD CONSTRAINT accounts_id_currency_unique UNIQUE (id, currency);

CREATE TABLE ledger_transactions (
    id UUID PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    created_at TIMESTAMPTZ NOT NULL,
    -- Fixing the count prevents later inserts from extending a committed entry.
    posting_count BIGINT NOT NULL CHECK (posting_count >= 2)
);

CREATE TABLE ledger_postings (
    id UUID PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    transaction_id UUID NOT NULL REFERENCES ledger_transactions (id) ON DELETE RESTRICT,
    account_id UUID NOT NULL CHECK (account_id <> '00000000-0000-0000-0000-000000000000'),
    side TEXT NOT NULL CHECK (side IN ('debit', 'credit')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency currency_code NOT NULL,
    FOREIGN KEY (account_id, currency) REFERENCES accounts (id, currency) ON DELETE RESTRICT
);

CREATE INDEX ledger_postings_transaction_id_idx ON ledger_postings (transaction_id);
CREATE INDEX ledger_postings_account_id_idx ON ledger_postings (account_id);

-- +goose StatementBegin
CREATE FUNCTION reject_ledger_mutation() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'ledger entries are append-only' USING ERRCODE = '23514';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ledger_transactions_no_mutation
BEFORE UPDATE OR DELETE OR TRUNCATE ON ledger_transactions
FOR EACH STATEMENT EXECUTE FUNCTION reject_ledger_mutation();

CREATE TRIGGER ledger_postings_no_mutation
BEFORE UPDATE OR DELETE OR TRUNCATE ON ledger_postings
FOR EACH STATEMENT EXECUTE FUNCTION reject_ledger_mutation();

-- +goose StatementBegin
CREATE FUNCTION validate_ledger_transaction() RETURNS TRIGGER
LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
DECLARE
    entry_id UUID;
    expected_count BIGINT;
    actual_count BIGINT;
BEGIN
    IF TG_TABLE_NAME = 'ledger_transactions' THEN
        entry_id := NEW.id;
    ELSE
        entry_id := NEW.transaction_id;
    END IF;

    SELECT posting_count INTO expected_count FROM ledger_transactions WHERE id = entry_id;
    SELECT COUNT(*) INTO actual_count FROM ledger_postings WHERE transaction_id = entry_id;
    IF actual_count <> expected_count THEN
        RAISE EXCEPTION 'ledger posting count does not match transaction'
            USING ERRCODE = '23514';
    END IF;

    -- SUM(bigint) uses exact numeric accumulation, even above int64 totals.
    IF EXISTS (
        SELECT currency FROM ledger_postings WHERE transaction_id = entry_id
        GROUP BY currency
        HAVING SUM(amount_minor) FILTER (WHERE side = 'debit')
            IS DISTINCT FROM SUM(amount_minor) FILTER (WHERE side = 'credit')
    ) THEN
        RAISE EXCEPTION 'ledger transaction is unbalanced per currency'
            USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER ledger_transactions_validate
AFTER INSERT ON ledger_transactions DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_ledger_transaction();

CREATE CONSTRAINT TRIGGER ledger_postings_validate
AFTER INSERT ON ledger_postings DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_ledger_transaction();

-- +goose Down

DROP TABLE ledger_postings;
DROP TABLE ledger_transactions;
DROP FUNCTION validate_ledger_transaction();
DROP FUNCTION reject_ledger_mutation();
ALTER TABLE accounts DROP CONSTRAINT accounts_id_currency_unique;
ALTER TABLE accounts ALTER COLUMN currency TYPE VARCHAR(3) USING currency::text;
ALTER TABLE accounts ADD CONSTRAINT accounts_currency_format CHECK (currency ~ '^[A-Z]{3}$');
DROP TYPE currency_code;
