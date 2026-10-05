package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	accounthttp "github.com/Imanghvs/froggobank/internal/account/adapters/httpapi"
	accountpostgres "github.com/Imanghvs/froggobank/internal/account/adapters/postgres"
	accounts "github.com/Imanghvs/froggobank/internal/account/application"
	account "github.com/Imanghvs/froggobank/internal/account/domain"
	authoidc "github.com/Imanghvs/froggobank/internal/platform/authentication/oidc"
	"github.com/Imanghvs/froggobank/internal/testutil/oidcfixture"
	testdb "github.com/Imanghvs/froggobank/internal/testutil/postgres"
	userpostgres "github.com/Imanghvs/froggobank/internal/user/adapters/postgres"
	users "github.com/Imanghvs/froggobank/internal/user/application"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
	"github.com/google/uuid"
)

func TestAuthenticatedAccountAPIWithPostgres(t *testing.T) {
	pool := testdb.New(t)
	p := oidcfixture.New(t)
	v, err := authoidc.New(t.Context(), authoidc.Config{IssuerURL: p.Issuer, Audience: "froggobank-api", AccessTokenProfile: authoidc.RFC9068, AllowInsecureHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	userService := users.New(userpostgres.New(pool))
	accountRepo := accountpostgres.New(pool)
	accountService := accounts.New(accountRepo)
	router := NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)), pool, accounthttp.New(accountService), v, userService)
	request := func(token, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(t.Context())
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	for _, path := range []string{"/health", "/ready"} {
		if res := request("", http.MethodGet, path, ""); res.Code != 200 {
			t.Fatalf("public %s: %d", path, res.Code)
		}
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/me", ""}, {http.MethodPost, "/accounts", `{"currency":"EUR"}`},
		{http.MethodGet, "/accounts/" + uuid.NewString(), ""}, {http.MethodGet, "/accounts/" + uuid.NewString() + "/balance", ""},
	} {
		if res := request("", tc.method, tc.path, tc.body); res.Code != 401 {
			t.Fatalf("unprotected %s: %d", tc.path, res.Code)
		}
	}
	for _, token := range []string{"invalid", p.Token(t, authoidc.RFC9068, "invalid", map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}), p.Token(t, authoidc.RFC9068, "invalid", map[string]any{"aud": "wrong-api"}),
		p.Sign(t, "JWT", map[string]any{"iss": p.Issuer, "sub": "invalid", "aud": "froggobank-api", "exp": time.Now().Add(time.Minute).Unix()})} {
		if res := request(token, http.MethodGet, "/me", ""); res.Code != 401 {
			t.Fatalf("invalid credentials: %d", res.Code)
		}
	}
	var count int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM users`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid credentials provisioned users: %d %v", count, err)
	}
	tokenA := p.Token(t, authoidc.RFC9068, "user-a", nil)
	tokenB := p.Token(t, authoidc.RFC9068, "user-b", nil)
	profile := request(tokenA, http.MethodGet, "/me", "")
	var me struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(profile.Body.Bytes(), &me); err != nil || profile.Code != 200 || me.ID == uuid.Nil {
		t.Fatalf("profile: %d %s", profile.Code, profile.Body.String())
	}
	if strings.Contains(profile.Body.String(), "user-a") || strings.Contains(profile.Body.String(), "issuer") {
		t.Fatal("provider identity leaked in public profile")
	}
	res := request(tokenA, http.MethodPost, "/accounts", `{"currency":"EUR"}`)
	var created struct {
		ID      uuid.UUID `json:"id"`
		Type    string    `json:"account_type"`
		Enforce bool      `json:"enforce_nonnegative_balance"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil || res.Code != 201 || created.Type != "liability" || !created.Enforce {
		t.Fatalf("create account: %d %s", res.Code, res.Body.String())
	}
	persisted, err := accountRepo.GetByID(t.Context(), created.ID)
	if err != nil || persisted.OwnerID != me.ID {
		t.Fatalf("wrong owner: %+v %v", persisted, err)
	}
	for _, suffix := range []string{"", "/balance"} {
		path := "/accounts/" + created.ID.String() + suffix
		if res := request(tokenA, http.MethodGet, path, ""); res.Code != 200 {
			t.Fatalf("owner denied: %d %s", res.Code, res.Body.String())
		}
		if res := request(tokenB, http.MethodGet, path, ""); res.Code != 404 {
			t.Fatalf("cross-user account exposed: %d %s", res.Code, res.Body.String())
		}
	}
	for _, body := range []string{`{"currency":"EUR","owner_id":"` + uuid.NewString() + `"}`, `{"currency":"EUR","account_type":"asset"}`, `{"currency":"EUR","enforce_nonnegative_balance":false}`} {
		if res := request(tokenA, http.MethodPost, "/accounts", body); res.Code != 400 {
			t.Fatalf("client-controlled policy accepted: %d", res.Code)
		}
	}
	internal := account.New(account.CurrencyEUR)
	if err := accountRepo.Create(t.Context(), internal); err != nil {
		t.Fatal(err)
	}
	if res := request(tokenA, http.MethodGet, "/accounts/"+internal.ID.String(), ""); res.Code != 404 {
		t.Fatal("internal account exposed")
	}
	if res := request(tokenA, http.MethodGet, "/accounts/"+internal.ID.String()+"/balance", ""); res.Code != 404 {
		t.Fatal("internal balance exposed")
	}
	if res := request(tokenA, http.MethodGet, "/accounts/"+uuid.NewString(), ""); res.Code != 404 {
		t.Fatal("missing-account status incorrect")
	}
	// New access tokens and changed profile attributes resolve the same issuer/subject.
	res = request(p.Token(t, authoidc.RFC9068, "user-a", map[string]any{"email": "changed@example.test"}), http.MethodGet, "/me", "")
	var again struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &again); err != nil || again.ID != me.ID {
		t.Fatal("email change replaced user identity")
	}
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM users`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("identity count: %d %v", count, err)
	}
}

func TestOwnershipMigrationLeavesLegacyAccountsInternalAndRollsBack(t *testing.T) {
	pool := testdb.New(t, "00001_create_accounts.sql", "00002_create_ledger.sql", "00003_create_account_balances.sql")
	id := uuid.New()
	if _, err := pool.Exec(t.Context(), `INSERT INTO accounts (id,currency,created_at) VALUES ($1,'EUR',now())`, id); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join("..", "..", "migrations", "00004_create_users_and_account_ownership.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up, down, found := strings.Cut(string(contents), "-- +goose Down")
	if !found {
		t.Fatal("missing Down")
	}
	for cycle := range 2 {
		if _, err := pool.Exec(t.Context(), up); err != nil {
			t.Fatal(err)
		}
		pool.Reset()
		var anonymous bool
		if err := pool.QueryRow(t.Context(), `SELECT owner_id IS NULL FROM accounts WHERE id=$1`, id).Scan(&anonymous); err != nil || !anonymous {
			t.Fatalf("legacy owner assigned: %v", err)
		}
		identity, _ := user.NewIdentity("https://issuer.example", "subject")
		localUser, err := users.New(userpostgres.New(pool)).ResolveIdentity(t.Context(), identity)
		if err != nil {
			t.Fatal(err)
		}
		s := accounts.New(accountpostgres.New(pool))
		if _, err := s.GetAccountByID(t.Context(), localUser.ID, id); !errors.Is(err, account.ErrNotFound) {
			t.Fatalf("legacy account exposed: %v", err)
		}
		if cycle == 0 {
			if _, err := pool.Exec(t.Context(), down); err != nil {
				t.Fatal(err)
			}
			pool.Reset()
			var exists bool
			if err := pool.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM accounts WHERE id=$1)`, id).Scan(&exists); err != nil || !exists {
				t.Fatal("rollback lost account")
			}
		}
	}
}
