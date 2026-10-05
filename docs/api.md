# API

[Documentation index](README.md) · [Project overview](../README.md)

## Endpoints

| Method | Path | Authentication | Purpose |
| --- | --- | --- | --- |
| GET | `/health` | Public | Process liveness |
| GET | `/ready` | Public | PostgreSQL readiness |
| GET | `/me` | Bearer token | Local user ID and creation time |
| POST | `/accounts` | Bearer token | Create an owned customer account |
| GET | `/accounts/{id}` | Bearer token | Read an owned account |
| GET | `/accounts/{id}/balance` | Bearer token | Read its posted balance |

See [authentication and ownership](authentication.md) for authorization and
error behavior, and [account balances](ledger.md#account-balances) for request
and response examples. Ledger posting remains internal; no deposit, withdrawal,
or transfer HTTP endpoint is provided.

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

## Related Documentation

- [Getting started](getting-started.md) — obtain and use an access token
- [Authentication](authentication.md) — token requirements and ownership
- [Money, ledger, and balances](ledger.md) — amounts and balance semantics
