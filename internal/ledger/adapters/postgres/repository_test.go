package postgres

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Imanghvs/froggobank/internal/ledger/application"
	"github.com/Imanghvs/froggobank/internal/ledger/domain"
	money "github.com/Imanghvs/froggobank/internal/money/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	return setupTestRepositoryWithMigrations(t, "00001_create_accounts.sql", "00002_create_ledger.sql", "00003_create_account_balances.sql")
}

func setupTestRepositoryWithMigrations(t *testing.T, filenames ...string) (*Repository, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	admin, err := pgxpool.NewWithConfig(t.Context(), config.Copy())
	if err != nil {
		t.Fatalf("create admin pool: %v", err)
	}
	t.Cleanup(admin.Close)
	if err := admin.Ping(t.Context()); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	// Immutable rows cannot be deleted for cleanup. Install the actual migration
	// files in an isolated schema and drop only that test-owned schema afterward.
	schema := "ledger_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(t.Context(), "CREATE SCHEMA "+quoted); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatalf("create test pool: %v", err)
	}
	t.Cleanup(pool.Close)
	for _, filename := range filenames {
		contents, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", filename))
		if err != nil {
			t.Fatalf("read migration %s: %v", filename, err)
		}
		up, _, found := strings.Cut(string(contents), "-- +goose Down")
		if !found {
			t.Fatalf("migration %s has no Down section", filename)
		}
		if _, err := pool.Exec(t.Context(), up); err != nil {
			t.Fatalf("apply migration %s: %v", filename, err)
		}
	}
	return New(pool), pool
}

func createAccount(t *testing.T, pool *pgxpool.Pool, currency money.Currency) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(t.Context(), `INSERT INTO accounts (id, currency, created_at) VALUES ($1, $2, $3)`, id, string(currency), time.Now().UTC()); err != nil {
		t.Fatalf("create account fixture: %v", err)
	}
	return id
}

func makePosting(t *testing.T, accountID uuid.UUID, side domain.Side, units int64, currency money.Currency) domain.Posting {
	t.Helper()
	amount, err := money.New(units, currency)
	if err != nil {
		t.Fatal(err)
	}
	posting, err := domain.NewPosting(accountID, side, amount)
	if err != nil {
		t.Fatal(err)
	}
	return posting
}

func makeTransaction(t *testing.T, postings []domain.Posting) domain.Transaction {
	t.Helper()
	tx, err := domain.NewTransaction(postings)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func assertEntryCounts(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, transactions, postings int64) {
	t.Helper()
	var transactionCount, postingCount int64
	err := pool.QueryRow(t.Context(), `SELECT
		(SELECT COUNT(*) FROM ledger_transactions WHERE id = $1),
		(SELECT COUNT(*) FROM ledger_postings WHERE transaction_id = $1)`, id).Scan(&transactionCount, &postingCount)
	if err != nil {
		t.Fatalf("count ledger rows: %v", err)
	}
	if transactionCount != transactions || postingCount != postings {
		t.Fatalf("expected %d transactions and %d postings, got %d and %d", transactions, postings, transactionCount, postingCount)
	}
}

func assertDatabaseError(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("expected PostgreSQL error %s, got %v", code, err)
	}
}

func TestAccountAndLedgerShareCurrencyEnum(t *testing.T) {
	_, pool := setupTestRepository(t)
	var columns int
	err := pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM pg_attribute
		WHERE attrelid IN ('accounts'::regclass, 'ledger_postings'::regclass)
		AND attname = 'currency' AND atttypid = 'currency_code'::regtype`).Scan(&columns)
	if err != nil {
		t.Fatalf("inspect currency columns: %v", err)
	}
	if columns != 2 {
		t.Fatalf("expected both columns to use currency_code, got %d", columns)
	}
	for _, currency := range []money.Currency{money.CurrencyEUR, money.CurrencyUSD, money.CurrencyGBP} {
		createAccount(t, pool, currency)
	}
	for _, currency := range []string{"JPY", "eur", ""} {
		_, err := pool.Exec(t.Context(), `INSERT INTO accounts (id, currency, created_at) VALUES ($1, $2, $3)`,
			uuid.New(), currency, time.Now().UTC())
		assertDatabaseError(t, err, "22P02")
	}
}

func TestCurrencyMigrationPreservesExistingAccounts(t *testing.T) {
	_, pool := setupTestRepositoryWithMigrations(t, "00001_create_accounts.sql", "00002_create_ledger.sql")
	accounts := make(map[uuid.UUID]money.Currency)
	for _, currency := range []money.Currency{money.CurrencyEUR, money.CurrencyUSD, money.CurrencyGBP} {
		accounts[createAccount(t, pool, currency)] = currency
	}
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "00002_create_ledger.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up, down, found := strings.Cut(string(contents), "-- +goose Down")
	if !found {
		t.Fatal("ledger migration has no Down section")
	}
	if _, err := pool.Exec(t.Context(), down); err != nil {
		t.Fatalf("roll back ledger migration: %v", err)
	}
	// Migrations precede application startup. Discard prepared statements that
	// reference the enum type removed by rollback before testing the old schema.
	pool.Reset()
	var restored bool
	err = pool.QueryRow(t.Context(), `SELECT to_regtype('currency_code') IS NULL AND EXISTS (
		SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema()
		AND table_name = 'accounts' AND column_name = 'currency'
		AND data_type = 'character varying' AND character_maximum_length = 3
	)`).Scan(&restored)
	if err != nil || !restored {
		t.Fatalf("expected original account column type and enum removal, got %t, %v", restored, err)
	}
	_, err = pool.Exec(t.Context(), `INSERT INTO accounts (id, currency, created_at) VALUES ($1, 'eur', $2)`, uuid.New(), time.Now().UTC())
	assertDatabaseError(t, err, "23514")

	// Previously permitted unsupported data must make conversion fail atomically,
	// preserving the original schema and records rather than changing their currency.
	unsupportedID := createAccount(t, pool, "JPY")
	_, err = pool.Exec(t.Context(), up)
	assertDatabaseError(t, err, "22P02")
	var unchanged bool
	err = pool.QueryRow(t.Context(), `SELECT to_regtype('currency_code') IS NULL AND EXISTS (
		SELECT 1 FROM accounts WHERE id = $1 AND currency = 'JPY'
	)`, unsupportedID).Scan(&unchanged)
	if err != nil || !unchanged {
		t.Fatalf("failed migration must leave schema and records unchanged, got %t, %v", unchanged, err)
	}
	if _, err := pool.Exec(t.Context(), `DELETE FROM accounts WHERE id = $1`, unsupportedID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), up); err != nil {
		t.Fatalf("reapply ledger migration: %v", err)
	}
	pool.Reset()
	for id, expected := range accounts {
		var currency string
		if err := pool.QueryRow(t.Context(), `SELECT currency FROM accounts WHERE id = $1`, id).Scan(&currency); err != nil {
			t.Fatalf("read migrated account: %v", err)
		}
		if currency != string(expected) {
			t.Errorf("expected preserved currency %s, got %s", expected, currency)
		}
	}
}

func TestRepositoryPost(t *testing.T) {
	repo, pool := setupTestRepository(t)
	eurDebit := createAccount(t, pool, money.CurrencyEUR)
	eurCredit := createAccount(t, pool, money.CurrencyEUR)
	usdDebit := createAccount(t, pool, money.CurrencyUSD)
	usdCredit := createAccount(t, pool, money.CurrencyUSD)
	postings := []domain.Posting{
		makePosting(t, eurDebit, domain.Debit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, eurDebit, domain.Debit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, eurCredit, domain.Credit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, eurCredit, domain.Credit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, usdDebit, domain.Debit, 12345, money.CurrencyUSD),
		makePosting(t, usdCredit, domain.Credit, 12345, money.CurrencyUSD),
	}
	tx, err := application.New(repo).PostTransaction(t.Context(), postings)
	if err != nil {
		t.Fatalf("post ledger transaction: %v", err)
	}
	assertEntryCounts(t, pool, tx.ID(), 1, int64(len(postings)))
	var createdAt time.Time
	var count int64
	if err := pool.QueryRow(t.Context(), `SELECT created_at, posting_count FROM ledger_transactions WHERE id = $1`, tx.ID()).Scan(&createdAt, &count); err != nil {
		t.Fatalf("read transaction: %v", err)
	}
	if !createdAt.Equal(tx.CreatedAt()) || count != int64(len(postings)) {
		t.Error("persisted transaction metadata differs")
	}
	expected := make(map[uuid.UUID]domain.Posting, len(postings))
	for _, posting := range postings {
		expected[posting.ID()] = posting
	}
	rows, err := pool.Query(t.Context(), `SELECT id, account_id, side, amount_minor, currency FROM ledger_postings WHERE transaction_id = $1`, tx.ID())
	if err != nil {
		t.Fatalf("read postings: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, accountID uuid.UUID
		var side, currency string
		var units int64
		if err := rows.Scan(&id, &accountID, &side, &units, &currency); err != nil {
			t.Fatalf("scan posting: %v", err)
		}
		posting, found := expected[id]
		if !found {
			t.Fatalf("unexpected posting ID %s", id)
		}
		if accountID != posting.AccountID() || side != string(posting.Side()) || units != posting.Amount().MinorUnits() || currency != string(posting.Amount().Currency()) {
			t.Errorf("persisted posting %s differs from domain posting", id)
		}
		delete(expected, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate postings: %v", err)
	}
	if len(expected) != 0 {
		t.Errorf("missing %d persisted postings", len(expected))
	}
}

func TestRepositoryPostWithThreePostings(t *testing.T) {
	repo, pool := setupTestRepository(t)
	postings := []domain.Posting{
		makePosting(t, createAccount(t, pool, money.CurrencyGBP), domain.Debit, 40, money.CurrencyGBP),
		makePosting(t, createAccount(t, pool, money.CurrencyGBP), domain.Debit, 60, money.CurrencyGBP),
		makePosting(t, createAccount(t, pool, money.CurrencyGBP), domain.Credit, 100, money.CurrencyGBP),
	}
	entry := makeTransaction(t, postings)
	if err := repo.Post(t.Context(), entry); err != nil {
		t.Fatalf("post three-posting transaction: %v", err)
	}
	assertEntryCounts(t, pool, entry.ID(), 1, 3)
	for _, posting := range postings {
		var accountID uuid.UUID
		var side, currency string
		var units int64
		err := pool.QueryRow(t.Context(), `SELECT account_id, side, amount_minor, currency
			FROM ledger_postings WHERE transaction_id = $1 AND id = $2`,
			entry.ID(), posting.ID()).Scan(&accountID, &side, &units, &currency)
		if err != nil {
			t.Fatalf("read posting %s: %v", posting.ID(), err)
		}
		if accountID != posting.AccountID() || side != string(posting.Side()) || units != posting.Amount().MinorUnits() || currency != string(posting.Amount().Currency()) {
			t.Errorf("persisted posting %s differs from domain posting", posting.ID())
		}
	}
}

func TestRepositoryPostRollsBackPartialWrites(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		name := "missing account"
		if mismatch {
			name = "account currency mismatch"
		}
		t.Run(name, func(t *testing.T) {
			repo, pool := setupTestRepository(t)
			validAccount := createAccount(t, pool, money.CurrencyEUR)
			invalidAccount := uuid.New()
			if mismatch {
				invalidAccount = createAccount(t, pool, money.CurrencyUSD)
			}
			tx := makeTransaction(t, []domain.Posting{
				makePosting(t, validAccount, domain.Debit, 100, money.CurrencyEUR),
				makePosting(t, invalidAccount, domain.Credit, 100, money.CurrencyEUR),
			})
			err := repo.Post(t.Context(), tx)
			assertDatabaseError(t, err, "23503")
			assertEntryCounts(t, pool, tx.ID(), 0, 0)
		})
	}
}

func TestRepositoryPostRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	err := New(nil).Post(t.Context(), domain.Transaction{})
	if !errors.Is(err, domain.ErrInvalidTransaction) {
		t.Fatalf("expected domain validation error, got %v", err)
	}
}

func TestRepositoryPostCanceledContext(t *testing.T) {
	repo, pool := setupTestRepository(t)
	accountID := createAccount(t, pool, money.CurrencyEUR)
	tx := makeTransaction(t, []domain.Posting{
		makePosting(t, accountID, domain.Debit, 1, money.CurrencyEUR),
		makePosting(t, accountID, domain.Credit, 1, money.CurrencyEUR),
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := repo.Post(ctx, tx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation error, got %v", err)
	}
	assertEntryCounts(t, pool, tx.ID(), 0, 0)
}

func TestLedgerTablesRejectMutation(t *testing.T) {
	repo, pool := setupTestRepository(t)
	accountID := createAccount(t, pool, money.CurrencyEUR)
	postings := []domain.Posting{
		makePosting(t, accountID, domain.Debit, 100, money.CurrencyEUR),
		makePosting(t, accountID, domain.Credit, 100, money.CurrencyEUR),
	}
	tx := makeTransaction(t, postings)
	if err := repo.Post(t.Context(), tx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		query string
		args  []any
	}{
		{"update transaction", `UPDATE ledger_transactions SET posting_count = 4 WHERE id = $1`, []any{tx.ID()}},
		{"delete transaction", `DELETE FROM ledger_transactions WHERE id = $1`, []any{tx.ID()}},
		{"truncate transactions", `TRUNCATE ledger_transactions CASCADE`, nil},
		{"update posting", `UPDATE ledger_postings SET amount_minor = 200 WHERE id = $1`, []any{postings[0].ID()}},
		{"delete posting", `DELETE FROM ledger_postings WHERE id = $1`, []any{postings[0].ID()}},
		{"truncate postings", `TRUNCATE ledger_postings`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(t.Context(), tc.query, tc.args...)
			assertDatabaseError(t, err, "23514")
			if !strings.Contains(err.Error(), "append-only") {
				t.Errorf("expected append-only guard, got %v", err)
			}
			assertEntryCounts(t, pool, tx.ID(), 1, 2)
		})
	}
}

func TestLedgerCannotExtendCommittedTransaction(t *testing.T) {
	repo, pool := setupTestRepository(t)
	accountID := createAccount(t, pool, money.CurrencyEUR)
	entry := makeTransaction(t, []domain.Posting{
		makePosting(t, accountID, domain.Debit, 100, money.CurrencyEUR),
		makePosting(t, accountID, domain.Credit, 100, money.CurrencyEUR),
	})
	if err := repo.Post(t.Context(), entry); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	for _, side := range []domain.Side{domain.Debit, domain.Credit} {
		_, err := tx.Exec(t.Context(), `INSERT INTO ledger_postings (id, transaction_id, account_id, side, amount_minor, currency)
			VALUES ($1, $2, $3, $4, 100, 'EUR')`, uuid.New(), entry.ID(), accountID, string(side))
		if err != nil {
			t.Fatalf("insert tentative posting: %v", err)
		}
	}
	assertDatabaseError(t, tx.Commit(t.Context()), "23514")
	assertEntryCounts(t, pool, entry.ID(), 1, 2)
}

func TestLedgerDatabaseRejectsIncompleteOrUnbalancedEntries(t *testing.T) {
	_, pool := setupTestRepository(t)
	eurAccount := createAccount(t, pool, money.CurrencyEUR)
	usdAccount := createAccount(t, pool, money.CurrencyUSD)
	type rawPosting struct {
		side      string
		units     int64
		currency  string
		accountID uuid.UUID
	}
	debit := rawPosting{"debit", 100, "EUR", eurAccount}
	credit := rawPosting{"credit", 100, "EUR", eurAccount}
	for _, tc := range []struct {
		name     string
		count    int64
		postings []rawPosting
	}{
		{"empty", 2, nil},
		{"single posting", 2, []rawPosting{debit}},
		{"incorrect count", 3, []rawPosting{debit, credit}},
		{"unbalanced", 2, []rawPosting{debit, {"credit", 99, "EUR", eurAccount}}},
		{"cross currency", 2, []rawPosting{debit, {"credit", 100, "USD", usdAccount}}},
		{"overflow cannot hide imbalance", 4, []rawPosting{
			{"debit", math.MaxInt64, "EUR", eurAccount},
			{"debit", math.MaxInt64, "EUR", eurAccount},
			{"debit", 3, "EUR", eurAccount},
			{"credit", 1, "EUR", eurAccount},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(t.Context())
			id := uuid.New()
			_, err = tx.Exec(t.Context(), `INSERT INTO ledger_transactions (id, created_at, posting_count) VALUES ($1, $2, $3)`, id, time.Now().UTC(), tc.count)
			if err != nil {
				t.Fatalf("insert entry: %v", err)
			}
			for _, posting := range tc.postings {
				_, err = tx.Exec(t.Context(), `INSERT INTO ledger_postings (id, transaction_id, account_id, side, amount_minor, currency)
					VALUES ($1, $2, $3, $4, $5, $6)`, uuid.New(), id, posting.accountID, posting.side, posting.units, posting.currency)
				if err != nil {
					t.Fatalf("insert tentative posting: %v", err)
				}
			}
			assertDatabaseError(t, tx.Commit(t.Context()), "23514")
			assertEntryCounts(t, pool, id, 0, 0)
		})
	}
}

func TestLedgerDatabaseRejectsInvalidPostingRows(t *testing.T) {
	_, pool := setupTestRepository(t)
	accountID := createAccount(t, pool, money.CurrencyEUR)
	for _, tc := range []struct {
		name      string
		id        uuid.UUID
		accountID uuid.UUID
		side      string
		units     int64
		currency  string
		code      string
	}{
		{"zero posting ID", uuid.Nil, accountID, "debit", 1, "EUR", "23514"},
		{"zero account ID", uuid.New(), uuid.Nil, "debit", 1, "EUR", "23514"},
		{"invalid side", uuid.New(), accountID, "other", 1, "EUR", "23514"},
		{"zero amount", uuid.New(), accountID, "debit", 0, "EUR", "23514"},
		{"negative amount", uuid.New(), accountID, "credit", -1, "EUR", "23514"},
		{"unsupported currency", uuid.New(), accountID, "debit", 1, "JPY", "22P02"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(t.Context())
			id := uuid.New()
			if _, err := tx.Exec(t.Context(), `INSERT INTO ledger_transactions (id, created_at, posting_count) VALUES ($1, $2, 2)`, id, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(t.Context(), `INSERT INTO ledger_postings (id, transaction_id, account_id, side, amount_minor, currency)
				VALUES ($1, $2, $3, $4, $5, $6)`, tc.id, id, tc.accountID, tc.side, tc.units, tc.currency)
			assertDatabaseError(t, err, tc.code)
			if err := tx.Rollback(t.Context()); err != nil {
				t.Fatalf("rollback invalid row: %v", err)
			}
			assertEntryCounts(t, pool, id, 0, 0)
		})
	}
}
