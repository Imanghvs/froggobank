# Docker

[Documentation index](README.md) · [Project overview](../README.md)

Run commands from the repository root. Install Docker with Compose v2 or
newer and start its daemon. Go and Goose are built inside Docker; neither is
required on the host to run the stack.

## Start the Local Stack

```bash
export KEYCLOAK_ADMIN_PASSWORD='choose-a-local-admin-password'
docker compose up --build --wait --wait-timeout 300
```

Compose starts PostgreSQL and Keycloak, applies every pending migration with
Goose, and starts the API after migrations succeed and Keycloak is ready. A
failed migration prevents API startup. The `migrate` container exiting with
code 0 is expected.

| Service | Address or access |
| --- | --- |
| API | `http://localhost:8080` |
| Swagger UI | `http://localhost:8080/docs/` |
| Keycloak administration | `http://localhost:8081`, user `admin` |
| PostgreSQL | Internal Compose network, database/user/password `froggobank` |

This stack is for local development: Keycloak uses development mode and the
database uses fixed local credentials. Published ports bind only to loopback;
PostgreSQL has no published host port.

Swagger UI and its OpenAPI specification are served by the API on port 8080.
“Try it out” uses the same origin; no separate documentation container or CORS
configuration is required. Enter an access token in **Authorize** to call
protected endpoints. Rebuild the API image after editing `docs/openapi.yaml`.
If a previous stack included a `swagger` service, use
`docker compose up --build --wait --wait-timeout 300 --remove-orphans` to remove
that old container.

```bash
curl --fail http://localhost:8080/health
curl --fail http://localhost:8080/ready
docker compose ps --all
docker compose logs -f api
```

Use the [login helper](getting-started.md#start-the-api-with-local-keycloak)
in another terminal to register or sign in and call protected endpoints.
The optional helper requires Python 3; starting the stack does not.

## Configuration and OIDC Networking

`KEYCLOAK_ADMIN_PASSWORD` is required. `LOG_LEVEL` defaults to `info`. Set them
in the shell or an untracked `.env` file; container configuration is supplied at
runtime and is excluded from the image. The Compose file supplies the local
database URL, issuer, audience, and Keycloak access-token profile.

The local issuer is exactly `http://localhost:8081/realms/froggobank` for both
the browser and API. The API shares Keycloak's network namespace, and Keycloak
listens on port 8081 there. API port 8080 is consequently published on the
Keycloak service. This preserves the verifier's loopback-only development HTTP
rule without weakening issuer or JWKS validation. See Docker's
[service network mode reference](https://docs.docker.com/reference/compose-file/services/#network_mode).

Keycloak readiness uses its management endpoint on port 9000, which remains
unpublished. The check follows the
[Keycloak health-check guide](https://www.keycloak.org/observability/health).
The API image checks `/ready` inside its container.

Keep API and Keycloak together when recreating the stack. Stop the auth-only
stack (`docker compose -f compose.auth.yaml down`) or a host API before starting
this stack to free ports 8080 and 8081. Realm import happens only on initial
Keycloak startup; an existing volume retains its configuration.

## Build and Run the API Image Separately

```bash
docker build -t froggobank:local .
docker build --target migrations -t froggobank-migrations:local .
```

The multi-stage build uses Go 1.27.1 on Alpine 3.23 and produces a static binary in an Alpine
runtime with CA certificates. The API and migration images run as UID/GID
65532. Build inputs include Go source, module files, migrations, and the embedded
OpenAPI specification and Swagger UI assets. Other documentation, repository
metadata, local secrets, and test files are excluded.
The API image contains neither the Go toolchain nor Goose.

For an existing migrated database and HTTPS OIDC provider, supply the variables
from [configuration](configuration.md) in an untracked runtime environment file:

```bash
docker run --rm --env-file .env \
  -p 127.0.0.1:8080:8080 froggobank:local
```

URLs in that file must be reachable from inside the container; `localhost`
refers to the container itself. Set `HTTP_PORT=8080` for this port mapping and
leave `OIDC_ALLOW_INSECURE_HTTP=false` for an HTTPS provider. The image alone
does not provision dependencies or apply migrations.

## Migrations, Logs, and Shutdown

```bash
docker compose logs migrate
docker compose run --rm migrate status
docker compose run --rm migrate up
docker compose exec postgres psql -U froggobank -d froggobank
docker compose logs -f
docker compose down
```

PostgreSQL and Keycloak store data in named volumes. `docker compose down`
preserves them, and the next startup applies only pending migrations. Rebuild
and start the stack after adding migrations or changing Go code.

To delete all local database and Keycloak data and start fresh:

```bash
docker compose down --volumes
```

## CI

The Docker job builds both image targets, starts the full stack, waits for
container health checks, verifies `/health` and `/ready`, and removes its
containers and volumes. Logs are collected if the job fails.

## Related Documentation

- [Getting started](getting-started.md) — host development and login helper
- [Configuration](configuration.md) — runtime environment variables
- [Database schema](database.md) — migration tables and relationships
- [Testing](testing.md) — Go tests and code quality
