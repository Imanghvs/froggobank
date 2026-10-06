# API

[Documentation index](README.md) · [Project overview](../README.md)

## Endpoints

| Method | Path | Authentication | Purpose |
| --- | --- | --- | --- |
| GET | `/health` | Public | Process liveness |
| GET | `/ready` | Public | PostgreSQL readiness |
| GET | `/me` | Bearer token | Local user ID and creation time |
| POST | `/accounts` | Bearer token | Create an owned customer account |
| GET | `/accounts` | Bearer token | List the caller's owned accounts |
| GET | `/accounts/{id}` | Bearer token | Read an owned account |
| GET | `/accounts/{id}/balance` | Bearer token | Read its posted balance |

See [authentication and ownership](authentication.md) for authorization and
error behavior, and [account balances](ledger.md#account-balances) for request
and response examples. Ledger posting remains internal; no deposit, withdrawal,
or transfer HTTP endpoint is provided.

## List Accounts

```http
GET /accounts?currency=EUR&limit=2&offset=0
Authorization: Bearer <access token>
```

| Query parameter | Default | Allowed values |
| --- | --- | --- |
| `limit` | `20` | Integer from 1 to 100 |
| `offset` | `0` | Nonnegative integer; number of matching accounts to skip |
| `currency` | Omitted (all currencies) | Exactly `EUR`, `USD`, or `GBP` |

Only accounts owned by the authenticated caller are included. Other users'
accounts and unowned internal accounts are excluded. Results are ordered by
`created_at DESC, id DESC`; the ID breaks ties when timestamps match. Pagination
is applied after ownership and optional currency filtering. For example,
`?currency=EUR&limit=2&offset=1` skips the newest matching EUR account and
returns up to the next two EUR accounts owned by the caller.

Omitting `currency` includes all supported currencies. Supplying an empty value
(`?currency=` or `?currency`), an unsupported code such as `JPY`, a lowercase
code such as `eur`, or surrounding whitespace returns HTTP 400. Currency values
are not trimmed or converted to uppercase. Authentication runs before query
validation.

Example HTTP 200 response:

```json
{
  "accounts": [
    {
      "id": "82875e6c-857c-4ca0-9301-af810c68c5a9",
      "currency": "EUR",
      "account_type": "liability",
      "enforce_nonnegative_balance": true,
      "created_at": "2026-01-01T12:00:00Z"
    }
  ],
  "limit": 2,
  "offset": 0
}
```

Account fields match the creation and single-account responses; owner IDs are
not exposed. `limit` and `offset` describe the requested page. A page may contain
fewer than `limit` accounts, and no total count is returned. Separate requests
do not share a database snapshot, so newly created accounts can shift later
pages.

When no owned accounts match the filter, or the offset is beyond the matching
results, the endpoint returns HTTP 200 with an empty array. For a caller without
accounts, the default request returns:

```json
{"accounts": [], "limit": 20, "offset": 0}
```

| Status | Meaning |
| --- | --- |
| `200` | Page returned, including an empty page |
| `400` | Invalid pagination or currency; the service is not called |
| `401` | Missing or invalid authentication, with a Bearer challenge |
| `500` | Account listing failed; internal error details are not exposed |

Example pagination validation response:

```json
{"error": "limit must be 1-100 and offset must be nonnegative integers"}
```

Example currency validation response:

```json
{"error": "invalid currency"}
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

## Related Documentation

- [Getting started](getting-started.md) — obtain and use an access token
- [Authentication](authentication.md) — token requirements and ownership
- [Money, ledger, and balances](ledger.md) — amounts and balance semantics
