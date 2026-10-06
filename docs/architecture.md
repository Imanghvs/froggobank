# Architecture

[Documentation index](README.md) · [Project overview](../README.md)

FroggoBank is a modular monolith. Business modules separate domain rules,
application use cases, and infrastructure adapters:

- `internal/account/domain`: accounts, accounting roles, exact posted balance
  snapshots, currencies, construction, and semantic
  errors. It has no transport or infrastructure dependencies.
- `internal/account/application`: account creation, retrieval, paginated listing,
  and posted balance reads, with a small
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

The user module follows the same domain/application/adapter boundaries and
resolves a verified `(issuer, subject)` to a stable local user. The composition
root also wires OIDC verification and user persistence into authentication
middleware. Account application use cases require the caller ID explicitly and
check ownership independently of HTTP middleware.

For account listing, the HTTP adapter binds query parameters to
`getUserAccountsQuery` and validates pagination. It parses an optional currency
through `domain.ParseCurrency`, then constructs `application.AccountFilter`
with limit, offset, and an optional `*domain.Currency`. A nil currency means no
currency filter. The application service and repository port share this filter;
the trusted caller ID remains a separate argument.

The PostgreSQL adapter filters by owner and optional currency before applying
deterministic ordering, limit, and offset. It assembles fixed SQL fragments and
passes all values as query parameters. The HTTP adapter
maps the resulting domain accounts to public response DTOs, including an empty
JSON array for empty pages. JSON field names and transport binding rules stay
in the HTTP adapter.

## Related Documentation

- [Database schema](database.md) — entity relationship diagrams
- [Authentication](authentication.md) — identity and ownership
- [Money, ledger, and balances](ledger.md) — accounting rules and persistence
