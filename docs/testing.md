# Testing and Code Quality

[Documentation index](README.md) · [Project overview](../README.md)

Run all commands from the repository root.

Use golangci-lint v2.14.0 (the version pinned in CI). On macOS, install it
with `brew install golangci-lint`; for other platforms or a specific version,
see the [installation guide](https://golangci-lint.run/docs/welcome/install/local/).

```bash
make fmt   # Format Go files and organize imports
make lint  # Check formatting and run the standard linters
make test  # Run tests
```

The shared `.golangci.yml` enables `goimports` formatting, with project imports
grouped separately, and the standard `errcheck`, `govet`, `ineffassign`,
`staticcheck`, and `unused` linters. CI checks formatting and lint without
rewriting files.

Authentication tests use a local discovery/JWKS fixture with real signed tokens,
including invalid credentials, ID-token rejection, and signing-key rotation.
PostgreSQL integration tests cover concurrent user provisioning, account
ownership, and protected HTTP routes. Automated tests never require live Keycloak.

Domain and application tests run without a database. Application tests fake the
repository port; HTTP tests fake the application service boundary.

Money and ledger unit tests cover currency precision, arithmetic overflow,
posting structure, per-currency balancing, and rejection before persistence.
Ledger PostgreSQL tests cover round trips, partial-write rollback, database
constraints, immutable committed entries, migration backfill, exact balance
totals, reconciliation, concurrent posting, and concurrent overdraft rejection.
They install the actual migrations
in isolated test schemas and drop those schemas for cleanup; the test database
role must be able to create and drop schemas.

PostgreSQL integration tests use `TEST_DATABASE_URL` and skip when it is unset.
Apply the existing migrations to a test database before running them (using
[Goose](https://github.com/pressly/goose)):

```bash
export TEST_DATABASE_URL='postgres://froggobank:froggobank@localhost:5432/froggobank_test?sslmode=disable'
goose -dir migrations postgres "$TEST_DATABASE_URL" up
go test ./...
```

CI provisions PostgreSQL, applies these migrations, and runs the entire suite
with `TEST_DATABASE_URL` set.

A separate CI job builds and starts the [Docker stack](docker.md#ci) and checks
the API's liveness and readiness endpoints.

## Related Documentation

- [Getting started](getting-started.md) — development prerequisites
- [Database schema](database.md) — tables and constraints exercised by integration tests
