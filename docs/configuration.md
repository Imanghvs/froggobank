# Configuration

[Documentation index](README.md) · [Project overview](../README.md)

FroggoBank uses environment variables for configuration.

| Variable | Default | Description |
| --- | --- | --- |
| `HTTP_PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, or `error` |
| `DATABASE_URL` | required | PostgreSQL connection URL |
| `OIDC_ISSUER_URL` | required | Exact identity-provider issuer URL |
| `OIDC_AUDIENCE` | required | Audience identifying the FroggoBank API |
| `OIDC_ACCESS_TOKEN_PROFILE` | `rfc9068` | Access-token profile: `rfc9068` or `keycloak` |
| `OIDC_ALLOW_INSECURE_HTTP` | `false` | Allow HTTP issuer/JWKS URLs on loopback for local development only |

Example:

```bash
DATABASE_URL='postgres://froggobank:froggobank@localhost:5432/froggobank' \
OIDC_ISSUER_URL='https://identity.example/realms/froggobank' \
OIDC_AUDIENCE='froggobank-api' \
go run ./cmd/api
```

## Related Documentation

- [Getting started](getting-started.md) — local environment setup
- [Authentication](authentication.md) — supported access-token profiles
