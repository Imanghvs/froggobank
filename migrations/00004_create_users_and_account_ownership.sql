-- +goose Up

CREATE TABLE users (
    id UUID PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    issuer TEXT NOT NULL CHECK (length(issuer) BETWEEN 1 AND 2048 AND issuer = btrim(issuer)),
    subject TEXT NOT NULL CHECK (length(subject) BETWEEN 1 AND 255 AND btrim(subject) <> ''),
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (issuer, subject)
);

-- +goose StatementBegin
CREATE FUNCTION protect_user_identity() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id
        OR NEW.issuer IS DISTINCT FROM OLD.issuer
        OR NEW.subject IS DISTINCT FROM OLD.subject THEN
        RAISE EXCEPTION 'local user identity is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER users_protect_identity
BEFORE UPDATE OF id, issuer, subject ON users
FOR EACH ROW EXECUTE FUNCTION protect_user_identity();

-- NULL identifies legacy/internal ledger accounts. No existing account is
-- assigned to a customer automatically.
ALTER TABLE accounts ADD COLUMN owner_id UUID REFERENCES users (id) ON DELETE RESTRICT;
ALTER TABLE accounts ADD CONSTRAINT accounts_customer_policy
    CHECK (owner_id IS NULL OR (account_type = 'liability' AND enforce_nonnegative_balance));
CREATE INDEX accounts_owner_id_idx ON accounts (owner_id) WHERE owner_id IS NOT NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION protect_account_identity() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id
        OR NEW.currency IS DISTINCT FROM OLD.currency
        OR NEW.account_type IS DISTINCT FROM OLD.account_type
        OR NEW.enforce_nonnegative_balance IS DISTINCT FROM OLD.enforce_nonnegative_balance
        OR (OLD.owner_id IS NOT NULL AND NEW.owner_id IS DISTINCT FROM OLD.owner_id) THEN
        RAISE EXCEPTION 'account identity, currency, accounting type, balance policy and assigned owner are immutable'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

DROP TRIGGER accounts_protect_identity ON accounts;
CREATE TRIGGER accounts_protect_identity
BEFORE UPDATE OF id, currency, account_type, enforce_nonnegative_balance, owner_id ON accounts
FOR EACH ROW EXECUTE FUNCTION protect_account_identity();

-- +goose Down

DROP TRIGGER accounts_protect_identity ON accounts;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION protect_account_identity() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id
        OR NEW.currency IS DISTINCT FROM OLD.currency
        OR NEW.account_type IS DISTINCT FROM OLD.account_type
        OR NEW.enforce_nonnegative_balance IS DISTINCT FROM OLD.enforce_nonnegative_balance THEN
        RAISE EXCEPTION 'account identity, currency, accounting type and balance policy are immutable'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER accounts_protect_identity
BEFORE UPDATE OF id, currency, account_type, enforce_nonnegative_balance ON accounts
FOR EACH ROW EXECUTE FUNCTION protect_account_identity();
ALTER TABLE accounts DROP COLUMN owner_id;
DROP TABLE users;
DROP FUNCTION protect_user_identity();
