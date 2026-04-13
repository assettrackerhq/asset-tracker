# Plaid & Teller Bank Account Integration

**Date:** 2026-04-13
**Status:** Draft

## Overview

Add two configurable bank account linking integrations — Plaid and Teller — to the asset tracker. Both are toggled via the KOTS config screen during Embedded Cluster v3 installation. When enabled, users can link sandbox bank accounts and see balances imported as assets. Both providers can coexist simultaneously.

## KOTS Config Screen

New "Integrations" group in `replicated/config.yaml`:

- **Plaid Integration**: `select_one` (disabled/enabled), default disabled
  - Client ID (text, shown when enabled, required)
  - Secret (password, shown when enabled, required)
- **Teller Integration**: `select_one` (disabled/enabled), default disabled
  - Application ID (text, shown when enabled, required)

## HelmChart CR Wiring

Two new `optionalValues` blocks in `replicated/helmchart-asset-tracker.yaml`:

```yaml
- when: '{{repl ConfigOptionEquals "plaid_enabled" "enabled" }}'
  recursiveMerge: true
  values:
    backend:
      plaid:
        enabled: true
        clientId: '{{repl ConfigOption "plaid_client_id" }}'
        secret: '{{repl ConfigOption "plaid_secret" }}'

- when: '{{repl ConfigOptionEquals "teller_enabled" "enabled" }}'
  recursiveMerge: true
  values:
    backend:
      teller:
        enabled: true
        applicationId: '{{repl ConfigOption "teller_application_id" }}'
```

## Helm Values

New sections in `values.yaml`:

```yaml
backend:
  plaid:
    enabled: false
    clientId: ""
    secret: ""
    environment: "sandbox"
  teller:
    enabled: false
    applicationId: ""
    environment: "sandbox"
```

Corresponding schema entries in `values.schema.json`. Environment is hardcoded to sandbox for this demo.

## Backend Deployment

New environment variables injected from Helm values:

- `PLAID_ENABLED`, `PLAID_CLIENT_ID`, `PLAID_SECRET`, `PLAID_ENVIRONMENT`
- `TELLER_ENABLED`, `TELLER_APPLICATION_ID`, `TELLER_ENVIRONMENT`

## Backend Architecture

### Config (`backend/internal/config/config.go`)

New fields: `PlaidEnabled`, `PlaidClientID`, `PlaidSecret`, `PlaidEnvironment`, `TellerEnabled`, `TellerApplicationID`, `TellerEnvironment`.

### New package: `backend/internal/banking`

**Interface:**

```go
type BankProvider interface {
    Name() string
    CreateLinkToken(ctx context.Context, userID string) (string, error)
    ExchangeToken(ctx context.Context, publicToken string) (string, error)
    FetchAccounts(ctx context.Context, accessToken string) ([]Account, error)
}

type Account struct {
    ExternalID  string
    Name        string
    Type        string   // checking, savings, investment
    Balance     float64
    Currency    string
    Institution string
}
```

**Implementations:**

- `plaid.go` — Uses `github.com/plaid/plaid-go` SDK. `CreateLinkToken` calls Plaid API with client_id + secret. `ExchangeToken` exchanges public token for access token. `FetchAccounts` calls `/accounts/balance/get`.
- `teller.go` — Direct HTTP calls to `https://api.teller.io`. `CreateLinkToken` returns empty string (Teller Connect handles this client-side with the application ID). `ExchangeToken` is a passthrough (Teller returns the access token directly from Connect). `FetchAccounts` calls Teller accounts + balances endpoints.

**Handlers (`banking/handler.go`):**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/banking/link-token` | POST | Accepts `{provider}`, returns link token (Plaid) or application ID (Teller) |
| `/api/banking/connect` | POST | Accepts `{provider, token}`, stores access token, fetches initial accounts |
| `/api/banking/accounts` | GET | Lists all linked accounts for the authenticated user |
| `/api/banking/accounts/sync` | POST | Manual refresh of all linked account balances |
| `/api/banking/accounts/{id}` | DELETE | Unlink an account (delete from DB) |

**Middleware:**

`bankingFeatureGate` on `/api/banking/*` — returns 403 if neither Plaid nor Teller is enabled. Individual endpoints also check that the requested provider is enabled.

**Daily sync:**

A goroutine started in `main.go` that runs once every 24 hours. Iterates all stored access tokens grouped by source, calls `FetchAccounts` on the appropriate provider, and updates balances by creating new value points (building history over time).

## Database Schema

Add columns to the `assets` table via a SchemaHero migration:

```sql
ALTER TABLE assets ADD COLUMN source TEXT NOT NULL DEFAULT 'manual';
ALTER TABLE assets ADD COLUMN external_id TEXT;
ALTER TABLE assets ADD COLUMN access_token TEXT;
ALTER TABLE assets ADD COLUMN institution TEXT;
```

- `source`: `manual`, `plaid`, or `teller`
- `external_id`: Account ID from the provider (for dedup on sync)
- `access_token`: Stored per linked account for API calls (plaintext, acceptable for sandbox demo)
- `institution`: Bank name from the provider

Linked accounts are assets with `source != 'manual'`. Value points work the same way — the daily sync creates new value points with updated balances.

## Frontend

### New page: `LinkedAccounts.jsx`

Route: `/linked-accounts`, protected. Visible in NavBar when `plaidEnabled || tellerEnabled`.

**Layout:**
- Header: "Linked Accounts"
- Action buttons (conditional):
  - "Link with Plaid" — shown when Plaid enabled. Calls backend for link token, opens `react-plaid-link` modal. Sandbox test credentials: `user_good` / `pass_good`.
  - "Link with Teller" — shown when Teller enabled. Opens `teller-connect-react` modal with application ID from features endpoint. Sandbox test credentials: `username` / `password`.
- Table: Name, Institution, Type, Balance, Currency, Source (badge), Last Synced, Unlink button
- "Sync Now" button to trigger manual refresh

### Features endpoint changes

Extend `GET /api/features` response:

```json
{
  "analytics_enabled": true,
  "plaid_enabled": true,
  "teller_enabled": true,
  "teller_application_id": "app_abc123"
}
```

The Teller application ID is public and needed by the frontend SDK. Plaid credentials stay server-side only.

### New npm dependencies

- `react-plaid-link` — Official Plaid Link React component
- `teller-connect-react` — Official Teller Connect React component

### NavBar changes

Add "Linked Accounts" link, conditionally shown when `plaidEnabled || tellerEnabled` (passed down like `analyticsEnabled`).

### API additions (`api.js`)

- `createLinkToken(provider)` — POST `/api/banking/link-token`
- `connectAccount(provider, token)` — POST `/api/banking/connect`
- `listLinkedAccounts()` — GET `/api/banking/accounts`
- `syncAccounts()` — POST `/api/banking/accounts/sync`
- `unlinkAccount(id)` — DELETE `/api/banking/accounts/{id}`

## E2E Testing

Add assertions to the existing Playwright spec (`asset-tracker.spec.mjs`):

- Verify "Linked Accounts" nav link appears when features are enabled
- Verify "Link with Plaid" and "Link with Teller" buttons are present

Third-party Link modals (Plaid Link, Teller Connect) are not tested in CI — they load iframes that are unreliable in headless browsers. The test validates the config toggle wiring.

CI config values include dummy Plaid/Teller credentials to enable the features:

```yaml
plaid_enabled: "enabled"
plaid_client_id: "test_client_id"
plaid_secret: "test_secret"
teller_enabled: "enabled"
teller_application_id: "test_app_id"
```

## Out of Scope

- Production Plaid/Teller environments (sandbox only)
- Access token encryption at rest
- Multiple link sessions per provider per user (re-linking replaces the existing access token and upserts accounts by external_id)
- Webhook-based real-time updates (daily poll is sufficient)
