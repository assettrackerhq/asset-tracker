# Analytics Page Design

## Overview

Add an Analytics page to the asset tracker that shows portfolio value over time as a chart, with a total portfolio value summary. All values are converted to a user-selected display currency using manually managed (or fetched) exchange rates. A new Exchange Rates page allows users to manage conversion rates.

This feature will later be gated behind a Replicated license entitlement. This spec covers only the feature itself, not the gating.

## Data Model

### New table: `exchange_rates`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK, auto-generated |
| base_currency | varchar(3) | e.g., "USD" |
| target_currency | varchar(3) | e.g., "EUR" |
| rate | numeric(15,6) | conversion rate |
| updated_at | timestamp | defaults to now, updates on modification |

- Unique constraint on `(base_currency, target_currency)` — one rate per pair.
- Not user-scoped — exchange rates are shared globally (factual data).
- SchemaHero YAML in `schemas/tables/exchange_rates.yaml`, DDL added to `schemas/ddl/schema.sql`.

## Backend

### New package: `backend/internal/exchangerates`

Handler with routes:

- `GET /api/exchange-rates` — list all exchange rates.
- `POST /api/exchange-rates` — upsert a rate. Body: `{"base_currency": "USD", "target_currency": "EUR", "rate": 1.08}`. Upserts on the `(base_currency, target_currency)` unique constraint.
- `DELETE /api/exchange-rates/{id}` — delete a rate by ID.
- `POST /api/exchange-rates/fetch` — fetch current rates from `https://api.frankfurter.app/latest?base={currency}` for a given base currency (body: `{"base_currency": "USD"}`). Upserts all returned rates into the table. Returns the number of rates updated. Fails gracefully if the external API is unreachable (returns error message, no crash).

All routes protected by license middleware + auth middleware (same as `/api/assets`).

### New package: `backend/internal/analytics`

Handler with one route:

- `GET /api/analytics/portfolio?currency=USD` — returns portfolio data converted to the requested display currency.

Response shape:
```json
{
  "total_value": 12345.67,
  "currency": "USD",
  "series": [
    {"date": "2026-01-15", "value": 5000.00},
    {"date": "2026-02-01", "value": 7500.00},
    {"date": "2026-03-10", "value": 12345.67}
  ]
}
```

**Portfolio calculation logic:**

1. Fetch all value points for the authenticated user across all assets.
2. For each value point, convert to the display currency:
   - If the value point's currency matches the display currency, use the value as-is.
   - Otherwise, look up the exchange rate from the value point's currency to the display currency. If no direct rate exists, try the inverse (display→value currency, use 1/rate). If neither exists, skip the value point.
3. **Total value**: for each asset, take the latest value point (by timestamp) and sum across all assets.
4. **Series**: group all converted value points by date (day granularity). For each date, sum the latest-known value of each asset as of that date (carry forward the most recent value point for assets that don't have a point on that specific date). This produces a running portfolio total over time.

Protected by license middleware + auth middleware.

## Frontend

### New dependency

- `recharts` — added to `package.json` for the line chart.

### New component: NavBar

A horizontal navigation bar rendered on all protected pages. Contains:

- "Assets" link → `/assets`
- "Analytics" link → `/analytics`
- "Exchange Rates" link → `/exchange-rates`
- Logout button (moved from AssetList header)

Implemented as a shared component used by `App.jsx` wrapping protected routes. The existing AssetList header loses its Logout button (moved to nav bar). Support bundle button stays on AssetList.

### New page: Analytics (`/analytics`)

- Route: `/analytics` (protected)
- Currency dropdown at top, defaults to USD. Options populated from currencies found in the user's value points (fetched from a new lightweight endpoint or derived client-side from the analytics response).
- Summary card: "Total Portfolio Value: $12,345.67" in the selected currency.
- Line chart (Recharts `LineChart` inside `ResponsiveContainer`):
  - X-axis: dates
  - Y-axis: portfolio value
  - Single line showing total portfolio value over time
  - Tooltip showing date and formatted value
- Empty state: message when no value points exist or no exchange rates are configured for conversion.
- Calls `GET /api/analytics/portfolio?currency={selected}` on mount and when currency changes.

### New page: Exchange Rates (`/exchange-rates`)

- Route: `/exchange-rates` (protected)
- Table showing all exchange rates: base currency, target currency, rate, last updated.
- "Add Rate" button opens a form: base currency input (3-char), target currency input (3-char), rate input (numeric).
- Edit button on each row to modify the rate.
- Delete button on each row with confirmation.
- "Fetch Current Rates" button:
  - Shows a base currency input (defaults to USD).
  - Calls `POST /api/exchange-rates/fetch` with the base currency.
  - On success, refreshes the table and shows "X rates updated" message.
  - On failure (e.g., air-gapped), shows error message.

### API client additions (`frontend/src/api.js`)

- `listExchangeRates()` — GET /api/exchange-rates
- `upsertExchangeRate(baseCurrency, targetCurrency, rate)` — POST /api/exchange-rates
- `deleteExchangeRate(id)` — DELETE /api/exchange-rates/{id}
- `fetchExchangeRates(baseCurrency)` — POST /api/exchange-rates/fetch
- `getPortfolioAnalytics(currency)` — GET /api/analytics/portfolio?currency={currency}

### CSS

Styles for the nav bar and analytics page added to `App.css`. Follow existing patterns (simple utility classes, no CSS modules). Chart container gets a white background card matching existing form styling.

## Route structure update

```
/login
/register
/verify-email
/license-expired
/assets              (protected, nav bar)
/assets/:id          (protected, nav bar)
/analytics           (protected, nav bar)
/exchange-rates      (protected, nav bar)
```

## What's NOT in scope

- License entitlement gating (separate task)
- Real-time exchange rate auto-refresh
- Multiple chart types or date range filtering
- Per-user exchange rate preferences
- Export/download functionality
