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

FroggoBank is a modular monolith. Business modules separate domain rules,
application use cases, and infrastructure adapters:

- `internal/account/domain`: accounts, accounting roles, exact posted balance
  snapshots, currencies, construction, and semantic
  errors. It has no transport or infrastructure dependencies.
- `internal/account/application`: account creation, retrieval, and posted balance
  reads, with a small
  repository port owned by the use cases. Creation validates currency through
  the domain and generates the account before persistence.
- `internal/account/adapters/httpapi`: Gin handlers, request/response DTOs, and
  HTTP error mapping. Handlers consume an application service interface.
- `internal/account/adapters/postgres`: pgx persistence and translation of
  `pgx.ErrNoRows` to the domain's account-not-found error.
- `internal/money/domain`: integer minor-unit money values, supported currencies,
  explicit currency scales, and checked arithmetic.
- `internal/ledger/domain`: immutable postings and balanced transactions.
- `internal/ledger/application`: the posting use case and an atomic repository
  port.
- `internal/ledger/adapters/postgres`: atomic, append-only ledger persistence
  with synchronous balance projections and account balance limits.
- `internal/platform`: shared configuration, database pool setup, and logging.
- `internal/server`: middleware, health/readiness checks, and route registration.

Dependencies point inward to the application and domain. Domain packages may
share other pure domain packages, but have no transport, application, or
infrastructure dependencies. Accounts and the ledger share currency rules from
the money domain. `cmd/api/main.go` is the composition root for the HTTP API: it
wires the pool, PostgreSQL repository, application service, HTTP handler, and
router in that order. The router receives an already constructed account handler.

## Money and Ledger

Money stores a signed `int64` count of minor units. EUR, USD and GBP are supported,
each with scale 2 (100 minor units per major unit). For example,
`money.New(1234, money.CurrencyEUR)` represents 1234 EUR minor units. Constructors
and arithmetic use integers throughout; addition and subtraction reject currency
mismatches and overflow. Zero and negative money values are valid quantities,
but ledger postings require strictly positive amounts.

Construct postings with `domain.NewPosting(accountID, side, amount)`, using
`domain.Debit` or `domain.Credit`. The ledger application's
`PostTransaction(ctx, postings)` creates and validates a transaction before
calling its repository. Transactions require at least two valid postings with
unique posting IDs. Debit and credit totals must balance separately for every
currency; exact integer accumulation prevents overflow in aggregate totals.
Posting and transaction fields are private, and posting slices are copied on
construction and access.

Migration `00002_create_ledger.sql` adds ledger transactions and postings and
converts both account and posting currencies to the shared PostgreSQL
`currency_code` enum (`EUR`, `USD`, `GBP`). The PostgreSQL repository writes the
transaction and every posting in one database
transaction, rolling everything back on failure. A composite account/currency
foreign key requires each posting's currency to match its existing account.
Deferred database checks also enforce posting count and per-currency balance at
commit, including when SQL bypasses the application.

Both ledger tables reject updates, deletes and truncation. A transaction's
immutable posting count prevents adding postings after it has committed.
Corrections must be recorded as new balanced transactions. The ledger posting
use case remains internal; no deposit, withdrawal, or transfer HTTP endpoint is
provided.

## Account Balances

Migration `00003_create_account_balances.sql` adds accounting types and one
balance projection per account. Existing accounts become liabilities with
unrestricted balances. New accounts default to the same policy; applications
must explicitly enable the nonnegative policy for accounts that cannot overdraw.

| Account type | Normal side | Posted balance |
| --- | --- | --- |
| `asset`, `expense` | Debit | Debits minus credits |
| `liability`, `equity`, `revenue` | Credit | Credits minus debits |

Customer deposit accounts are normally liabilities from the bank's perspective.
These accounting types describe ledger accounts; customer/product accounts can
later map to several ledger accounts. Account identity, currency, accounting
type, and balance policy are immutable after creation.

Create a liability account that cannot have a negative posted balance:

```http
POST /accounts
Content-Type: application/json

{"currency":"EUR","account_type":"liability","enforce_nonnegative_balance":true}
```

Read its posted balance:

```http
GET /accounts/{id}/balance
```

Example after postings crediting 10000 and debiting 2500 EUR minor units:

```json
{
  "account_id": "82875e6c-857c-4ca0-9301-af810c68c5a9",
  "currency": "EUR",
  "account_type": "liability",
  "normal_side": "credit",
  "scale": 2,
  "debits_minor": "2500",
  "credits_minor": "10000",
  "posted_minor": "7500"
}
```

The posted balance is EUR 75.00. An existing account with no postings returns
zero; an unknown account returns 404. Totals are decimal strings in JSON to
preserve integer precision in clients. Go snapshots use `big.Int`; PostgreSQL
uses exact `NUMERIC` values constrained to finite, nonnegative integers. Lifetime
debit/credit turnover may exceed `int64`, although individual postings use
`int64` minor units.

A statement-level PostgreSQL trigger aggregates every inserted posting and
updates affected balance rows in account-ID order. Updates acquire row locks,
and projections commit or roll back with their ledger entries. This also applies
to direct SQL writes. Deferred checks reject negative final posted balances for
accounts with `enforce_nonnegative_balance=true`; the repository reports
`ledger/domain.ErrInsufficientFunds`. Concurrent postings serialize on the
affected rows, so two 8000-unit spends against 10000 units cannot both commit.
Unrestricted accounts can have negative posted balances. Ordinary SQL cannot
edit or truncate projections independently of the ledger.

Posted balance is not yet available balance. Holds, overdrafts, pending incoming
funds, and idempotency remain separate future money-movement features. There is
no available-balance response or reservation mechanism yet.

The migration backfills balances from existing postings. To reconcile balances
against authoritative postings using one consistent database snapshot, run:

```sql
SELECT * FROM account_balance_reconciliation WHERE NOT matches;
```

An empty result means all projections agree, including accounts with no
postings. Schedule this query in operational monitoring; no scheduler is bundled.
The view detects missing projections as well as incorrect totals. Migration
rollback retains the ledger but removes projections, accounting types, and
balance policies. Reapplying reconstructs the totals from postings and restores
the default liability type and unrestricted policy.

## Testing

```bash
gofmt -w .
go vet ./...
go test ./...
```

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
