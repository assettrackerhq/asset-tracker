# Plaid & Teller Bank Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Plaid and Teller bank account linking as two independently toggleable features configured via the KOTS config screen, allowing users to import bank accounts as assets.

**Architecture:** Shared `BankProvider` interface with Plaid and Teller implementations. Backend handlers expose REST endpoints for link token creation, account connection, listing, syncing, and unlinking. Frontend adds a new LinkedAccounts page with conditional Plaid/Teller Link buttons. Features are toggled via KOTS config → HelmChart CR optionalValues → Helm values → backend env vars.

**Tech Stack:** Go (chi router, pgxpool), Plaid Go SDK (`github.com/plaid/plaid-go/v29`), Teller REST API (direct HTTP), React (`react-plaid-link`, `teller-connect-react`), SchemaHero migrations, KOTS Config.

**Spec:** `docs/superpowers/specs/2026-04-13-plaid-teller-integration-design.md`

---

## File Structure

### New Files
- `backend/internal/banking/types.go` — BankProvider interface, Account type, shared constants
- `backend/internal/banking/plaid.go` — Plaid provider implementation
- `backend/internal/banking/teller.go` — Teller provider implementation
- `backend/internal/banking/handler.go` — HTTP handlers for banking endpoints
- `backend/internal/banking/sync.go` — Daily sync goroutine
- `frontend/src/pages/LinkedAccounts.jsx` — Linked accounts page with Plaid/Teller Link buttons
- `schemas/tables/assets_banking_columns.yaml` — SchemaHero migration for new columns

### Modified Files
- `backend/internal/config/config.go` — Add Plaid/Teller config fields
- `backend/main.go` — Wire banking routes, feature endpoint, daily sync
- `backend/go.mod` / `backend/go.sum` — Add plaid-go dependency
- `frontend/src/api.js` — Add banking API functions
- `frontend/src/App.jsx` — Add LinkedAccounts route, pass feature flags
- `frontend/src/components/NavBar.jsx` — Add Linked Accounts nav link
- `frontend/package.json` — Add react-plaid-link, teller-connect-react
- `helm/asset-tracker/values.yaml` — Add backend.plaid.* and backend.teller.* sections
- `helm/asset-tracker/values.schema.json` — Add schema for new values
- `helm/asset-tracker/templates/backend-deployment.yaml` — Add Plaid/Teller env vars
- `replicated/config.yaml` — Add Integrations group with Plaid/Teller toggles
- `replicated/helmchart-asset-tracker.yaml` — Add optionalValues for Plaid/Teller
- `e2e/asset-tracker.spec.mjs` — Add linked accounts visibility tests

---

## Task 1: Database Schema Migration

**Files:**
- Modify: `schemas/tables/assets.yaml`

- [ ] **Step 1: Update the SchemaHero assets table definition**

Add the new columns to the existing assets table schema. SchemaHero handles migrations declaratively — we just update the desired state.

Edit `schemas/tables/assets.yaml` to add four new columns after the existing ones:

```yaml
apiVersion: schemas.schemahero.io/v1alpha4
kind: Table
metadata:
  name: assets
spec:
  database: asset-tracker-db
  name: assets
  schema:
    postgres:
      primaryKey:
        - id
        - user_id
      columns:
        - name: id
          type: varchar(50)
          constraints:
            notNull: true
        - name: user_id
          type: uuid
          constraints:
            notNull: true
        - name: name
          type: varchar(255)
          constraints:
            notNull: true
        - name: description
          type: text
        - name: created_at
          type: timestamp
          default: "now()"
          constraints:
            notNull: true
        - name: updated_at
          type: timestamp
          default: "now()"
          constraints:
            notNull: true
        - name: source
          type: varchar(20)
          default: "'manual'"
          constraints:
            notNull: true
        - name: external_id
          type: text
        - name: access_token
          type: text
        - name: institution
          type: text
```

- [ ] **Step 2: Also add the DDL to the SQL file for reference**

Check if there's a `schemas/ddl/` directory with SQL files. If so, add the corresponding ALTER statements. If not, skip this step.

Run: `ls schemas/`

- [ ] **Step 3: Commit**

```bash
git add schemas/tables/assets.yaml
git -c commit.gpgsign=false commit -m "schema: add source, external_id, access_token, institution columns to assets"
```

---

## Task 2: Backend Config

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Add Plaid and Teller fields to Config struct**

Add these fields after the existing `SupportBundleImagePullSecrets` field:

```go
PlaidEnabled        bool
PlaidClientID       string
PlaidSecret         string
PlaidEnvironment    string
TellerEnabled       bool
TellerApplicationID string
TellerEnvironment   string
```

- [ ] **Step 2: Load Plaid/Teller config from environment variables**

Add this code in the `Load()` function, before the final `return &Config{` block:

```go
plaidEnabled := false
if v := os.Getenv("PLAID_ENABLED"); v != "" {
	plaidEnabled = strings.EqualFold(v, "true") || v == "1"
}

plaidEnvironment := os.Getenv("PLAID_ENVIRONMENT")
if plaidEnvironment == "" {
	plaidEnvironment = "sandbox"
}

tellerEnabled := false
if v := os.Getenv("TELLER_ENABLED"); v != "" {
	tellerEnabled = strings.EqualFold(v, "true") || v == "1"
}

tellerEnvironment := os.Getenv("TELLER_ENVIRONMENT")
if tellerEnvironment == "" {
	tellerEnvironment = "sandbox"
}
```

- [ ] **Step 3: Include new fields in the return struct**

Add to the return statement in `Load()`:

```go
PlaidEnabled:        plaidEnabled,
PlaidClientID:       os.Getenv("PLAID_CLIENT_ID"),
PlaidSecret:         os.Getenv("PLAID_SECRET"),
PlaidEnvironment:    plaidEnvironment,
TellerEnabled:       tellerEnabled,
TellerApplicationID: os.Getenv("TELLER_APPLICATION_ID"),
TellerEnvironment:   tellerEnvironment,
```

- [ ] **Step 4: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: Success, no errors.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/config/config.go
git -c commit.gpgsign=false commit -m "feat: add Plaid and Teller config fields"
```

---

## Task 3: Banking Package — Types and Interface

**Files:**
- Create: `backend/internal/banking/types.go`

- [ ] **Step 1: Create the banking package with types and interface**

Create `backend/internal/banking/types.go`:

```go
package banking

import "context"

// BankProvider defines the interface for bank account linking providers.
type BankProvider interface {
	Name() string
	CreateLinkToken(ctx context.Context, userID string) (string, error)
	ExchangeToken(ctx context.Context, publicToken string) (string, error)
	FetchAccounts(ctx context.Context, accessToken string) ([]Account, error)
}

// Account represents a bank account fetched from a provider.
type Account struct {
	ExternalID  string  `json:"external_id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
	Institution string  `json:"institution"`
}

// LinkedAccount represents a bank account stored in the database.
type LinkedAccount struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Source      string  `json:"source"`
	ExternalID  *string `json:"external_id"`
	Institution *string `json:"institution"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
	UpdatedAt   string  `json:"updated_at"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: Success.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/banking/types.go
git -c commit.gpgsign=false commit -m "feat: add banking package types and BankProvider interface"
```

---

## Task 4: Banking Package — Plaid Provider

**Files:**
- Modify: `backend/go.mod`
- Create: `backend/internal/banking/plaid.go`

- [ ] **Step 1: Add Plaid Go SDK dependency**

Run:
```bash
cd backend && go get github.com/plaid/plaid-go/v29
```

- [ ] **Step 2: Create the Plaid provider implementation**

Create `backend/internal/banking/plaid.go`:

```go
package banking

import (
	"context"
	"fmt"

	"github.com/plaid/plaid-go/v29/plaid"
)

type PlaidProvider struct {
	client      *plaid.APIClient
	clientID    string
	secret      string
	environment string
}

func NewPlaidProvider(clientID, secret, environment string) *PlaidProvider {
	cfg := plaid.NewConfiguration()
	switch environment {
	case "production":
		cfg.UseEnvironment(plaid.Production)
	case "development":
		cfg.UseEnvironment(plaid.Development)
	default:
		cfg.UseEnvironment(plaid.Sandbox)
	}
	cfg.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	cfg.AddDefaultHeader("PLAID-SECRET", secret)

	return &PlaidProvider{
		client:      plaid.NewAPIClient(cfg),
		clientID:    clientID,
		secret:      secret,
		environment: environment,
	}
}

func (p *PlaidProvider) Name() string {
	return "plaid"
}

func (p *PlaidProvider) CreateLinkToken(ctx context.Context, userID string) (string, error) {
	user := plaid.LinkTokenCreateRequestUser{ClientUserId: userID}
	req := plaid.NewLinkTokenCreateRequest("Asset Tracker", "en", []plaid.CountryCode{plaid.COUNTRYCODE_US}, user)
	req.SetProducts([]plaid.Products{plaid.PRODUCTS_AUTH, plaid.PRODUCTS_TRANSACTIONS})

	resp, _, err := p.client.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*req).Execute()
	if err != nil {
		return "", fmt.Errorf("plaid link token create: %w", err)
	}
	return resp.GetLinkToken(), nil
}

func (p *PlaidProvider) ExchangeToken(ctx context.Context, publicToken string) (string, error) {
	req := plaid.NewItemPublicTokenExchangeRequest(publicToken)
	resp, _, err := p.client.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*req).Execute()
	if err != nil {
		return "", fmt.Errorf("plaid token exchange: %w", err)
	}
	return resp.GetAccessToken(), nil
}

func (p *PlaidProvider) FetchAccounts(ctx context.Context, accessToken string) ([]Account, error) {
	req := plaid.NewAccountsBalanceGetRequest(accessToken)
	resp, _, err := p.client.PlaidApi.AccountsBalanceGet(ctx).AccountsBalanceGetRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("plaid accounts balance get: %w", err)
	}

	var accounts []Account
	for _, acct := range resp.GetAccounts() {
		balance := acct.GetBalances()
		current := 0.0
		if balance.HasCurrent() && balance.GetCurrent().IsSet() {
			current = float64(*balance.GetCurrent().Get())
		}
		currency := "USD"
		if balance.HasIsoCurrencyCode() && balance.GetIsoCurrencyCode().IsSet() {
			currency = *balance.GetIsoCurrencyCode().Get()
		}

		accounts = append(accounts, Account{
			ExternalID:  acct.GetAccountId(),
			Name:        acct.GetName(),
			Type:        string(acct.GetType()),
			Balance:     current,
			Currency:    currency,
			Institution: "Plaid Sandbox",
		})
	}

	return accounts, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: Success.

- [ ] **Step 4: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/banking/plaid.go
git -c commit.gpgsign=false commit -m "feat: add Plaid bank provider implementation"
```

---

## Task 5: Banking Package — Teller Provider

**Files:**
- Create: `backend/internal/banking/teller.go`

- [ ] **Step 1: Create the Teller provider implementation**

Create `backend/internal/banking/teller.go`:

```go
package banking

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type TellerProvider struct {
	applicationID string
	environment   string
	httpClient    *http.Client
}

func NewTellerProvider(applicationID, environment string) *TellerProvider {
	return &TellerProvider{
		applicationID: applicationID,
		environment:   environment,
		httpClient:    &http.Client{},
	}
}

func (t *TellerProvider) Name() string {
	return "teller"
}

func (t *TellerProvider) CreateLinkToken(_ context.Context, _ string) (string, error) {
	// Teller Connect is initialized client-side with the application ID.
	// No server-side link token is needed.
	return "", nil
}

func (t *TellerProvider) ExchangeToken(_ context.Context, accessToken string) (string, error) {
	// Teller Connect returns the access token directly to the frontend.
	// No exchange step needed — pass through.
	return accessToken, nil
}

func (t *TellerProvider) FetchAccounts(ctx context.Context, accessToken string) ([]Account, error) {
	baseURL := "https://api.teller.io"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("teller create request: %w", err)
	}
	req.SetBasicAuth(accessToken, "")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teller fetch accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("teller accounts returned status %d", resp.StatusCode)
	}

	var tellerAccounts []tellerAccount
	if err := json.NewDecoder(resp.Body).Decode(&tellerAccounts); err != nil {
		return nil, fmt.Errorf("teller decode accounts: %w", err)
	}

	var accounts []Account
	for _, ta := range tellerAccounts {
		balance, err := t.fetchBalance(ctx, accessToken, ta.ID)
		if err != nil {
			balance = 0
		}

		accounts = append(accounts, Account{
			ExternalID:  ta.ID,
			Name:        ta.Name,
			Type:        ta.Type,
			Balance:     balance,
			Currency:    ta.Currency,
			Institution: ta.Institution.Name,
		})
	}

	return accounts, nil
}

func (t *TellerProvider) fetchBalance(ctx context.Context, accessToken, accountID string) (float64, error) {
	baseURL := "https://api.teller.io"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/accounts/"+accountID+"/balances", nil)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(accessToken, "")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("teller balances returned status %d", resp.StatusCode)
	}

	var bal tellerBalance
	if err := json.NewDecoder(resp.Body).Decode(&bal); err != nil {
		return 0, err
	}

	return bal.Available, nil
}

type tellerAccount struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Currency    string            `json:"currency"`
	Institution tellerInstitution `json:"institution"`
}

type tellerInstitution struct {
	Name string `json:"name"`
}

type tellerBalance struct {
	Available float64 `json:"available,string"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: Success.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/banking/teller.go
git -c commit.gpgsign=false commit -m "feat: add Teller bank provider implementation"
```

---

## Task 6: Banking Package — Handler

**Files:**
- Create: `backend/internal/banking/handler.go`

- [ ] **Step 1: Create the banking HTTP handler**

Create `backend/internal/banking/handler.go`:

```go
package banking

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db        *pgxpool.Pool
	providers map[string]BankProvider
}

func NewHandler(db *pgxpool.Pool, providers map[string]BankProvider) *Handler {
	return &Handler{db: db, providers: providers}
}

type linkTokenRequest struct {
	Provider string `json:"provider"`
}

type linkTokenResponse struct {
	LinkToken     string `json:"link_token,omitempty"`
	ApplicationID string `json:"application_id,omitempty"`
	Provider      string `json:"provider"`
}

type connectRequest struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
}

func (h *Handler) CreateLinkToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	var req linkTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	provider, ok := h.providers[req.Provider]
	if !ok {
		http.Error(w, `{"error":"unsupported or disabled provider"}`, http.StatusBadRequest)
		return
	}

	resp := linkTokenResponse{Provider: req.Provider}

	if req.Provider == "teller" {
		// Teller doesn't need a server-side link token; return the application ID
		if tp, ok := provider.(*TellerProvider); ok {
			resp.ApplicationID = tp.applicationID
		}
	} else {
		token, err := provider.CreateLinkToken(r.Context(), userID)
		if err != nil {
			log.Printf("banking: create link token error: %v", err)
			http.Error(w, `{"error":"failed to create link token"}`, http.StatusInternalServerError)
			return
		}
		resp.LinkToken = token
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	provider, ok := h.providers[req.Provider]
	if !ok {
		http.Error(w, `{"error":"unsupported or disabled provider"}`, http.StatusBadRequest)
		return
	}

	accessToken, err := provider.ExchangeToken(r.Context(), req.Token)
	if err != nil {
		log.Printf("banking: exchange token error: %v", err)
		http.Error(w, `{"error":"failed to exchange token"}`, http.StatusInternalServerError)
		return
	}

	accounts, err := provider.FetchAccounts(r.Context(), accessToken)
	if err != nil {
		log.Printf("banking: fetch accounts error: %v", err)
		http.Error(w, `{"error":"failed to fetch accounts"}`, http.StatusInternalServerError)
		return
	}

	for _, acct := range accounts {
		err := h.upsertLinkedAccount(r.Context(), userID, provider.Name(), accessToken, acct)
		if err != nil {
			log.Printf("banking: upsert account error: %v", err)
			http.Error(w, `{"error":"failed to save accounts"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"accounts_linked": len(accounts)})
}

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	rows, err := h.db.Query(context.Background(),
		`SELECT a.id, a.name, a.source, a.external_id, a.institution, a.updated_at,
		        COALESCE(v.value, 0), COALESCE(v.currency, 'USD')
		 FROM assets a
		 LEFT JOIN LATERAL (
		     SELECT value, currency FROM asset_value_points
		     WHERE asset_id = a.id AND user_id = a.user_id
		     ORDER BY timestamp DESC LIMIT 1
		 ) v ON true
		 WHERE a.user_id = $1 AND a.source != 'manual'
		 ORDER BY a.updated_at DESC`,
		userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	accounts := []LinkedAccount{}
	for rows.Next() {
		var a LinkedAccount
		if err := rows.Scan(&a.ID, &a.Name, &a.Source, &a.ExternalID, &a.Institution, &a.UpdatedAt, &a.Balance, &a.Currency); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		a.UserID = userID
		accounts = append(accounts, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accounts)
}

func (h *Handler) SyncAccounts(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	synced, err := h.syncUserAccounts(r.Context(), userID)
	if err != nil {
		log.Printf("banking: sync error: %v", err)
		http.Error(w, `{"error":"sync failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"accounts_synced": synced})
}

func (h *Handler) UnlinkAccount(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	accountID := chi.URLParam(r, "id")

	// Delete value points first, then the asset
	_, err := h.db.Exec(context.Background(),
		"DELETE FROM asset_value_points WHERE asset_id = $1 AND user_id = $2",
		accountID, userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	tag, err := h.db.Exec(context.Background(),
		"DELETE FROM assets WHERE id = $1 AND user_id = $2 AND source != 'manual'",
		accountID, userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, `{"error":"linked account not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) upsertLinkedAccount(ctx context.Context, userID, source, accessToken string, acct Account) error {
	assetID := fmt.Sprintf("%s-%s", source, acct.ExternalID)
	description := fmt.Sprintf("%s %s account", acct.Institution, acct.Type)

	_, err := h.db.Exec(ctx,
		`INSERT INTO assets (id, user_id, name, description, source, external_id, access_token, institution)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id, user_id) DO UPDATE SET
		     name = EXCLUDED.name,
		     description = EXCLUDED.description,
		     access_token = EXCLUDED.access_token,
		     institution = EXCLUDED.institution,
		     updated_at = now()`,
		assetID, userID, acct.Name, description, source, acct.ExternalID, accessToken, acct.Institution,
	)
	if err != nil {
		return fmt.Errorf("upsert asset: %w", err)
	}

	// Insert a value point for the current balance
	_, err = h.db.Exec(ctx,
		`INSERT INTO asset_value_points (asset_id, user_id, value, currency)
		 VALUES ($1, $2, $3, $4)`,
		assetID, userID, acct.Balance, acct.Currency,
	)
	if err != nil {
		return fmt.Errorf("insert value point: %w", err)
	}

	return nil
}

func (h *Handler) syncUserAccounts(ctx context.Context, userID string) (int, error) {
	rows, err := h.db.Query(ctx,
		`SELECT DISTINCT source, access_token FROM assets
		 WHERE user_id = $1 AND source != 'manual' AND access_token IS NOT NULL`,
		userID,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type tokenEntry struct {
		source      string
		accessToken string
	}
	var tokens []tokenEntry
	for rows.Next() {
		var t tokenEntry
		if err := rows.Scan(&t.source, &t.accessToken); err != nil {
			return 0, err
		}
		tokens = append(tokens, t)
	}

	synced := 0
	for _, t := range tokens {
		provider, ok := h.providers[t.source]
		if !ok {
			continue
		}

		accounts, err := provider.FetchAccounts(ctx, t.accessToken)
		if err != nil {
			log.Printf("banking: sync fetch error for %s: %v", t.source, err)
			continue
		}

		for _, acct := range accounts {
			assetID := fmt.Sprintf("%s-%s", t.source, acct.ExternalID)
			_, err := h.db.Exec(ctx,
				`INSERT INTO asset_value_points (asset_id, user_id, value, currency)
				 VALUES ($1, $2, $3, $4)`,
				assetID, userID, acct.Balance, acct.Currency,
			)
			if err != nil {
				log.Printf("banking: sync value point error: %v", err)
				continue
			}
			synced++
		}
	}

	return synced, nil
}

// BankingFeatureGate returns middleware that blocks access if neither provider is enabled.
func BankingFeatureGate(providers map[string]BankProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(providers) == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "feature_disabled"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

Note: The `strings` and `time` imports may be unused depending on exact implementation. Remove any unused imports when compiling.

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: Success. Fix any unused import errors if needed.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/banking/handler.go
git -c commit.gpgsign=false commit -m "feat: add banking HTTP handlers for link, connect, list, sync, unlink"
```

---

## Task 7: Banking Package — Daily Sync

**Files:**
- Create: `backend/internal/banking/sync.go`

- [ ] **Step 1: Create the daily sync goroutine**

Create `backend/internal/banking/sync.go`:

```go
package banking

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Syncer struct {
	db        *pgxpool.Pool
	providers map[string]BankProvider
	interval  time.Duration
}

func NewSyncer(db *pgxpool.Pool, providers map[string]BankProvider) *Syncer {
	return &Syncer{
		db:        db,
		providers: providers,
		interval:  24 * time.Hour,
	}
}

func (s *Syncer) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAll(ctx)
		}
	}
}

func (s *Syncer) syncAll(ctx context.Context) {
	log.Println("banking: starting daily sync")

	rows, err := s.db.Query(ctx,
		`SELECT DISTINCT ON (source, access_token) user_id, source, access_token
		 FROM assets
		 WHERE source != 'manual' AND access_token IS NOT NULL`,
	)
	if err != nil {
		log.Printf("banking: sync query error: %v", err)
		return
	}
	defer rows.Close()

	type syncEntry struct {
		userID      string
		source      string
		accessToken string
	}
	var entries []syncEntry
	for rows.Next() {
		var e syncEntry
		if err := rows.Scan(&e.userID, &e.source, &e.accessToken); err != nil {
			log.Printf("banking: sync scan error: %v", err)
			continue
		}
		entries = append(entries, e)
	}

	synced := 0
	for _, e := range entries {
		provider, ok := s.providers[e.source]
		if !ok {
			continue
		}

		accounts, err := provider.FetchAccounts(ctx, e.accessToken)
		if err != nil {
			log.Printf("banking: sync fetch error for %s user %s: %v", e.source, e.userID, err)
			continue
		}

		for _, acct := range accounts {
			assetID := e.source + "-" + acct.ExternalID
			_, err := s.db.Exec(ctx,
				`INSERT INTO asset_value_points (asset_id, user_id, value, currency)
				 VALUES ($1, $2, $3, $4)`,
				assetID, e.userID, acct.Balance, acct.Currency,
			)
			if err != nil {
				log.Printf("banking: sync value point error: %v", err)
				continue
			}
			synced++
		}
	}

	log.Printf("banking: daily sync complete, %d value points updated", synced)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: Success.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/banking/sync.go
git -c commit.gpgsign=false commit -m "feat: add daily bank account balance sync"
```

---

## Task 8: Wire Banking Into main.go

**Files:**
- Modify: `backend/main.go`

- [ ] **Step 1: Add banking import**

Add to the import block in `main.go`:

```go
"github.com/assettrackerhq/asset-tracker/backend/internal/banking"
```

- [ ] **Step 2: Initialize providers and handler after license checker setup**

Add this block after the `go licenseChecker.Run(ctx)` line (around line 98):

```go
// Banking providers
bankingProviders := map[string]banking.BankProvider{}
if cfg.PlaidEnabled {
	bankingProviders["plaid"] = banking.NewPlaidProvider(cfg.PlaidClientID, cfg.PlaidSecret, cfg.PlaidEnvironment)
	log.Println("banking: Plaid provider enabled")
}
if cfg.TellerEnabled {
	bankingProviders["teller"] = banking.NewTellerProvider(cfg.TellerApplicationID, cfg.TellerEnvironment)
	log.Println("banking: Teller provider enabled")
}
bankingHandler := banking.NewHandler(pool, bankingProviders)

// Start daily bank account sync if any providers are enabled
if len(bankingProviders) > 0 {
	syncer := banking.NewSyncer(pool, bankingProviders)
	go syncer.Run(ctx)
}
```

- [ ] **Step 3: Add banking routes**

Add this route group after the analytics routes (around line 187):

```go
// Banking routes (license + auth + banking feature gate)
r.Route("/api/banking", func(r chi.Router) {
	r.Use(license.LicenseMiddleware(licenseChecker))
	r.Use(auth.Middleware(cfg.JWTSecret))
	r.Use(banking.BankingFeatureGate(bankingProviders))
	r.Post("/link-token", bankingHandler.CreateLinkToken)
	r.Post("/connect", bankingHandler.Connect)
	r.Get("/accounts", bankingHandler.ListAccounts)
	r.Post("/accounts/sync", bankingHandler.SyncAccounts)
	r.Delete("/accounts/{id}", bankingHandler.UnlinkAccount)
})
```

- [ ] **Step 4: Update the features endpoint**

Modify the existing `/api/features` handler to include banking flags. Replace the current features handler with:

```go
r.Get("/api/features", func(w http.ResponseWriter, r *http.Request) {
	enabled, err := licenseClient.AnalyticsEnabled(r.Context())
	if err != nil {
		log.Printf("license: failed to check analytics_enabled, using default: %v", err)
		enabled = cfg.AnalyticsEnabled
	}
	features := map[string]any{
		"analytics_enabled": enabled,
		"plaid_enabled":     cfg.PlaidEnabled,
		"teller_enabled":    cfg.TellerEnabled,
	}
	if cfg.TellerEnabled {
		features["teller_application_id"] = cfg.TellerApplicationID
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(features)
})
```

- [ ] **Step 5: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: Success.

- [ ] **Step 6: Commit**

```bash
git add backend/main.go
git -c commit.gpgsign=false commit -m "feat: wire banking routes, providers, sync, and feature flags into main"
```

---

## Task 9: Helm Chart — Values, Schema, Deployment

**Files:**
- Modify: `helm/asset-tracker/values.yaml`
- Modify: `helm/asset-tracker/values.schema.json`
- Modify: `helm/asset-tracker/templates/backend-deployment.yaml`

- [ ] **Step 1: Add Plaid/Teller values to values.yaml**

Add under the `backend:` section, after the existing `resources:` block:

```yaml
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

- [ ] **Step 2: Add schema entries to values.schema.json**

Add within the `backend.properties` object in the JSON schema:

```json
"plaid": {
  "type": "object",
  "properties": {
    "enabled": { "type": "boolean" },
    "clientId": { "type": "string" },
    "secret": { "type": "string" },
    "environment": { "type": "string" }
  }
},
"teller": {
  "type": "object",
  "properties": {
    "enabled": { "type": "boolean" },
    "applicationId": { "type": "string" },
    "environment": { "type": "string" }
  }
}
```

- [ ] **Step 3: Add environment variables to backend deployment template**

Add these env var entries to the backend container's `env:` list in `helm/asset-tracker/templates/backend-deployment.yaml`:

```yaml
        - name: PLAID_ENABLED
          value: {{ .Values.backend.plaid.enabled | quote }}
        - name: PLAID_CLIENT_ID
          value: {{ .Values.backend.plaid.clientId | quote }}
        - name: PLAID_SECRET
          value: {{ .Values.backend.plaid.secret | quote }}
        - name: PLAID_ENVIRONMENT
          value: {{ .Values.backend.plaid.environment | quote }}
        - name: TELLER_ENABLED
          value: {{ .Values.backend.teller.enabled | quote }}
        - name: TELLER_APPLICATION_ID
          value: {{ .Values.backend.teller.applicationId | quote }}
        - name: TELLER_ENVIRONMENT
          value: {{ .Values.backend.teller.environment | quote }}
```

- [ ] **Step 4: Verify helm template renders**

Run: `cd helm/asset-tracker && helm dependency build && helm template test . --set backend.plaid.enabled=true --set backend.teller.enabled=true | grep -A2 PLAID_ENABLED`
Expected: Shows `PLAID_ENABLED` with value `"true"`.

- [ ] **Step 5: Commit**

```bash
git add helm/asset-tracker/values.yaml helm/asset-tracker/values.schema.json helm/asset-tracker/templates/backend-deployment.yaml
git -c commit.gpgsign=false commit -m "feat: add Plaid and Teller Helm values and deployment env vars"
```

---

## Task 10: KOTS Config & HelmChart CR

**Files:**
- Modify: `replicated/config.yaml`
- Modify: `replicated/helmchart-asset-tracker.yaml`

- [ ] **Step 1: Add Integrations group to KOTS config**

Add a new group after the existing `database` group in `replicated/config.yaml`:

```yaml
    - name: integrations
      title: Integrations
      description: Configure optional third-party integrations for Asset Tracker.
      items:
        - name: plaid_enabled
          title: Plaid Integration
          help_text: Enable Plaid to allow users to link bank accounts.
          type: select_one
          default: disabled
          items:
            - name: disabled
              title: Disabled
            - name: enabled
              title: Enabled
        - name: plaid_client_id
          title: Plaid Client ID
          type: text
          when: '{{repl ConfigOptionEquals "plaid_enabled" "enabled" }}'
          required: true
        - name: plaid_secret
          title: Plaid Secret
          type: password
          when: '{{repl ConfigOptionEquals "plaid_enabled" "enabled" }}'
          required: true
        - name: teller_enabled
          title: Teller Integration
          help_text: Enable Teller to allow users to link bank accounts.
          type: select_one
          default: disabled
          items:
            - name: disabled
              title: Disabled
            - name: enabled
              title: Enabled
        - name: teller_application_id
          title: Teller Application ID
          type: text
          when: '{{repl ConfigOptionEquals "teller_enabled" "enabled" }}'
          required: true
```

- [ ] **Step 2: Add optionalValues to HelmChart CR**

Add these blocks to the `optionalValues` array in `replicated/helmchart-asset-tracker.yaml`:

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

- [ ] **Step 3: Commit**

```bash
git add replicated/config.yaml replicated/helmchart-asset-tracker.yaml
git -c commit.gpgsign=false commit -m "feat: add Plaid and Teller KOTS config toggles and HelmChart wiring"
```

---

## Task 11: Frontend — NPM Dependencies and API Functions

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/src/api.js`

- [ ] **Step 1: Install frontend dependencies**

Run:
```bash
cd frontend && npm install react-plaid-link teller-connect-react
```

- [ ] **Step 2: Add banking API functions to api.js**

Add these functions at the end of `frontend/src/api.js`:

```javascript
export function createLinkToken(provider) {
  return request('/banking/link-token', {
    method: 'POST',
    body: JSON.stringify({ provider }),
  });
}

export function connectAccount(provider, token) {
  return request('/banking/connect', {
    method: 'POST',
    body: JSON.stringify({ provider, token }),
  });
}

export function listLinkedAccounts() {
  return request('/banking/accounts');
}

export function syncAccounts() {
  return request('/banking/accounts/sync', { method: 'POST' });
}

export function unlinkAccount(id) {
  return request(`/banking/accounts/${id}`, { method: 'DELETE' });
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/api.js
git -c commit.gpgsign=false commit -m "feat: add banking API functions and Plaid/Teller frontend dependencies"
```

---

## Task 12: Frontend — LinkedAccounts Page

**Files:**
- Create: `frontend/src/pages/LinkedAccounts.jsx`

- [ ] **Step 1: Create the LinkedAccounts page**

Create `frontend/src/pages/LinkedAccounts.jsx`:

```jsx
import { useState, useEffect, useCallback } from 'react';
import { usePlaidLink } from 'react-plaid-link';
import { listLinkedAccounts, createLinkToken, connectAccount, syncAccounts, unlinkAccount } from '../api';

function PlaidLinkButton({ onSuccess }) {
  const [linkToken, setLinkToken] = useState(null);

  useEffect(() => {
    createLinkToken('plaid').then((data) => {
      setLinkToken(data.link_token);
    });
  }, []);

  const onPlaidSuccess = useCallback(async (publicToken) => {
    await connectAccount('plaid', publicToken);
    onSuccess();
  }, [onSuccess]);

  const { open, ready } = usePlaidLink({
    token: linkToken,
    onSuccess: onPlaidSuccess,
  });

  return (
    <button className="primary" onClick={() => open()} disabled={!ready}>
      Link with Plaid
    </button>
  );
}

function TellerConnectButton({ applicationId, onSuccess }) {
  const handleClick = useCallback(() => {
    if (!window.TellerConnect) {
      console.error('Teller Connect SDK not loaded');
      return;
    }
    const teller = window.TellerConnect.setup({
      applicationId: applicationId,
      environment: 'sandbox',
      onSuccess: async (enrollment) => {
        await connectAccount('teller', enrollment.accessToken);
        onSuccess();
      },
    });
    teller.open();
  }, [applicationId, onSuccess]);

  return (
    <button className="primary" onClick={handleClick}>
      Link with Teller
    </button>
  );
}

export default function LinkedAccounts({ plaidEnabled, tellerEnabled, tellerApplicationId }) {
  const [accounts, setAccounts] = useState([]);
  const [error, setError] = useState('');
  const [syncing, setSyncing] = useState(false);

  useEffect(() => {
    loadAccounts();
  }, []);

  async function loadAccounts() {
    try {
      const data = await listLinkedAccounts();
      setAccounts(data);
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleSync() {
    setSyncing(true);
    setError('');
    try {
      await syncAccounts();
      await loadAccounts();
    } catch (err) {
      setError(err.message);
    } finally {
      setSyncing(false);
    }
  }

  async function handleUnlink(id) {
    if (!confirm('Unlink this account? All value history will be deleted.')) return;
    try {
      await unlinkAccount(id);
      await loadAccounts();
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="container">
      <div className="header">
        <h1>Linked Accounts</h1>
        <div>
          {plaidEnabled && <PlaidLinkButton onSuccess={loadAccounts} />}
          {tellerEnabled && (
            <TellerConnectButton
              applicationId={tellerApplicationId}
              onSuccess={loadAccounts}
            />
          )}
          {accounts.length > 0 && (
            <button className="secondary" onClick={handleSync} disabled={syncing}>
              {syncing ? 'Syncing...' : 'Sync Now'}
            </button>
          )}
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Institution</th>
            <th>Source</th>
            <th>Balance</th>
            <th>Currency</th>
            <th>Last Updated</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {accounts.map((acct) => (
            <tr key={acct.id}>
              <td>{acct.name}</td>
              <td>{acct.institution}</td>
              <td><span className={`badge ${acct.source}`}>{acct.source}</span></td>
              <td>{acct.balance.toFixed(2)}</td>
              <td>{acct.currency}</td>
              <td>{new Date(acct.updated_at).toLocaleDateString()}</td>
              <td className="actions">
                <button className="danger" onClick={() => handleUnlink(acct.id)}>Unlink</button>
              </td>
            </tr>
          ))}
          {accounts.length === 0 && (
            <tr><td colSpan="7" style={{ textAlign: 'center', color: '#999' }}>No linked accounts yet</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 2: Add Teller Connect script to index.html**

Edit `frontend/index.html` to add the Teller Connect SDK script in the `<head>`:

```html
<script src="https://cdn.teller.io/connect/connect.js"></script>
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/LinkedAccounts.jsx frontend/index.html
git -c commit.gpgsign=false commit -m "feat: add LinkedAccounts page with Plaid Link and Teller Connect"
```

---

## Task 13: Frontend — App.jsx and NavBar Wiring

**Files:**
- Modify: `frontend/src/App.jsx`
- Modify: `frontend/src/components/NavBar.jsx`

- [ ] **Step 1: Update App.jsx to add LinkedAccounts route and feature flags**

Add the import at the top:

```javascript
import LinkedAccounts from './pages/LinkedAccounts';
```

Add state variables in the `App` component (after the existing `analyticsEnabled` state):

```javascript
const [plaidEnabled, setPlaidEnabled] = useState(false);
const [tellerEnabled, setTellerEnabled] = useState(false);
const [tellerApplicationId, setTellerApplicationId] = useState('');
```

Update the `getFeatures` callback in the `useEffect` to also set the new flags:

```javascript
getFeatures().then((data) => {
  setAnalyticsEnabled(data.analytics_enabled);
  setPlaidEnabled(data.plaid_enabled || false);
  setTellerEnabled(data.teller_enabled || false);
  setTellerApplicationId(data.teller_application_id || '');
});
```

Update the `Layout` component to accept and pass the new props:

```jsx
<Layout updateAvailable={updateAvailable} analyticsEnabled={analyticsEnabled} plaidEnabled={plaidEnabled} tellerEnabled={tellerEnabled}>
```

Add the new route before the catch-all `*` route:

```jsx
<Route path="/linked-accounts" element={
  <ProtectedRoute>
    <LinkedAccounts plaidEnabled={plaidEnabled} tellerEnabled={tellerEnabled} tellerApplicationId={tellerApplicationId} />
  </ProtectedRoute>
} />
```

Update the `Layout` function signature and NavBar props:

```jsx
function Layout({ children, updateAvailable, analyticsEnabled, plaidEnabled, tellerEnabled }) {
```

```jsx
<NavBar updateAvailable={updateAvailable} analyticsEnabled={analyticsEnabled} plaidEnabled={plaidEnabled} tellerEnabled={tellerEnabled} />
```

- [ ] **Step 2: Update NavBar to show Linked Accounts link**

Add `plaidEnabled` and `tellerEnabled` to the NavBar component's props destructuring.

Add this NavLink after the existing Analytics link (conditionally rendered):

```jsx
{(plaidEnabled || tellerEnabled) && (
  <NavLink to="/linked-accounts" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
    Linked Accounts
  </NavLink>
)}
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Success, no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.jsx frontend/src/components/NavBar.jsx
git -c commit.gpgsign=false commit -m "feat: wire LinkedAccounts route and nav link with feature flag conditionals"
```

---

## Task 14: E2E Test Updates

**Files:**
- Modify: `e2e/asset-tracker.spec.mjs`

- [ ] **Step 1: Add linked accounts visibility test**

Add a new test group at the end of the existing spec file, inside the outermost `test.describe.serial`:

```javascript
test.describe('Linked Accounts', () => {
  test('linked accounts nav link is visible when integrations are enabled', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);
    await page.locator('input[type="text"]').fill(username);
    await page.locator('input[type="password"]').fill(password);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL('**/assets');

    const navLink = page.locator('a[href="/linked-accounts"]');
    await expect(navLink).toBeVisible();
    await expect(navLink).toHaveText('Linked Accounts');
  });

  test('linked accounts page shows link buttons', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);
    await page.locator('input[type="text"]').fill(username);
    await page.locator('input[type="password"]').fill(password);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL('**/assets');

    await page.goto(`${BASE_URL}/linked-accounts`);
    await expect(page.locator('h1')).toHaveText('Linked Accounts');

    // At least one link button should be visible (Plaid, Teller, or both)
    const plaidButton = page.locator('button:has-text("Link with Plaid")');
    const tellerButton = page.locator('button:has-text("Link with Teller")');
    const eitherVisible = await plaidButton.isVisible() || await tellerButton.isVisible();
    expect(eitherVisible).toBeTruthy();
  });

  test('linked accounts table shows empty state', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);
    await page.locator('input[type="text"]').fill(username);
    await page.locator('input[type="password"]').fill(password);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL('**/assets');

    await page.goto(`${BASE_URL}/linked-accounts`);
    await expect(page.locator('text=No linked accounts yet')).toBeVisible();
  });
});
```

Note: The `username`, `password`, and `BASE_URL` variables are shared from the outer test scope — check the existing spec to confirm exact variable names and login flow.

- [ ] **Step 2: Update CI config values for EC tests**

Check `.github/workflows/ci.yml` for where config values are passed to EC installs. Add the Plaid/Teller test values to the config-values files used in CI:

```yaml
plaid_enabled: "enabled"
plaid_client_id: "test_client_id"
plaid_secret: "test_secret"
teller_enabled: "enabled"
teller_application_id: "test_app_id"
```

These are added to the inline config-values in the CI workflow wherever `--config-values` is used for EC installs that run Playwright tests.

- [ ] **Step 3: Commit**

```bash
git add e2e/asset-tracker.spec.mjs .github/workflows/ci.yml
git -c commit.gpgsign=false commit -m "test: add linked accounts e2e tests and CI config values"
```

---

## Task 15: Final Build Verification

- [ ] **Step 1: Build backend**

Run: `cd backend && go build ./...`
Expected: Success.

- [ ] **Step 2: Build frontend**

Run: `cd frontend && npm run build`
Expected: Success.

- [ ] **Step 3: Build Helm chart**

Run: `cd helm/asset-tracker && helm dependency build && helm template test .`
Expected: Success, template renders without errors.

- [ ] **Step 4: Verify Helm template with Plaid/Teller enabled**

Run: `helm template test helm/asset-tracker --set backend.plaid.enabled=true --set backend.plaid.clientId=test --set backend.plaid.secret=test --set backend.teller.enabled=true --set backend.teller.applicationId=test | grep -E "(PLAID|TELLER)" | head -20`
Expected: Shows all 7 environment variables with correct values.

- [ ] **Step 5: Clean up any build artifacts**

Run: `rm -f helm/asset-tracker-*.tgz`
