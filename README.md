# FroggoBank

An open-source banking backend built with Go and PostgreSQL.

FroggoBank is a modular monolith with OIDC authentication, owned customer
accounts, an append-only double-entry ledger, and exact posted balances.
Ledger posting is currently internal; deposit, withdrawal, and transfer HTTP
endpoints are not yet available.

## Getting Started

With Docker and Compose installed:

```bash
export KEYCLOAK_ADMIN_PASSWORD='choose-a-local-admin-password'
docker compose up --build --wait --wait-timeout 300
```

The API is available at `http://localhost:8080`, with Swagger UI at
`http://localhost:8080/docs/`. See the [Docker guide](docs/docker.md)
for configuration, login, logs, and shutdown.

For development on the host:

You need Go 1.27.1+, PostgreSQL, and an OIDC identity provider. The
[local setup guide](docs/getting-started.md) covers Keycloak, database migrations,
and obtaining an access token.

Once the database is migrated and the required environment variables are set:

```bash
go run ./cmd/api
```

See [configuration](docs/configuration.md) for the required variables.

## Development

With golangci-lint v2.14.0 installed:

```bash
make fmt
make lint
make test
```

See [testing and code quality](docs/testing.md) for installation and PostgreSQL
integration tests.

## Documentation

Browse the [documentation index](docs/README.md), or go directly to a guide:

- [Getting started](docs/getting-started.md)
- [Docker](docs/docker.md)
- [Configuration](docs/configuration.md)
- [API](docs/api.md)
- [Architecture](docs/architecture.md)
- [Database schema and ER diagrams](docs/database.md)
- [Authentication and ownership](docs/authentication.md)
- [Money, ledger, and balances](docs/ledger.md)
- [Testing and code quality](docs/testing.md)
