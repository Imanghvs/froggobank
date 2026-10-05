# Database Schema

[Documentation index](README.md) · [Project overview](../README.md)

These diagrams describe the PostgreSQL schema after migrations 00001–00004.
`PK` marks primary keys and `FK` marks foreign keys. Composite unique keys
are described below the diagrams.

## Users and Account Ownership

```mermaid
erDiagram
    users |o--o{ accounts : owns

    users {
        UUID id PK
        TEXT issuer
        TEXT subject
        TIMESTAMPTZ created_at
    }

    accounts {
        UUID id PK
        UUID owner_id FK "nullable for internal or legacy accounts"
        currency_code currency
        TEXT account_type
        BOOLEAN enforce_nonnegative_balance
        TIMESTAMPTZ created_at
    }
```

A user can own zero or more accounts; an account has zero or one owner.
The pair `(issuer, subject)` uniquely identifies a local user. Owned accounts
must be liabilities with nonnegative-balance enforcement. Assigned ownership
and user identity are immutable.

## Ledger and Balance Projections

The `accounts` entity below is the same table shown above, with only the
columns relevant to the ledger relationships displayed.

```mermaid
erDiagram
    accounts ||--o{ ledger_postings : receives
    ledger_transactions ||--|{ ledger_postings : contains
    accounts ||--|| account_balances : projects

    accounts {
        UUID id PK
        currency_code currency
        TEXT account_type
        BOOLEAN enforce_nonnegative_balance
    }

    ledger_transactions {
        UUID id PK
        TIMESTAMPTZ created_at
        BIGINT posting_count "at least 2"
    }

    ledger_postings {
        UUID id PK
        UUID transaction_id FK
        UUID account_id FK "part of composite foreign key"
        TEXT side "debit or credit"
        BIGINT amount_minor "positive integer"
        currency_code currency FK "part of composite foreign key"
    }

    account_balances {
        UUID account_id PK, FK
        NUMERIC debits_minor "nonnegative integer total"
        NUMERIC credits_minor "nonnegative integer total"
    }
```

Each committed ledger transaction contains at least two postings, with equal
debit and credit totals per currency. Mermaid's one-or-more notation represents
this relationship; deferred database checks enforce the minimum and the exact
`posting_count`. Transactions and postings are append-only.

The unique account key `(id, currency)` is referenced by the composite posting
foreign key `(account_id, currency)`, so a posting always uses its account's
currency (`EUR`, `USD`, or `GBP`). Each account receives one balance projection
through an insert trigger and migration backfill. Posting inserts update these
totals atomically. Posted balance is calculated from the totals and account type;
it is not a stored column.

## Related Documentation

- [Architecture](architecture.md) — module boundaries
- [Authentication](authentication.md) — ownership policy and migration behavior
- [Money, ledger, and balances](ledger.md) — constraints, projections, and reconciliation
