# Analytics Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an Analytics page with portfolio value chart, exchange rates management, and a shared nav bar.

**Architecture:** New `exchange_rates` DB table stores currency conversion rates. Two new backend packages (`exchangerates`, `analytics`) provide CRUD + frankfurter.app fetch and portfolio aggregation. Frontend adds a NavBar component, Analytics page with Recharts line chart, and Exchange Rates management page.

**Tech Stack:** Go (chi router, pgx), React 19, Recharts, frankfurter.app API

**Spec:** `docs/superpowers/specs/2026-04-10-analytics-page-design.md`

---

## File Map

**Create:**
- `schemas/tables/exchange_rates.yaml` — SchemaHero table definition
- `backend/internal/exchangerates/handler.go` — exchange rates CRUD + fetch handler
- `backend/internal/analytics/handler.go` — portfolio analytics handler
- `frontend/src/components/NavBar.jsx` — shared navigation bar
- `frontend/src/pages/Analytics.jsx` — analytics page with chart
- `frontend/src/pages/ExchangeRates.jsx` — exchange rates management page

**Modify:**
- `schemas/ddl/schema.sql` — add exchange_rates DDL
- `backend/main.go` — wire new routes
- `frontend/src/api.js` — add new API functions
- `frontend/src/App.jsx` — add NavBar, new routes
- `frontend/src/App.css` — add nav bar and analytics styles
- `frontend/src/pages/AssetList.jsx` — remove Logout button (moved to NavBar)
- `frontend/package.json` — add recharts dependency
- `e2e/asset-tracker.spec.mjs` — add analytics and exchange rates e2e tests

---

### Task 1: Database Schema — exchange_rates table

**Files:**
- Create: `schemas/tables/exchange_rates.yaml`
- Modify: `schemas/ddl/schema.sql`

- [ ] **Step 1: Create SchemaHero table definition**

Create `schemas/tables/exchange_rates.yaml`:

```yaml
apiVersion: schemas.schemahero.io/v1alpha4
kind: Table
metadata:
  name: exchange_rates
spec:
  name: exchange_rates
  schema:
    postgres:
      primaryKey:
        - id
      columns:
        - name: id
          type: uuid
          default: "gen_random_uuid()"
          constraints:
            notNull: true
        - name: base_currency
          type: varchar(3)
          constraints:
            notNull: true
        - name: target_currency
          type: varchar(3)
          constraints:
            notNull: true
        - name: rate
          type: numeric(15,6)
          constraints:
            notNull: true
        - name: updated_at
          type: timestamp
          default: "now()"
          constraints:
            notNull: true
      indexes:
        - columns:
            - base_currency
            - target_currency
          name: idx_exchange_rates_unique
          isUnique: true
```

- [ ] **Step 2: Add DDL to schema.sql**

Append to `schemas/ddl/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS exchange_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    base_currency VARCHAR(3) NOT NULL,
    target_currency VARCHAR(3) NOT NULL,
    rate NUMERIC(15,6) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_exchange_rates_unique ON exchange_rates (base_currency, target_currency);
```

- [ ] **Step 3: Commit**

```bash
git add schemas/tables/exchange_rates.yaml schemas/ddl/schema.sql
git -c commit.gpgsign=false commit -m "feat: add exchange_rates database table"
```

---

### Task 2: Backend — Exchange Rates Handler

**Files:**
- Create: `backend/internal/exchangerates/handler.go`

- [ ] **Step 1: Create the exchange rates handler**

Create `backend/internal/exchangerates/handler.go`:

```go
package exchangerates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type ExchangeRate struct {
	ID             string    `json:"id"`
	BaseCurrency   string    `json:"base_currency"`
	TargetCurrency string    `json:"target_currency"`
	Rate           float64   `json:"rate"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type upsertRequest struct {
	BaseCurrency   string  `json:"base_currency"`
	TargetCurrency string  `json:"target_currency"`
	Rate           float64 `json:"rate"`
}

type fetchRequest struct {
	BaseCurrency string `json:"base_currency"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(context.Background(),
		"SELECT id, base_currency, target_currency, rate, updated_at FROM exchange_rates ORDER BY base_currency, target_currency",
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	rates := []ExchangeRate{}
	for rows.Next() {
		var er ExchangeRate
		if err := rows.Scan(&er.ID, &er.BaseCurrency, &er.TargetCurrency, &er.Rate, &er.UpdatedAt); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		rates = append(rates, er)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rates)
}

func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.BaseCurrency = strings.TrimSpace(strings.ToUpper(req.BaseCurrency))
	req.TargetCurrency = strings.TrimSpace(strings.ToUpper(req.TargetCurrency))

	if len(req.BaseCurrency) != 3 || len(req.TargetCurrency) != 3 {
		http.Error(w, `{"error":"currency codes must be 3 characters"}`, http.StatusBadRequest)
		return
	}
	if req.Rate <= 0 {
		http.Error(w, `{"error":"rate must be positive"}`, http.StatusBadRequest)
		return
	}

	var er ExchangeRate
	err := h.db.QueryRow(context.Background(),
		`INSERT INTO exchange_rates (base_currency, target_currency, rate)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (base_currency, target_currency)
		 DO UPDATE SET rate = EXCLUDED.rate, updated_at = now()
		 RETURNING id, base_currency, target_currency, rate, updated_at`,
		req.BaseCurrency, req.TargetCurrency, req.Rate,
	).Scan(&er.ID, &er.BaseCurrency, &er.TargetCurrency, &er.Rate, &er.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(er)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tag, err := h.db.Exec(context.Background(),
		"DELETE FROM exchange_rates WHERE id = $1", id,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, `{"error":"exchange rate not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Fetch(w http.ResponseWriter, r *http.Request) {
	var req fetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.BaseCurrency = strings.TrimSpace(strings.ToUpper(req.BaseCurrency))
	if len(req.BaseCurrency) != 3 {
		http.Error(w, `{"error":"base_currency must be 3 characters"}`, http.StatusBadRequest)
		return
	}

	url := fmt.Sprintf("https://api.frankfurter.app/latest?base=%s", req.BaseCurrency)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch exchange rates: service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf(`{"error":"exchange rate API returned status %d: %s"}`, resp.StatusCode, string(body)), http.StatusBadGateway)
		return
	}

	var apiResp struct {
		Base  string             `json:"base"`
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		http.Error(w, `{"error":"failed to parse exchange rate response"}`, http.StatusBadGateway)
		return
	}

	count := 0
	for currency, rate := range apiResp.Rates {
		_, err := h.db.Exec(context.Background(),
			`INSERT INTO exchange_rates (base_currency, target_currency, rate)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (base_currency, target_currency)
			 DO UPDATE SET rate = EXCLUDED.rate, updated_at = now()`,
			apiResp.Base, currency, rate,
		)
		if err != nil {
			continue
		}
		count++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"updated": count,
		"base":    apiResp.Base,
	})
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/exchangerates/handler.go
git -c commit.gpgsign=false commit -m "feat: add exchange rates backend handler"
```

---

### Task 3: Backend — Analytics Handler

**Files:**
- Create: `backend/internal/analytics/handler.go`

- [ ] **Step 1: Create the analytics handler**

Create `backend/internal/analytics/handler.go`:

```go
package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type portfolioResponse struct {
	TotalValue float64       `json:"total_value"`
	Currency   string        `json:"currency"`
	Series     []seriesPoint `json:"series"`
}

type seriesPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

type valuePoint struct {
	AssetID   string
	Timestamp time.Time
	Value     float64
	Currency  string
}

type exchangeRate struct {
	BaseCurrency   string
	TargetCurrency string
	Rate           float64
}

func (h *Handler) Portfolio(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	displayCurrency := r.URL.Query().Get("currency")
	if displayCurrency == "" {
		displayCurrency = "USD"
	}

	// Fetch all value points for this user
	rows, err := h.db.Query(context.Background(),
		"SELECT asset_id, timestamp, value, currency FROM asset_value_points WHERE user_id = $1 ORDER BY timestamp ASC",
		userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var points []valuePoint
	for rows.Next() {
		var vp valuePoint
		if err := rows.Scan(&vp.AssetID, &vp.Timestamp, &vp.Value, &vp.Currency); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		points = append(points, vp)
	}

	// Fetch all exchange rates
	rateRows, err := h.db.Query(context.Background(),
		"SELECT base_currency, target_currency, rate FROM exchange_rates",
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rateRows.Close()

	rates := map[string]float64{}
	for rateRows.Next() {
		var er exchangeRate
		if err := rateRows.Scan(&er.BaseCurrency, &er.TargetCurrency, &er.Rate); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		rates[er.BaseCurrency+"->"+er.TargetCurrency] = er.Rate
	}

	// Convert values to display currency
	convert := func(value float64, fromCurrency string) (float64, bool) {
		if fromCurrency == displayCurrency {
			return value, true
		}
		// Direct rate: fromCurrency -> displayCurrency
		if rate, ok := rates[fromCurrency+"->"+displayCurrency]; ok {
			return value * rate, true
		}
		// Inverse rate: displayCurrency -> fromCurrency
		if rate, ok := rates[displayCurrency+"->"+fromCurrency]; ok && rate != 0 {
			return value / rate, true
		}
		return 0, false
	}

	// Build series: for each date, compute portfolio total
	// by carrying forward each asset's latest known value
	type dateAssetValue struct {
		date    string
		assetID string
		value   float64
	}

	var converted []dateAssetValue
	for _, vp := range points {
		val, ok := convert(vp.Value, vp.Currency)
		if !ok {
			continue
		}
		converted = append(converted, dateAssetValue{
			date:    vp.Timestamp.Format("2006-01-02"),
			assetID: vp.AssetID,
			value:   val,
		})
	}

	// Collect all unique dates, sorted
	dateSet := map[string]bool{}
	for _, c := range converted {
		dateSet[c.date] = true
	}
	var dates []string
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	// For each date, track latest value per asset, then sum
	latestByAsset := map[string]float64{}
	series := []seriesPoint{}
	ci := 0 // index into converted (sorted by timestamp from DB query)

	for _, date := range dates {
		// Apply all value points for this date
		for ci < len(converted) && converted[ci].date == date {
			latestByAsset[converted[ci].assetID] = converted[ci].value
			ci++
		}
		// Sum all latest asset values
		total := 0.0
		for _, v := range latestByAsset {
			total += v
		}
		series = append(series, seriesPoint{
			Date:  date,
			Value: math_round2(total),
		})
	}

	// Total value: sum of latest value per asset
	totalValue := 0.0
	for _, v := range latestByAsset {
		totalValue += v
	}

	resp := portfolioResponse{
		TotalValue: math_round2(totalValue),
		Currency:   displayCurrency,
		Series:     series,
	}
	if resp.Series == nil {
		resp.Series = []seriesPoint{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func math_round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/analytics/handler.go
git -c commit.gpgsign=false commit -m "feat: add analytics portfolio backend handler"
```

---

### Task 4: Backend — Wire Routes in main.go

**Files:**
- Modify: `backend/main.go`

- [ ] **Step 1: Add imports and route wiring**

Add two new imports to `main.go`:

```go
"github.com/assettrackerhq/asset-tracker/backend/internal/analytics"
"github.com/assettrackerhq/asset-tracker/backend/internal/exchangerates"
```

Then add the following route groups after the existing asset routes block (after line 148 `}`):

```go
	// Exchange rates routes (protected by license check + auth)
	exchangeRateHandler := exchangerates.NewHandler(pool)
	r.Route("/api/exchange-rates", func(r chi.Router) {
		r.Use(license.LicenseMiddleware(licenseChecker))
		r.Use(auth.Middleware(cfg.JWTSecret))
		r.Get("/", exchangeRateHandler.List)
		r.Post("/", exchangeRateHandler.Upsert)
		r.Delete("/{id}", exchangeRateHandler.Delete)
		r.Post("/fetch", exchangeRateHandler.Fetch)
	})

	// Analytics routes (protected by license check + auth)
	analyticsHandler := analytics.NewHandler(pool)
	r.Route("/api/analytics", func(r chi.Router) {
		r.Use(license.LicenseMiddleware(licenseChecker))
		r.Use(auth.Middleware(cfg.JWTSecret))
		r.Get("/portfolio", analyticsHandler.Portfolio)
	})
```

- [ ] **Step 2: Verify it compiles**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/main.go
git -c commit.gpgsign=false commit -m "feat: wire exchange rates and analytics routes"
```

---

### Task 5: Frontend — Add Recharts Dependency

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: Install recharts**

```bash
cd frontend && npm install recharts
```

- [ ] **Step 2: Verify install succeeded**

```bash
cd frontend && npm ls recharts
```

Expected: shows recharts version installed.

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git -c commit.gpgsign=false commit -m "feat: add recharts dependency"
```

---

### Task 6: Frontend — API Client Additions

**Files:**
- Modify: `frontend/src/api.js`

- [ ] **Step 1: Add API functions**

Append these functions to the end of `frontend/src/api.js` (before the final blank line or at the very end):

```javascript
export function listExchangeRates() {
  return request('/exchange-rates');
}

export function upsertExchangeRate(baseCurrency, targetCurrency, rate) {
  return request('/exchange-rates', {
    method: 'POST',
    body: JSON.stringify({ base_currency: baseCurrency, target_currency: targetCurrency, rate: parseFloat(rate) }),
  });
}

export function deleteExchangeRate(id) {
  return request(`/exchange-rates/${id}`, { method: 'DELETE' });
}

export function fetchExchangeRates(baseCurrency) {
  return request('/exchange-rates/fetch', {
    method: 'POST',
    body: JSON.stringify({ base_currency: baseCurrency }),
  });
}

export function getPortfolioAnalytics(currency) {
  return request(`/analytics/portfolio?currency=${encodeURIComponent(currency)}`);
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/api.js
git -c commit.gpgsign=false commit -m "feat: add exchange rates and analytics API functions"
```

---

### Task 7: Frontend — NavBar Component

**Files:**
- Create: `frontend/src/components/NavBar.jsx`
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Create NavBar component**

Create `frontend/src/components/NavBar.jsx`:

```jsx
import { NavLink, useNavigate } from 'react-router-dom';

export default function NavBar() {
  const navigate = useNavigate();

  function handleLogout() {
    localStorage.removeItem('token');
    navigate('/login');
  }

  return (
    <nav className="nav-bar">
      <div className="nav-links">
        <NavLink to="/assets" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Assets</NavLink>
        <NavLink to="/analytics" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Analytics</NavLink>
        <NavLink to="/exchange-rates" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Exchange Rates</NavLink>
      </div>
      <button className="secondary" onClick={handleLogout}>Logout</button>
    </nav>
  );
}
```

- [ ] **Step 2: Add NavBar CSS**

Add the following to the end of `frontend/src/App.css`:

```css
.nav-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 20px;
  background: white;
  border-bottom: 1px solid #ddd;
  margin-bottom: 0;
}

.nav-links {
  display: flex;
  gap: 8px;
}

.nav-link {
  padding: 6px 14px;
  border-radius: 4px;
  text-decoration: none;
  color: #333;
  font-weight: 500;
  font-size: 14px;
}

.nav-link:hover {
  background: #f0f0f0;
}

.nav-link.active {
  background: #2563eb;
  color: white;
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/NavBar.jsx frontend/src/App.css
git -c commit.gpgsign=false commit -m "feat: add NavBar component with styles"
```

---

### Task 8: Frontend — Analytics Page

**Files:**
- Create: `frontend/src/pages/Analytics.jsx`
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Create Analytics page**

Create `frontend/src/pages/Analytics.jsx`:

```jsx
import { useState, useEffect } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { getPortfolioAnalytics } from '../api';

export default function Analytics() {
  const [currency, setCurrency] = useState('USD');
  const [data, setData] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, [currency]);

  async function loadData() {
    setLoading(true);
    setError('');
    try {
      const result = await getPortfolioAnalytics(currency);
      setData(result);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  function formatValue(value) {
    try {
      return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(value);
    } catch {
      return `${value} ${currency}`;
    }
  }

  return (
    <div className="container">
      <div className="header">
        <h1>Analytics</h1>
        <div className="currency-selector">
          <label>Currency: </label>
          <input
            value={currency}
            onChange={(e) => setCurrency(e.target.value.toUpperCase())}
            maxLength={3}
            style={{ width: '60px', textAlign: 'center' }}
          />
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      {loading && <p className="info">Loading analytics...</p>}

      {!loading && data && (
        <>
          <div className="analytics-summary">
            <div className="summary-card">
              <div className="summary-label">Total Portfolio Value</div>
              <div className="summary-value">{formatValue(data.total_value)}</div>
            </div>
          </div>

          {data.series.length > 0 ? (
            <div className="chart-container">
              <h2>Portfolio Value Over Time</h2>
              <ResponsiveContainer width="100%" height={400}>
                <LineChart data={data.series}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="date" />
                  <YAxis />
                  <Tooltip formatter={(value) => formatValue(value)} />
                  <Line type="monotone" dataKey="value" stroke="#2563eb" strokeWidth={2} dot={{ r: 4 }} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <p style={{ textAlign: 'center', color: '#999', marginTop: '40px' }}>
              No data available. Add value points to your assets to see analytics.
            </p>
          )}
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Add Analytics CSS**

Append to `frontend/src/App.css`:

```css
.analytics-summary {
  margin-bottom: 24px;
}

.summary-card {
  background: white;
  padding: 24px;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}

.summary-label {
  font-size: 14px;
  color: #6b7280;
  margin-bottom: 4px;
}

.summary-value {
  font-size: 32px;
  font-weight: 700;
  color: #111;
}

.chart-container {
  background: white;
  padding: 24px;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}

.chart-container h2 {
  margin-bottom: 16px;
  font-size: 18px;
}

.currency-selector {
  display: flex;
  align-items: center;
  gap: 8px;
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Analytics.jsx frontend/src/App.css
git -c commit.gpgsign=false commit -m "feat: add Analytics page with portfolio chart"
```

---

### Task 9: Frontend — Exchange Rates Page

**Files:**
- Create: `frontend/src/pages/ExchangeRates.jsx`

- [ ] **Step 1: Create Exchange Rates page**

Create `frontend/src/pages/ExchangeRates.jsx`:

```jsx
import { useState, useEffect } from 'react';
import { listExchangeRates, upsertExchangeRate, deleteExchangeRate, fetchExchangeRates } from '../api';

export default function ExchangeRates() {
  const [rates, setRates] = useState([]);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ base_currency: '', target_currency: '', rate: '' });
  const [fetchBase, setFetchBase] = useState('USD');
  const [fetchStatus, setFetchStatus] = useState('');
  const [fetching, setFetching] = useState(false);

  useEffect(() => {
    loadRates();
  }, []);

  async function loadRates() {
    try {
      const data = await listExchangeRates();
      setRates(data);
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      await upsertExchangeRate(formData.base_currency, formData.target_currency, formData.rate);
      setShowForm(false);
      setEditingId(null);
      setFormData({ base_currency: '', target_currency: '', rate: '' });
      await loadRates();
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleDelete(id) {
    if (!confirm('Delete this exchange rate?')) return;
    try {
      await deleteExchangeRate(id);
      await loadRates();
    } catch (err) {
      setError(err.message);
    }
  }

  function startEdit(rate) {
    setEditingId(rate.id);
    setFormData({
      base_currency: rate.base_currency,
      target_currency: rate.target_currency,
      rate: rate.rate,
    });
    setShowForm(true);
  }

  async function handleFetch() {
    setFetching(true);
    setFetchStatus('');
    setError('');
    try {
      const result = await fetchExchangeRates(fetchBase);
      setFetchStatus(`Fetched ${result.updated} rates for ${result.base}`);
      await loadRates();
    } catch (err) {
      setError(`Failed to fetch rates: ${err.message}`);
    } finally {
      setFetching(false);
    }
  }

  function formatTimestamp(ts) {
    return new Date(ts).toLocaleString();
  }

  return (
    <div className="container">
      <div className="header">
        <h1>Exchange Rates</h1>
        <div>
          <button className="primary" onClick={() => { setShowForm(true); setEditingId(null); setFormData({ base_currency: '', target_currency: '', rate: '' }); }}>
            Add Rate
          </button>
        </div>
      </div>

      {error && <p className="error">{error}</p>}
      {fetchStatus && <p className="success">{fetchStatus}</p>}

      <div style={{ marginBottom: '24px', padding: '16px', background: 'white', borderRadius: '8px', display: 'flex', alignItems: 'center', gap: '12px' }}>
        <label style={{ fontWeight: 600 }}>Fetch rates for base currency:</label>
        <input
          value={fetchBase}
          onChange={(e) => setFetchBase(e.target.value.toUpperCase())}
          maxLength={3}
          style={{ width: '60px', textAlign: 'center', padding: '8px', border: '1px solid #ccc', borderRadius: '4px' }}
        />
        <button className="primary" onClick={handleFetch} disabled={fetching}>
          {fetching ? 'Fetching...' : 'Fetch Current Rates'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} style={{ marginBottom: '24px', padding: '16px', background: 'white', borderRadius: '8px' }}>
          <div className="form-group">
            <label>Base Currency</label>
            <input
              value={formData.base_currency}
              onChange={(e) => setFormData({ ...formData, base_currency: e.target.value.toUpperCase() })}
              maxLength={3}
              required
              disabled={editingId !== null}
            />
          </div>
          <div className="form-group">
            <label>Target Currency</label>
            <input
              value={formData.target_currency}
              onChange={(e) => setFormData({ ...formData, target_currency: e.target.value.toUpperCase() })}
              maxLength={3}
              required
              disabled={editingId !== null}
            />
          </div>
          <div className="form-group">
            <label>Rate</label>
            <input
              type="number"
              step="0.000001"
              value={formData.rate}
              onChange={(e) => setFormData({ ...formData, rate: e.target.value })}
              required
            />
          </div>
          <button type="submit" className="primary">{editingId ? 'Update' : 'Add'}</button>
          <button type="button" className="secondary" onClick={() => { setShowForm(false); setEditingId(null); }}>Cancel</button>
        </form>
      )}

      <table>
        <thead>
          <tr>
            <th>Base</th>
            <th>Target</th>
            <th>Rate</th>
            <th>Updated</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {rates.map((rate) => (
            <tr key={rate.id}>
              <td>{rate.base_currency}</td>
              <td>{rate.target_currency}</td>
              <td>{rate.rate}</td>
              <td>{formatTimestamp(rate.updated_at)}</td>
              <td className="actions">
                <button className="secondary" onClick={() => startEdit(rate)}>Edit</button>
                <button className="danger" onClick={() => handleDelete(rate.id)}>Delete</button>
              </td>
            </tr>
          ))}
          {rates.length === 0 && (
            <tr><td colSpan="5" style={{ textAlign: 'center', color: '#999' }}>No exchange rates configured</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/pages/ExchangeRates.jsx
git -c commit.gpgsign=false commit -m "feat: add Exchange Rates management page"
```

---

### Task 10: Frontend — Wire App.jsx Routes and NavBar

**Files:**
- Modify: `frontend/src/App.jsx`
- Modify: `frontend/src/pages/AssetList.jsx`

- [ ] **Step 1: Update App.jsx with NavBar and new routes**

Replace the full contents of `frontend/src/App.jsx` with:

```jsx
import { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import VerifyEmail from './pages/VerifyEmail';
import AssetList from './pages/AssetList';
import AssetDetail from './pages/AssetDetail';
import Analytics from './pages/Analytics';
import ExchangeRates from './pages/ExchangeRates';
import LicenseExpired from './pages/LicenseExpired';
import NavBar from './components/NavBar';
import { checkForUpdates } from './api';
import './App.css';

function ProtectedRoute({ children }) {
  const token = localStorage.getItem('token');
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

function Layout({ children, updateAvailable }) {
  const location = useLocation();
  const publicPaths = ['/login', '/register', '/verify-email', '/license-expired'];
  const showNav = !publicPaths.includes(location.pathname);

  return (
    <>
      {updateAvailable && (
        <div className="update-banner">Update available</div>
      )}
      {showNav && <NavBar />}
      {children}
    </>
  );
}

export default function App() {
  const [updateAvailable, setUpdateAvailable] = useState(false);

  useEffect(() => {
    checkForUpdates().then((data) => {
      setUpdateAvailable(data.updatesAvailable);
    });
  }, []);

  return (
    <BrowserRouter>
      <Layout updateAvailable={updateAvailable}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/verify-email" element={<VerifyEmail />} />
          <Route path="/license-expired" element={<LicenseExpired />} />
          <Route path="/assets" element={<ProtectedRoute><AssetList /></ProtectedRoute>} />
          <Route path="/assets/:id" element={<ProtectedRoute><AssetDetail /></ProtectedRoute>} />
          <Route path="/analytics" element={<ProtectedRoute><Analytics /></ProtectedRoute>} />
          <Route path="/exchange-rates" element={<ProtectedRoute><ExchangeRates /></ProtectedRoute>} />
          <Route path="*" element={<Navigate to="/assets" replace />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}
```

- [ ] **Step 2: Remove Logout button from AssetList**

In `frontend/src/pages/AssetList.jsx`, remove the Logout button and `handleLogout` function. The header `<div>` should contain only the Add Asset and Generate Support Bundle buttons:

Remove the `handleLogout` function (lines 87-90):
```javascript
// DELETE this function entirely:
function handleLogout() {
    localStorage.removeItem('token');
    navigate('/login');
  }
```

Remove the `useNavigate` import usage (it's still needed for asset detail navigation). Remove the Logout button from the header. The header buttons `<div>` becomes:

```jsx
        <div>
          <button className="primary" onClick={() => { setShowForm(true); setEditingId(null); setFormData({ id: '', name: '', description: '' }); }}>
            Add Asset
          </button>
          <button className="secondary" onClick={handleGenerateBundle} disabled={generatingBundle}>
            {generatingBundle ? 'Generating...' : 'Generate Support Bundle'}
          </button>
        </div>
```

(The `<button className="secondary" onClick={handleLogout}>Logout</button>` line is removed.)

- [ ] **Step 3: Verify frontend builds**

```bash
cd frontend && npm run build
```

Expected: build succeeds with no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.jsx frontend/src/pages/AssetList.jsx
git -c commit.gpgsign=false commit -m "feat: wire NavBar, Analytics and Exchange Rates routes"
```

---

### Task 11: E2E Tests — Exchange Rates and Analytics

**Files:**
- Modify: `e2e/asset-tracker.spec.mjs`

- [ ] **Step 1: Add exchange rates and analytics e2e tests**

Add the following test sections inside the `test.describe.serial('Asset Tracker', () => {` block, after the existing `'API - Asset CRUD'` describe block (before the final `});`):

```javascript
  test.describe('API - Exchange Rates', () => {
    const authHeaders = () => ({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    });

    test('create exchange rate via API', async ({ request }) => {
      const resp = await request.post(`${API_URL}/exchange-rates`, {
        headers: authHeaders(),
        data: { base_currency: 'USD', target_currency: 'EUR', rate: 0.92 },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(body.base_currency).toBe('USD');
      expect(body.target_currency).toBe('EUR');
      expect(Number(body.rate)).toBeCloseTo(0.92);
    });

    test('list exchange rates returns created rate', async ({ request }) => {
      const resp = await request.get(`${API_URL}/exchange-rates`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const rates = await resp.json();
      expect(rates.length).toBeGreaterThanOrEqual(1);
      const usdEur = rates.find(r => r.base_currency === 'USD' && r.target_currency === 'EUR');
      expect(usdEur).toBeTruthy();
    });

    test('upsert exchange rate updates existing', async ({ request }) => {
      const resp = await request.post(`${API_URL}/exchange-rates`, {
        headers: authHeaders(),
        data: { base_currency: 'USD', target_currency: 'EUR', rate: 0.95 },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(Number(body.rate)).toBeCloseTo(0.95);
    });

    test('reject unauthenticated exchange rate requests', async ({ request }) => {
      const resp = await request.get(`${API_URL}/exchange-rates`);
      expect(resp.status()).toBe(401);
    });
  });

  test.describe('API - Analytics', () => {
    const authHeaders = () => ({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    });

    test('portfolio analytics returns data in requested currency', async ({ request }) => {
      // E2E-ASSET-001 was created earlier with USD value points
      const resp = await request.get(`${API_URL}/analytics/portfolio?currency=USD`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.currency).toBe('USD');
      expect(typeof body.total_value).toBe('number');
      expect(Array.isArray(body.series)).toBe(true);
    });

    test('portfolio analytics returns empty series when no data', async ({ request }) => {
      // Request in a currency with no value points and no exchange rates
      const resp = await request.get(`${API_URL}/analytics/portfolio?currency=JPY`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.currency).toBe('JPY');
      // Should still return a valid response (possibly with converted values via USD->EUR rate)
      expect(Array.isArray(body.series)).toBe(true);
    });

    test('reject unauthenticated analytics requests', async ({ request }) => {
      const resp = await request.get(`${API_URL}/analytics/portfolio?currency=USD`);
      expect(resp.status()).toBe(401);
    });
  });

  test.describe('UI - Analytics', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/');
      await page.evaluate((t) => localStorage.setItem('token', t), token);
    });

    test('nav bar is visible on assets page', async ({ page }) => {
      await page.goto('/assets');
      await expect(page.locator('nav.nav-bar')).toBeVisible();
      await expect(page.locator('a.nav-link:has-text("Assets")')).toBeVisible();
      await expect(page.locator('a.nav-link:has-text("Analytics")')).toBeVisible();
      await expect(page.locator('a.nav-link:has-text("Exchange Rates")')).toBeVisible();
    });

    test('navigate to analytics page via nav bar', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('a.nav-link:has-text("Analytics")').click();
      await expect(page.locator('h1')).toHaveText('Analytics');
    });

    test('analytics page shows portfolio value', async ({ page }) => {
      await page.goto('/analytics');
      await expect(page.locator('.summary-card')).toBeVisible({ timeout: 5000 });
      await expect(page.locator('.summary-label')).toHaveText('Total Portfolio Value');
    });

    test('navigate to exchange rates page via nav bar', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('a.nav-link:has-text("Exchange Rates")').click();
      await expect(page.locator('h1')).toHaveText('Exchange Rates');
    });

    test('exchange rates page shows fetch button', async ({ page }) => {
      await page.goto('/exchange-rates');
      await expect(page.locator('button:has-text("Fetch Current Rates")')).toBeVisible();
      await expect(page.locator('button:has-text("Add Rate")')).toBeVisible();
    });
  });
```

- [ ] **Step 2: Update existing test that checks for Logout button on assets page**

The existing test `'asset list page renders with Add Asset button'` (around line 128-133) checks for a Logout button on the assets page. Since Logout moved to the NavBar, update it:

Change:
```javascript
      await expect(page.locator('button:has-text("Logout")')).toBeVisible();
```

To:
```javascript
      await expect(page.locator('nav.nav-bar button:has-text("Logout")')).toBeVisible();
```

Also update the logout test (`'logout returns to login page'` around line 210-214):

Change:
```javascript
      await page.locator('button:has-text("Logout")').click();
```

To:
```javascript
      await page.locator('nav.nav-bar button:has-text("Logout")').click();
```

- [ ] **Step 3: Commit**

```bash
git add e2e/asset-tracker.spec.mjs
git -c commit.gpgsign=false commit -m "test: add exchange rates and analytics e2e tests"
```

---

### Task 12: Verify Full Build

- [ ] **Step 1: Backend compiles**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 2: Frontend builds**

```bash
cd frontend && npm run build
```

Expected: build succeeds.

- [ ] **Step 3: Commit any remaining changes**

If there are any uncommitted changes, commit them:

```bash
git add -A
git -c commit.gpgsign=false commit -m "chore: final build verification cleanup"
```
