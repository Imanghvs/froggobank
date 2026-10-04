package postgres

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	accountpostgres "github.com/Imanghvs/froggobank/internal/account/adapters/postgres"
	account "github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/Imanghvs/froggobank/internal/ledger/domain"
	money "github.com/Imanghvs/froggobank/internal/money/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func assertBalance(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, debits, credits, posted string) {
	t.Helper()
	b, err := accountpostgres.New(pool).GetBalance(t.Context(), id)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	if b.DebitsMinor().String() != debits || b.CreditsMinor().String() != credits || b.PostedMinor().String() != posted {
		t.Fatalf("expected debits=%s credits=%s posted=%s, got %s %s %s", debits, credits, posted, b.DebitsMinor(), b.CreditsMinor(), b.PostedMinor())
	}
}

func assertReconciled(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var mismatches int
	if err := pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM account_balance_reconciliation WHERE NOT matches`).Scan(&mismatches); err != nil {
		t.Fatal(err)
	}
	if mismatches != 0 {
		t.Fatalf("found %d balance mismatches", mismatches)
	}
}

func TestPostedBalanceByAccountingType(t *testing.T) {
	repo, pool := setupTestRepository(t)
	accounts := accountpostgres.New(pool)
	for _, accountType := range []account.AccountType{account.Asset, account.Liability, account.Equity, account.Revenue, account.Expense} {
		t.Run(string(accountType), func(t *testing.T) {
			acc, err := account.NewWithType(account.CurrencyEUR, accountType)
			if err != nil {
				t.Fatal(err)
			}
			acc.EnforceNonnegativeBalance = true
			if err := accounts.Create(t.Context(), acc); err != nil {
				t.Fatal(err)
			}
			got, err := accounts.GetByID(t.Context(), acc.ID)
			if err != nil || got.Type != accountType || !got.EnforceNonnegativeBalance {
				t.Fatalf("read accounting type and policy: %+v, %v", got, err)
			}
			assertBalance(t, pool, acc.ID, "0", "0", "0")
			counterparty := createAccount(t, pool, money.CurrencyEUR)
			debit, credit := counterparty, acc.ID
			debits, credits := "0", "1234"
			side, _ := accountType.NormalSide()
			if side == account.DebitNormal {
				debit, credit = acc.ID, counterparty
				debits, credits = "1234", "0"
			}
			entry := makeTransaction(t, []domain.Posting{
				makePosting(t, debit, domain.Debit, 1234, money.CurrencyEUR),
				makePosting(t, credit, domain.Credit, 1234, money.CurrencyEUR),
			})
			if err := repo.Post(t.Context(), entry); err != nil {
				t.Fatal(err)
			}
			assertBalance(t, pool, acc.ID, debits, credits, "1234")
		})
	}
	if _, err := accounts.GetBalance(t.Context(), uuid.New()); !errors.Is(err, account.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing account must return domain not-found, got %v", err)
	}
	assertReconciled(t, pool)
}

func TestPostedBalancesExceedInt64Exactly(t *testing.T) {
	repo, pool := setupTestRepository(t)
	debit := createAccount(t, pool, money.CurrencyEUR)
	credit := createAccount(t, pool, money.CurrencyEUR)
	entry := makeTransaction(t, []domain.Posting{
		makePosting(t, debit, domain.Debit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, credit, domain.Credit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, debit, domain.Debit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, credit, domain.Credit, math.MaxInt64, money.CurrencyEUR),
	})
	if err := repo.Post(t.Context(), entry); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, pool, debit, "18446744073709551614", "0", "-18446744073709551614")
	assertBalance(t, pool, credit, "0", "18446744073709551614", "18446744073709551614")
	assertReconciled(t, pool)
}

func TestPostedBalancesRollbackOnCommitFailure(t *testing.T) {
	_, pool := setupTestRepository(t)
	debit := createAccount(t, pool, money.CurrencyEUR)
	credit := createAccount(t, pool, money.CurrencyEUR)
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	entryID := uuid.New()
	if _, err := tx.Exec(t.Context(), `INSERT INTO ledger_transactions (id, created_at, posting_count) VALUES ($1, now(), 2)`, entryID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), `INSERT INTO ledger_postings (id, transaction_id, account_id, side, amount_minor, currency)
		VALUES ($1, $2, $3, 'debit', 100, 'EUR'), ($4, $2, $5, 'credit', 99, 'EUR')`, uuid.New(), entryID, debit, uuid.New(), credit); err != nil {
		t.Fatal(err)
	}
	var provisional string
	if err := tx.QueryRow(t.Context(), `SELECT debits_minor::text FROM account_balances WHERE account_id=$1`, debit).Scan(&provisional); err != nil || provisional != "100" {
		t.Fatalf("projection should update within posting transaction: %s %v", provisional, err)
	}
	// Other connections cannot observe the uncommitted ledger or projection.
	assertBalance(t, pool, debit, "0", "0", "0")
	assertDatabaseError(t, tx.Commit(t.Context()), "23514")
	assertEntryCounts(t, pool, entryID, 0, 0)
	assertBalance(t, pool, debit, "0", "0", "0")
	assertBalance(t, pool, credit, "0", "0", "0")
	assertReconciled(t, pool)
}

func TestConcurrentPostingDoesNotLoseBalanceUpdates(t *testing.T) {
	repo, pool := setupTestRepository(t)
	a := createAccount(t, pool, money.CurrencyGBP)
	b := createAccount(t, pool, money.CurrencyGBP)
	const count = 32
	entries := make([]domain.Transaction, count)
	for i := range entries {
		debit, credit := a, b
		if i%2 == 1 {
			debit, credit = b, a
		}
		entries[i] = makeTransaction(t, []domain.Posting{
			makePosting(t, debit, domain.Debit, 100, money.CurrencyGBP),
			makePosting(t, credit, domain.Credit, 100, money.CurrencyGBP),
		})
	}
	start := make(chan struct{})
	results := make(chan error, count)
	for _, entry := range entries {
		go func() {
			<-start
			results <- repo.Post(t.Context(), entry)
		}()
	}
	close(start)
	for range count {
		if err := <-results; err != nil {
			t.Errorf("concurrent posting: %v", err)
		}
	}
	assertBalance(t, pool, a, "1600", "1600", "0")
	assertBalance(t, pool, b, "1600", "1600", "0")
	assertReconciled(t, pool)
}

func TestConcurrentSpendingCannotOverdrawRestrictedAccount(t *testing.T) {
	repo, pool := setupTestRepository(t)
	acc := account.New(account.CurrencyEUR)
	acc.EnforceNonnegativeBalance = true
	if err := accountpostgres.New(pool).Create(t.Context(), acc); err != nil {
		t.Fatal(err)
	}
	clearing := createAccount(t, pool, money.CurrencyEUR)
	funding := makeTransaction(t, []domain.Posting{
		makePosting(t, clearing, domain.Debit, 10000, money.CurrencyEUR),
		makePosting(t, acc.ID, domain.Credit, 10000, money.CurrencyEUR),
	})
	if err := repo.Post(t.Context(), funding); err != nil {
		t.Fatal(err)
	}
	entries := make([]domain.Transaction, 2)
	for i := range entries {
		entries[i] = makeTransaction(t, []domain.Posting{
			makePosting(t, acc.ID, domain.Debit, 8000, money.CurrencyEUR),
			makePosting(t, clearing, domain.Credit, 8000, money.CurrencyEUR),
		})
	}
	start := make(chan struct{})
	type result struct {
		id  uuid.UUID
		err error
	}
	results := make(chan result, 2)
	for _, entry := range entries {
		go func() {
			<-start
			results <- result{entry.ID(), repo.Post(t.Context(), entry)}
		}()
	}
	close(start)
	successes, rejected := 0, 0
	for range entries {
		r := <-results
		switch {
		case r.err == nil:
			successes++
			assertEntryCounts(t, pool, r.id, 1, 2)
		case errors.Is(r.err, domain.ErrInsufficientFunds):
			rejected++
			assertEntryCounts(t, pool, r.id, 0, 0)
		default:
			t.Errorf("unexpected posting result: %v", r.err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("expected one success and one insufficient-funds rejection, got %d and %d", successes, rejected)
	}
	assertBalance(t, pool, acc.ID, "8000", "10000", "2000")
	assertBalance(t, pool, clearing, "10000", "8000", "-2000")
	assertReconciled(t, pool)
}

func TestNonnegativePolicyUsesAccountNormalSide(t *testing.T) {
	repo, pool := setupTestRepository(t)
	for _, accountType := range []account.AccountType{account.Asset, account.Liability} {
		t.Run(string(accountType), func(t *testing.T) {
			acc, err := account.NewWithType(account.CurrencyUSD, accountType)
			if err != nil {
				t.Fatal(err)
			}
			acc.EnforceNonnegativeBalance = true
			if err := accountpostgres.New(pool).Create(t.Context(), acc); err != nil {
				t.Fatal(err)
			}
			clearing := createAccount(t, pool, money.CurrencyUSD)
			debit, credit := acc.ID, clearing
			if accountType == account.Asset {
				debit, credit = clearing, acc.ID
			}
			entry := makeTransaction(t, []domain.Posting{
				makePosting(t, debit, domain.Debit, 1, money.CurrencyUSD),
				makePosting(t, credit, domain.Credit, 1, money.CurrencyUSD),
			})
			if err := repo.Post(t.Context(), entry); !errors.Is(err, domain.ErrInsufficientFunds) {
				t.Fatalf("negative posted balance must be rejected: %v", err)
			}
			assertEntryCounts(t, pool, entry.ID(), 0, 0)
			assertBalance(t, pool, acc.ID, "0", "0", "0")
		})
	}
	assertReconciled(t, pool)
}

func TestNonnegativePolicyChecksFinalTotalsForDirectSQL(t *testing.T) {
	_, pool := setupTestRepository(t)
	acc := account.New(account.CurrencyEUR)
	acc.EnforceNonnegativeBalance = true
	if err := accountpostgres.New(pool).Create(t.Context(), acc); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	id := uuid.New()
	if _, err := tx.Exec(t.Context(), `INSERT INTO ledger_transactions (id, created_at, posting_count) VALUES ($1, now(), 2)`, id); err != nil {
		t.Fatal(err)
	}
	// Separate SQL statements temporarily make this credit-normal account
	// negative. Only the final balanced transaction must satisfy its limit.
	for _, side := range []string{"debit", "credit"} {
		if _, err := tx.Exec(t.Context(), `INSERT INTO ledger_postings (id, transaction_id, account_id, side, amount_minor, currency)
			VALUES ($1, $2, $3, $4, 100, 'EUR')`, uuid.New(), id, acc.ID, side); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("valid final balance must commit: %v", err)
	}
	assertBalance(t, pool, acc.ID, "100", "100", "0")
	assertEntryCounts(t, pool, id, 1, 2)

	// The same direct SQL path must enforce the limit when the final total
	// really is negative, even though the ledger entry itself is balanced.
	clearing := createAccount(t, pool, money.CurrencyEUR)
	tx, err = pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	id = uuid.New()
	if _, err := tx.Exec(t.Context(), `INSERT INTO ledger_transactions (id, created_at, posting_count) VALUES ($1, now(), 2)`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), `INSERT INTO ledger_postings (id, transaction_id, account_id, side, amount_minor, currency)
		VALUES ($1, $2, $3, 'debit', 1, 'EUR'), ($4, $2, $5, 'credit', 1, 'EUR')`, uuid.New(), id, acc.ID, uuid.New(), clearing); err != nil {
		t.Fatal(err)
	}
	assertDatabaseError(t, tx.Commit(t.Context()), "23514")
	assertBalance(t, pool, acc.ID, "100", "100", "0")
	assertBalance(t, pool, clearing, "0", "0", "0")
	assertEntryCounts(t, pool, id, 0, 0)
	assertReconciled(t, pool)
}

func TestBalanceLimitFailureRollsBackEveryCurrency(t *testing.T) {
	repo, pool := setupTestRepository(t)
	eurDebit := createAccount(t, pool, money.CurrencyEUR)
	eurCredit := createAccount(t, pool, money.CurrencyEUR)
	usdCredit := createAccount(t, pool, money.CurrencyUSD)
	restricted := account.New(account.CurrencyUSD)
	restricted.EnforceNonnegativeBalance = true
	if err := accountpostgres.New(pool).Create(t.Context(), restricted); err != nil {
		t.Fatal(err)
	}
	entry := makeTransaction(t, []domain.Posting{
		makePosting(t, eurDebit, domain.Debit, 100, money.CurrencyEUR),
		makePosting(t, eurCredit, domain.Credit, 100, money.CurrencyEUR),
		makePosting(t, restricted.ID, domain.Debit, 1, money.CurrencyUSD),
		makePosting(t, usdCredit, domain.Credit, 1, money.CurrencyUSD),
	})
	if err := repo.Post(t.Context(), entry); !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("expected insufficient funds: %v", err)
	}
	for _, id := range []uuid.UUID{eurDebit, eurCredit, usdCredit, restricted.ID} {
		assertBalance(t, pool, id, "0", "0", "0")
	}
	assertEntryCounts(t, pool, entry.ID(), 0, 0)
	assertReconciled(t, pool)
}

func TestBalanceAndAccountingIdentityCannotBeEditedDirectly(t *testing.T) {
	_, pool := setupTestRepository(t)
	id := createAccount(t, pool, money.CurrencyEUR)
	for _, query := range []string{
		`UPDATE account_balances SET credits_minor=100 WHERE account_id=$1`,
		`DELETE FROM account_balances WHERE account_id=$1`,
		`INSERT INTO account_balances (account_id) VALUES ($1)`,
		`UPDATE accounts SET account_type='asset' WHERE id=$1`,
		`UPDATE accounts SET currency='USD' WHERE id=$1`,
		`UPDATE accounts SET id=gen_random_uuid() WHERE id=$1`,
		`UPDATE accounts SET enforce_nonnegative_balance=true WHERE id=$1`,
	} {
		_, err := pool.Exec(t.Context(), query, id)
		assertDatabaseError(t, err, "23514")
	}
	_, err := pool.Exec(t.Context(), `TRUNCATE account_balances`)
	assertDatabaseError(t, err, "23514")
	assertBalance(t, pool, id, "0", "0", "0")
	assertReconciled(t, pool)
}

func TestBalanceMigrationBackfillsAndCanBeReapplied(t *testing.T) {
	repo, pool := setupTestRepositoryWithMigrations(t, "00001_create_accounts.sql", "00002_create_ledger.sql")
	debit := createAccount(t, pool, money.CurrencyEUR)
	credit := createAccount(t, pool, money.CurrencyEUR)
	empty := createAccount(t, pool, money.CurrencyGBP)
	entry := makeTransaction(t, []domain.Posting{
		makePosting(t, debit, domain.Debit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, debit, domain.Debit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, credit, domain.Credit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, credit, domain.Credit, math.MaxInt64, money.CurrencyEUR),
	})
	if err := repo.Post(t.Context(), entry); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "00003_create_account_balances.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up, down, found := strings.Cut(string(contents), "-- +goose Down")
	if !found {
		t.Fatal("balance migration has no Down section")
	}
	for cycle := range 2 {
		if _, err := pool.Exec(t.Context(), up); err != nil {
			t.Fatalf("apply balance migration: %v", err)
		}
		pool.Reset()
		assertBalance(t, pool, debit, "18446744073709551614", "0", "-18446744073709551614")
		assertBalance(t, pool, credit, "0", "18446744073709551614", "18446744073709551614")
		assertBalance(t, pool, empty, "0", "0", "0")
		assertReconciled(t, pool)
		if cycle == 0 {
			if _, err := pool.Exec(t.Context(), down); err != nil {
				t.Fatalf("roll back balance migration: %v", err)
			}
			pool.Reset()
			assertEntryCounts(t, pool, entry.ID(), 1, 4)
		}
	}
}

func TestReconciliationDetectsCorruptedOrMissingProjection(t *testing.T) {
	_, pool := setupTestRepository(t)
	id := createAccount(t, pool, money.CurrencyEUR)
	assertReconciled(t, pool)
	// Simulate privileged corruption in this isolated test schema. Normal writes
	// are protected; reconciliation must still detect administrative mistakes.
	if _, err := pool.Exec(t.Context(), `ALTER TABLE account_balances DISABLE TRIGGER account_balances_no_direct_mutation`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`UPDATE account_balances SET credits_minor=1 WHERE account_id=$1`,
		`DELETE FROM account_balances WHERE account_id=$1`,
	} {
		if _, err := pool.Exec(t.Context(), query, id); err != nil {
			t.Fatal(err)
		}
		var matches bool
		if err := pool.QueryRow(t.Context(), `SELECT matches FROM account_balance_reconciliation WHERE account_id=$1`, id).Scan(&matches); err != nil || matches {
			t.Fatalf("reconciliation missed corruption: matches=%t err=%v", matches, err)
		}
	}
	if _, err := accountpostgres.New(pool).GetBalance(t.Context(), id); err == nil || errors.Is(err, account.ErrNotFound) {
		t.Fatalf("missing projection must fail rather than appear as zero or unknown account: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `ALTER TABLE account_balances ENABLE TRIGGER account_balances_no_direct_mutation`); err != nil {
		t.Fatal(err)
	}
	entry := makeTransaction(t, []domain.Posting{
		makePosting(t, id, domain.Debit, 1, money.CurrencyEUR),
		makePosting(t, id, domain.Credit, 1, money.CurrencyEUR),
	})
	if err := New(pool).Post(t.Context(), entry); err == nil {
		t.Fatal("posting must fail if its projection is missing")
	}
	assertEntryCounts(t, pool, entry.ID(), 0, 0)
}
