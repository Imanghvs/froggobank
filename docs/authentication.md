# Authentication and Ownership

[Documentation index](README.md) · [Project overview](../README.md)

The API is an OAuth resource server. An identity provider handles registration,
login, logout, password recovery, and MFA; FroggoBank does not store passwords or
implement its own login protocol. Clients obtain an access token through their
provider's authorization-code flow with PKCE, then send it as a Bearer token.

The verifier uses OpenID discovery and cached JWKS with key-rotation refresh.
RS256 signatures, exact issuer, API audience, expiration, subject, not-before,
and access-token type are checked. RFC 9068 tokens must use `at+jwt` or
`application/at+jwt` and contain the required client ID, JWT ID, and issued-at
claims. The `keycloak` profile requires a `JWT` header and a signed `typ=Bearer`
claim. ID tokens are rejected even if they contain the API audience. Opaque
access tokens and other token profiles are not supported in this version.
Discovery must succeed at API startup. HTTPS is required except for explicitly
enabled local loopback development.

`GET /me`, `POST /accounts`, `GET /accounts`, `GET /accounts/{id}`, and
`GET /accounts/{id}/balance` require authentication. Missing, invalid, or expired
credentials return 401 with a Bearer challenge. Account lookups return 404 for
both unknown accounts and accounts the caller does not own. Health/readiness
remain public. Request logs contain no authorization headers, request bodies,
or query parameters.

[Account listing](api.md#list-accounts) filters by the authenticated user's ID
before applying pagination. It excludes unowned internal accounts and returns
an empty array when no owned accounts match.

The first authenticated request provisions a local user. A database unique
constraint on `(issuer, subject)` handles concurrent provisioning. Email is not
an identity key. `GET /me` returns only the local user ID and creation time.

Customer accounts receive the verified local user as owner, a liability role,
and nonnegative-balance enforcement. Creation accepts only `currency`; owner,
accounting type, and policy fields are rejected. Assigned ownership is immutable.
Migration `00004_create_users_and_account_ownership.sql` leaves all existing
accounts unowned and internal. They require a deliberate trusted migration to
establish eligible customer policy and ownership before customer access; there
is no public assignment endpoint. Rolling back migration 00004 removes users
and ownership metadata while retaining accounts and ledger entries.

## Related Documentation

- [Getting started](getting-started.md) — Keycloak, login helper, and token usage
- [Configuration](configuration.md) — issuer, audience, and token profiles
- [Database schema](database.md) — user and account relationships
