// Package postgres creates disposable schemas with the repository's real
// migrations for integration tests. Only the test-owned schema is dropped.
package postgres

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(t *testing.T, filenames ...string) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.NewWithConfig(t.Context(), config.Copy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(t.Context(), "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration directory")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	if len(filenames) == 0 {
		paths, err := filepath.Glob(filepath.Join(dir, "[0-9]*.sql"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			filenames = append(filenames, filepath.Base(path))
		}
	}
	for _, name := range filenames {
		contents, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		up, _, ok := strings.Cut(string(contents), "-- +goose Down")
		if !ok {
			t.Fatalf("migration %s has no Down", name)
		}
		if _, err := pool.Exec(t.Context(), up); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	return pool
}
