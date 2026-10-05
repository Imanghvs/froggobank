package postgres

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	accounts "github.com/Imanghvs/froggobank/internal/account/adapters/postgres"
	account "github.com/Imanghvs/froggobank/internal/account/domain"
	testdb "github.com/Imanghvs/froggobank/internal/testutil/postgres"
	"github.com/Imanghvs/froggobank/internal/user/application"
	"github.com/Imanghvs/froggobank/internal/user/domain"
)

func TestFindOrCreateConcurrentRequests(t *testing.T) {
	pool := testdb.New(t)
	s := application.New(New(pool))
	identity, _ := domain.NewIdentity("https://issuer.example", "subject")
	const count = 32
	type result struct {
		user domain.User
		err  error
	}
	results := make(chan result, count)
	start := make(chan struct{})
	for range count {
		go func() { <-start; u, err := s.ResolveIdentity(t.Context(), identity); results <- result{u, err} }()
	}
	close(start)
	var first domain.User
	for range count {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		if first.ID == uuid.Nil {
			first = r.user
		}
		if r.user != first {
			t.Fatalf("duplicate identity: %+v != %+v", r.user, first)
		}
	}
	var rows int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM users`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("user rows=%d err=%v", rows, err)
	}
	otherIdentity, _ := domain.NewIdentity("https://other-issuer.example", identity.Subject())
	other, err := s.ResolveIdentity(t.Context(), otherIdentity)
	if err != nil || other.ID == first.ID {
		t.Fatalf("issuers were conflated: %v", err)
	}
	differentSubject, _ := domain.NewIdentity(identity.Issuer(), "another-subject")
	other, err = s.ResolveIdentity(t.Context(), differentSubject)
	if err != nil || other.ID == first.ID {
		t.Fatalf("subjects were conflated: %v", err)
	}
}

func TestDatabaseCustomerOwnershipConstraints(t *testing.T) {
	pool := testdb.New(t)
	users := application.New(New(pool))
	identity, _ := domain.NewIdentity("https://issuer.example", "a")
	owner, err := users.ResolveIdentity(t.Context(), identity)
	if err != nil {
		t.Fatal(err)
	}
	identity, _ = domain.NewIdentity("https://issuer.example", "b")
	other, err := users.ResolveIdentity(t.Context(), identity)
	if err != nil {
		t.Fatal(err)
	}
	repo := accounts.New(pool)
	a, err := account.NewCustomer(account.CurrencyEUR, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(t.Context(), a.ID)
	if err != nil || got.OwnerID != owner.ID {
		t.Fatalf("owner not persisted: %+v %v", got, err)
	}
	for _, newOwner := range []any{other.ID, nil} {
		_, err := pool.Exec(t.Context(), `UPDATE accounts SET owner_id=$1 WHERE id=$2`, newOwner, a.ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Fatalf("owner mutation allowed: %v", err)
		}
	}
	for _, modify := range []func(*account.Account){
		func(a *account.Account) { a.Type = account.Asset },
		func(a *account.Account) { a.EnforceNonnegativeBalance = false },
	} {
		bad := a
		bad.ID = uuid.New()
		modify(&bad)
		err := repo.Create(t.Context(), bad)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Fatalf("invalid customer policy persisted: %v", err)
		}
	}
	_, err = pool.Exec(t.Context(), `DELETE FROM users WHERE id=$1`, owner.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("owned user deletion allowed: %v", err)
	}
	for _, query := range []string{
		`UPDATE users SET id=gen_random_uuid() WHERE id=$1`,
		`UPDATE users SET issuer='https://attacker.example' WHERE id=$1`,
		`UPDATE users SET subject='attacker' WHERE id=$1`,
	} {
		_, err := pool.Exec(t.Context(), query, owner.ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Fatalf("user identity mutation allowed: %v", err)
		}
	}
	internal := account.New(account.CurrencyEUR)
	if err := repo.Create(t.Context(), internal); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(t.Context(), internal.ID)
	if err != nil || got.OwnerID != uuid.Nil {
		t.Fatalf("internal account assigned: %+v %v", got, err)
	}
}
