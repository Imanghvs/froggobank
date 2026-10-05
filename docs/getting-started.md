# Getting Started

[Documentation index](README.md) · [Project overview](../README.md)

Run all commands in this guide from the repository root.

To run the complete stack without installing Go or Goose, use the
[Docker guide](docker.md). The steps below run the API on the host.

## Prerequisites

- Go 1.25+
- A running PostgreSQL instance and an existing database
- Docker Compose for the supplied local Keycloak environment
- Goose for applying migrations
- Python 3 if you use the optional login helper

The auth-only `compose.auth.yaml` file starts Keycloak only. Provision PostgreSQL separately; the
example below assumes a database named `froggobank` with a `froggobank` role
and password on `localhost:5432`.

Install the migration tool used by CI:

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
```

Ensure your Go binary directory is on `PATH` so the `goose` command is available.

## Start the API with Local Keycloak


The supplied Keycloak realm enables registration, password recovery, and TOTP
enrollment on first sign-in. It includes a public PKCE client whose access tokens
have the `froggobank-api` audience; ID tokens do not receive that audience.

```bash
export KEYCLOAK_ADMIN_PASSWORD='choose-a-local-admin-password'
docker compose -f compose.auth.yaml up -d

export OIDC_ISSUER_URL='http://localhost:8081/realms/froggobank'
export OIDC_AUDIENCE='froggobank-api'
export OIDC_ACCESS_TOKEN_PROFILE='keycloak'
export OIDC_ALLOW_INSECURE_HTTP='true'
export DATABASE_URL='postgres://froggobank:froggobank@localhost:5432/froggobank?sslmode=disable'

goose -dir migrations postgres "$DATABASE_URL" up
go run ./cmd/api
```

Keycloak's administration console is at `http://localhost:8081`, with bootstrap
username `admin` and the password you supplied. Configure SMTP on the FroggoBank
realm for password-recovery email. Email verification is disabled in this local
fixture; configure SMTP, enable verification, and use a production Keycloak
deployment or another compatible provider outside local development.

In another terminal, the optional Python 3 developer helper opens provider login
or registration using authorization code + PKCE:

```bash
python3 dev/login.py
```

Complete registration/login and TOTP setup in the provider's browser UI. The
helper saves only the access token to a private temporary file and prints its
path. Use that path without printing the token:

```bash
TOKEN_FILE='/path/printed/by/login/helper.token'
curl -H "Authorization: Bearer $(cat "$TOKEN_FILE")" http://localhost:8080/me
curl -H "Authorization: Bearer $(cat "$TOKEN_FILE")" \
  -H 'Content-Type: application/json' \
  -d '{"currency":"EUR"}' http://localhost:8080/accounts
rm "$TOKEN_FILE"
```

Users can manage MFA and sign-in sessions through
`http://localhost:8081/realms/froggobank/account/`. Provider logout ends the login
session, but previously issued JWT access tokens remain valid until expiration
(five minutes in the local realm). Refresh/ID tokens are not stored by the API
or the helper. The local helper uses the fixed `froggobank-cli` client and
`http://127.0.0.1:8090/callback`; other clients must be configured with their own
redirect URIs and API audience. Realm import occurs only on initial startup;
restarting with an existing volume does not overwrite its configuration.

## Related Documentation

- [Configuration](configuration.md) — environment variables and another identity provider
- [Authentication](authentication.md) — token validation and account ownership
- [API](api.md) — health checks and customer endpoints
- [Testing](testing.md) — formatting, linting, and tests
