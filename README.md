# FroggoBank

An open-source banking backend built with Go.

## Requirements

- Go 1.25+

## Run

```bash
go run ./cmd/api
```

## Configuration

FroggoBank uses environment variables for configuration.

| Variable | Default | Description |
| --- | --- | --- |
| `HTTP_PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, or `error` |
| `DATABASE_URL` | required | PostgreSQL connection URL |

Example:

```bash
DATABASE_URL='postgres://froggobank:froggobank@localhost:5432/froggobank' \
go run ./cmd/api
```

## Health Checks

### Liveness

```http
GET /health
```

Returns 200 OK when the application process is running.

### Readiness

```http
GET /ready
```

Returns 200 OK when PostgreSQL is available, or 503 Service Unavailable otherwise.

## Architecture

FroggoBank is a modular monolith. The account module separates business rules,
use cases, and adapters:

- `internal/account/domain`: accounts, currencies, construction, and semantic
  errors. It has no transport or infrastructure dependencies.
- `internal/account/application`: account creation and retrieval, with a small
  repository port owned by the use cases. Creation validates currency through
  the domain and generates the account before persistence.
- `internal/account/adapters/httpapi`: Gin handlers, request/response DTOs, and
  HTTP error mapping. Handlers consume an application service interface.
- `internal/account/adapters/postgres`: pgx persistence and translation of
  `pgx.ErrNoRows` to the domain's account-not-found error.
- `internal/platform`: shared configuration, database pool setup, and logging.
- `internal/server`: middleware, health/readiness checks, and route registration.

Dependencies point inward to the application and domain. The domain does not
import other internal packages. `cmd/api/main.go` is the composition root: it
wires the pool, PostgreSQL repository, application service, HTTP handler, and
router in that order. The router receives an already constructed account handler.

## Testing

```bash
gofmt -w .
go vet ./...
go test ./...
```

Domain and application tests run without a database. Application tests fake the
repository port; HTTP tests fake the application service boundary.

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
