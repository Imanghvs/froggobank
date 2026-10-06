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

Account-listing handler and router tests cover caller, pagination, and currency
forwarding, including a nil filter when currency is omitted. They verify that
EUR, USD, and GBP are accepted, while empty, unsupported, lowercase, and padded
values return 400 without calling the service. Router tests also verify that
authentication precedes currency validation. Existing cases cover pagination
validation, public JSON fields, exclusion of owner IDs, empty arrays, and service
errors.

Application tests verify that the caller, context, and complete `AccountFilter`
reach the repository unchanged. PostgreSQL tests verify ownership and currency
filtering before limit and offset, ordering (including equal timestamps), empty
matches, page boundaries, and cancellation.

Run the listing tests without a database:

```bash
go test ./internal/account/application -run '^TestGetUserAccounts' -v -count=1
go test ./internal/account/adapters/httpapi -run '^TestGetUserAccounts|^TestHandlersRequireAuthentication' -v -count=1
go test ./internal/server -run '^TestListAccountsRoute|^TestProtectedRoutes' -v -count=1
```

Run the repository listing test against an existing test database:

```bash
TEST_DATABASE_URL='postgres://froggobank:froggobank@localhost:5432/froggobank_test?sslmode=disable' \
go test ./internal/account/adapters/postgres -run '^TestRepositoryGetUserAccounts$' -v -count=1
```

This repository listing test applies the migrations in a disposable schema and
removes that schema afterward. The database role needs permission to create and
drop schemas. Other tests that use the database's default schema still require
the migrations below when running the complete suite.

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
