-- +goose Up

-- Goose runs this migration in a transaction. Block posting writes while
-- backfilling so every committed posting is counted exactly once.
LOCK TABLE ledger_postings IN SHARE ROW EXCLUSIVE MODE;

ALTER TABLE accounts ADD COLUMN account_type TEXT NOT NULL DEFAULT 'liability'
    CHECK (account_type IN ('asset', 'liability', 'equity', 'revenue', 'expense'));
ALTER TABLE accounts ADD COLUMN enforce_nonnegative_balance BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE account_balances (
    account_id UUID PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    -- Exact, unscaled numeric totals can exceed one posting's BIGINT capacity.
    debits_minor NUMERIC NOT NULL DEFAULT 0
        CHECK (debits_minor >= 0 AND debits_minor < 'Infinity'::numeric
            AND debits_minor = trunc(debits_minor)),
    credits_minor NUMERIC NOT NULL DEFAULT 0
        CHECK (credits_minor >= 0 AND credits_minor < 'Infinity'::numeric
            AND credits_minor = trunc(credits_minor))
);

INSERT INTO account_balances (account_id, debits_minor, credits_minor)
SELECT a.id,
    COALESCE(SUM(p.amount_minor) FILTER (WHERE p.side = 'debit'), 0),
    COALESCE(SUM(p.amount_minor) FILTER (WHERE p.side = 'credit'), 0)
FROM accounts a LEFT JOIN ledger_postings p ON p.account_id = a.id
GROUP BY a.id;

-- +goose StatementBegin
CREATE FUNCTION initialize_account_balance() RETURNS TRIGGER
LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
BEGIN
    INSERT INTO account_balances (account_id) VALUES (NEW.id);
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER accounts_initialize_balance
AFTER INSERT ON accounts FOR EACH ROW
EXECUTE FUNCTION initialize_account_balance();

-- +goose StatementBegin
CREATE FUNCTION update_posted_balances() RETURNS TRIGGER
LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
DECLARE
    delta RECORD;
BEGIN
    -- Aggregate the entire bulk insert before changing balances. Updates lock
    -- each balance row in UUID order, preventing lost updates and reducing
    -- deadlocks when transactions touch the same accounts in opposite orders.
    FOR delta IN
        SELECT account_id,
            COALESCE(SUM(amount_minor) FILTER (WHERE side = 'debit'), 0) AS debits,
            COALESCE(SUM(amount_minor) FILTER (WHERE side = 'credit'), 0) AS credits
        FROM inserted_postings GROUP BY account_id ORDER BY account_id
    LOOP
        UPDATE account_balances
        SET debits_minor = debits_minor + delta.debits,
            credits_minor = credits_minor + delta.credits
        WHERE account_id = delta.account_id;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'account balance projection is missing'
                USING ERRCODE = '23514';
        END IF;
    END LOOP;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ledger_postings_update_balances
AFTER INSERT ON ledger_postings
REFERENCING NEW TABLE AS inserted_postings
FOR EACH STATEMENT EXECUTE FUNCTION update_posted_balances();

-- +goose StatementBegin
CREATE FUNCTION validate_account_balance_limit() RETURNS TRIGGER
LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM accounts a JOIN account_balances b ON b.account_id = a.id
        WHERE a.id = NEW.account_id AND a.enforce_nonnegative_balance
        AND CASE WHEN a.account_type IN ('asset', 'expense')
            THEN b.debits_minor - b.credits_minor
            ELSE b.credits_minor - b.debits_minor END < 0
    ) THEN
        RAISE EXCEPTION 'insufficient posted funds'
            USING ERRCODE = '23514', CONSTRAINT = 'account_balances_nonnegative';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- Validate the final totals at commit, including entries inserted through
-- several SQL statements. Balance row locks remain held throughout the check.
CREATE CONSTRAINT TRIGGER account_balances_validate_limit
AFTER UPDATE ON account_balances DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_account_balance_limit();

-- +goose StatementBegin
CREATE FUNCTION protect_account_balance() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    -- Projections may change through posting/account triggers and FK cleanup,
    -- but cannot be edited independently through ordinary SQL statements.
    IF pg_trigger_depth() < 2 THEN
        RAISE EXCEPTION 'account balances are maintained by ledger postings'
            USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER account_balances_no_direct_mutation
BEFORE INSERT OR UPDATE OR DELETE ON account_balances
FOR EACH ROW EXECUTE FUNCTION protect_account_balance();

CREATE TRIGGER account_balances_no_truncate
BEFORE TRUNCATE ON account_balances
FOR EACH STATEMENT EXECUTE FUNCTION protect_account_balance();

-- +goose StatementBegin
CREATE FUNCTION protect_account_identity() RETURNS TRIGGER
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

-- A single MVCC snapshot compares the projection with authoritative postings.
CREATE VIEW account_balance_reconciliation AS
SELECT a.id AS account_id,
    b.debits_minor AS stored_debits_minor,
    b.credits_minor AS stored_credits_minor,
    COALESCE(SUM(p.amount_minor) FILTER (WHERE p.side = 'debit'), 0) AS ledger_debits_minor,
    COALESCE(SUM(p.amount_minor) FILTER (WHERE p.side = 'credit'), 0) AS ledger_credits_minor,
    (b.account_id IS NOT NULL
        AND b.debits_minor = COALESCE(SUM(p.amount_minor) FILTER (WHERE p.side = 'debit'), 0)
        AND b.credits_minor = COALESCE(SUM(p.amount_minor) FILTER (WHERE p.side = 'credit'), 0)) AS matches
FROM accounts a
LEFT JOIN account_balances b ON b.account_id = a.id
LEFT JOIN ledger_postings p ON p.account_id = a.id
GROUP BY a.id, b.account_id, b.debits_minor, b.credits_minor;

-- +goose Down

DROP VIEW account_balance_reconciliation;
DROP TRIGGER accounts_protect_identity ON accounts;
DROP FUNCTION protect_account_identity();
DROP TRIGGER ledger_postings_update_balances ON ledger_postings;
DROP FUNCTION update_posted_balances();
DROP TRIGGER accounts_initialize_balance ON accounts;
DROP FUNCTION initialize_account_balance();
DROP TABLE account_balances;
DROP FUNCTION protect_account_balance();
DROP FUNCTION validate_account_balance_limit();
ALTER TABLE accounts DROP COLUMN account_type;
ALTER TABLE accounts DROP COLUMN enforce_nonnegative_balance;
