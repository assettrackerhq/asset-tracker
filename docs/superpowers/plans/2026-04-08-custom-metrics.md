# Custom Metrics Reporting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Report registered user count from the backend to the Replicated SDK on a periodic interval, visible in Vendor Portal Instance Details.

**Architecture:** A new `metrics.Reporter` goroutine queries `SELECT COUNT(*) FROM users` and POSTs the result to the Replicated SDK custom metrics API. Configured via env vars, wired into `main.go` with graceful shutdown.

**Tech Stack:** Go 1.26, pgx/v5, net/http, Helm templates

---

### Task 1: Create metrics reporter with tests

**Files:**
- Create: `backend/internal/metrics/reporter.go`
- Create: `backend/internal/metrics/reporter_test.go`

- [ ] **Step 1: Write the test file**

Create `backend/internal/metrics/reporter_test.go`:

```go
package metrics_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/metrics"
)

// mockDB implements metrics.UserCounter for testing.
type mockDB struct {
	count int64
	err   error
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) metrics.Row {
	return &mockRow{count: m.count, err: m.err}
}

type mockRow struct {
	count int64
	err   error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.count
	return nil
}

func TestReportSendsCorrectPayload(t *testing.T) {
	var mu sync.Mutex
	var received map[string]map[string]int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		json.Unmarshal(body, &received)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := &mockDB{count: 42}
	reporter := metrics.New(db, srv.URL+"/api/v1/app/custom-metrics", 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	reporter.Run(ctx)

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("expected to receive a metrics payload")
	}
	if received["data"]["num_registered_users"] != 42 {
		t.Fatalf("expected num_registered_users=42, got %d", received["data"]["num_registered_users"])
	}
}

func TestReportHandlesDBError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call SDK when DB query fails")
	}))
	defer srv.Close()

	db := &mockDB{err: io.ErrUnexpectedEOF}
	reporter := metrics.New(db, srv.URL+"/api/v1/app/custom-metrics", 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should not panic or crash
	reporter.Run(ctx)
}

func TestReportHandlesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	db := &mockDB{count: 5}
	reporter := metrics.New(db, srv.URL+"/api/v1/app/custom-metrics", 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should not panic or crash
	reporter.Run(ctx)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/metrics/ -v`
Expected: Compilation error — `metrics` package does not exist yet.

- [ ] **Step 3: Write the reporter implementation**

Create `backend/internal/metrics/reporter.go`:

```go
package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Row is the interface for scanning a single row result.
type Row interface {
	Scan(dest ...any) error
}

// UserCounter is the interface the reporter needs from the database.
type UserCounter interface {
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// Reporter periodically reports custom metrics to the Replicated SDK.
type Reporter struct {
	db       UserCounter
	endpoint string
	interval time.Duration
}

// New creates a Reporter.
func New(db UserCounter, endpoint string, interval time.Duration) *Reporter {
	return &Reporter{
		db:       db,
		endpoint: endpoint,
		interval: interval,
	}
}

// Run starts the reporting loop. It reports immediately, then on each tick.
// It returns when ctx is cancelled.
func (r *Reporter) Run(ctx context.Context) {
	r.report(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.report(ctx)
		}
	}
}

func (r *Reporter) report(ctx context.Context) {
	var count int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Printf("metrics: failed to query user count: %v", err)
		return
	}

	payload := map[string]any{
		"data": map[string]int64{
			"num_registered_users": count,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("metrics: failed to marshal payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("metrics: failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("metrics: failed to send metrics: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("metrics: SDK returned status %d", resp.StatusCode)
		return
	}

	log.Printf("metrics: reported num_registered_users=%d", count)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/metrics/ -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/metrics/
git commit -m "feat: add metrics reporter with tests"
```

---

### Task 2: Add config fields and wire reporter into main.go

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/main.go`

- [ ] **Step 1: Add config fields for metrics**

In `backend/internal/config/config.go`, add two fields to `Config` and parse them in `Load()`:

```go
type Config struct {
	DatabaseURL            string
	JWTSecret              string
	Port                   string
	ReplicatedSDKEndpoint  string
	MetricsInterval        time.Duration
}
```

In `Load()`, after the existing env var parsing, add:

```go
	sdkEndpoint := os.Getenv("REPLICATED_SDK_ENDPOINT")
	if sdkEndpoint == "" {
		sdkEndpoint = "http://asset-tracker-sdk:3000"
	}

	metricsInterval := 4 * time.Hour
	if v := os.Getenv("METRICS_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid METRICS_INTERVAL: %w", err)
		}
		metricsInterval = d
	}
```

And include them in the returned `Config`:

```go
	return &Config{
		DatabaseURL:           dbURL,
		JWTSecret:             jwtSecret,
		Port:                  port,
		ReplicatedSDKEndpoint: sdkEndpoint,
		MetricsInterval:       metricsInterval,
	}, nil
```

Add `"time"` to the imports.

- [ ] **Step 2: Wire reporter into main.go**

In `backend/main.go`, add signal handling, context cancellation, and launch the reporter.

Add imports:

```go
	"os/signal"
	"syscall"

	"github.com/assettrackerhq/asset-tracker/backend/internal/metrics"
```

Replace the `ctx := context.Background()` line and everything after the route definitions (after line 85) with:

```go
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
```

After `defer pool.Close()`, add the pool adapter and start the reporter:

```go
	// Start metrics reporter
	sdkEndpoint := cfg.ReplicatedSDKEndpoint + "/api/v1/app/custom-metrics"
	reporter := metrics.New(&poolAdapter{pool}, sdkEndpoint, cfg.MetricsInterval)
	go reporter.Run(ctx)
```

Add a pool adapter type (before or after `main`):

```go
// poolAdapter adapts pgxpool.Pool to the metrics.UserCounter interface.
type poolAdapter struct {
	pool *pgxpool.Pool
}

func (a *poolAdapter) QueryRow(ctx context.Context, sql string, args ...any) metrics.Row {
	return a.pool.QueryRow(ctx, sql, args...)
}
```

Add `pgxpool` import:

```go
	"github.com/jackc/pgx/v5/pgxpool"
```

Update the server startup to use a goroutine and wait for shutdown:

```go
	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
```

- [ ] **Step 3: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/config/config.go backend/main.go
git commit -m "feat: wire metrics reporter into backend with graceful shutdown"
```

---

### Task 3: Update Helm chart with metrics env vars

**Files:**
- Modify: `helm/asset-tracker/templates/backend-deployment.yaml`
- Modify: `helm/asset-tracker/values.yaml`

- [ ] **Step 1: Add env vars to backend deployment template**

In `helm/asset-tracker/templates/backend-deployment.yaml`, after the `PORT` env var block (line 48), add:

```yaml
            - name: REPLICATED_SDK_ENDPOINT
              value: {{ .Values.backend.metrics.sdkEndpoint | quote }}
            - name: METRICS_INTERVAL
              value: {{ .Values.backend.metrics.interval | quote }}
```

- [ ] **Step 2: Add default values**

In `helm/asset-tracker/values.yaml`, inside the `backend:` section (after `jwtSecret`), add:

```yaml
  metrics:
    sdkEndpoint: "http://asset-tracker-sdk:3000"
    interval: "4h"
```

- [ ] **Step 3: Validate Helm template renders**

Run: `helm template test helm/asset-tracker/ 2>&1 | grep -A2 REPLICATED_SDK_ENDPOINT`
Expected: Shows the env var with default value `http://asset-tracker-sdk:3000`.

- [ ] **Step 4: Commit**

```bash
git add helm/asset-tracker/templates/backend-deployment.yaml helm/asset-tracker/values.yaml
git commit -m "feat: add metrics config to Helm chart"
```

---

### Task 4: Build, push Docker image, and deploy to test cluster

**Files:**
- No file changes — operational steps.

- [ ] **Step 1: Build and push the backend Docker image**

```bash
cd backend
docker build --platform linux/amd64 -t unawake2068/asset-tracker-backend:metrics .
docker push unawake2068/asset-tracker-backend:metrics
```

- [ ] **Step 2: Create a Replicated release with the updated chart**

```bash
# Package the Helm chart
helm repo add helmforge https://repo.helmforge.dev
helm repo add jetstack https://charts.jetstack.io
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm dependency build helm/asset-tracker
helm package helm/asset-tracker --destination /tmp/ --version 0.1.0-metrics

# Create a release channel and release
replicated channel create --name metrics-test --app $REPLICATED_APP_SLUG
replicated release create --chart /tmp/asset-tracker-0.1.0-metrics.tgz --promote metrics-test --version 0.1.0-metrics --app $REPLICATED_APP_SLUG
```

- [ ] **Step 3: Create a test customer and cluster**

```bash
replicated customer create --name metrics-test --channel metrics-test --app $REPLICATED_APP_SLUG --kots-install=false --expires-in 4h

replicated cluster create --name metrics-test --distribution k3s --ttl 2h
# Wait for cluster to be ready
replicated cluster kubeconfig --id <cluster-id> > /tmp/metrics-kubeconfig
```

- [ ] **Step 4: Deploy the app to the test cluster**

```bash
export KUBECONFIG=/tmp/metrics-kubeconfig

# Get customer license ID for registry auth
CUSTOMER_EMAIL=$(replicated customer ls --app $REPLICATED_APP_SLUG | grep metrics-test | awk '{print $3}')
LICENSE_ID=$(replicated customer ls --app $REPLICATED_APP_SLUG | grep metrics-test | awk '{print $1}')

# Login to registry
helm registry login registry.replicated.com --username $CUSTOMER_EMAIL --password $LICENSE_ID

# Install
helm install asset-tracker oci://registry.replicated.com/$REPLICATED_APP_SLUG/metrics-test/asset-tracker \
  --version 0.1.0-metrics \
  --set backend.image.tag=metrics \
  --set backend.metrics.interval=1m \
  --set global.proxy.enabled=true \
  --set global.proxy.domain=proxy.assettracker.tech \
  --set global.proxy.appSlug=assettracker
```

- [ ] **Step 5: Verify metrics appear**

```bash
# Check backend logs for metrics reporting
kubectl logs -l app.kubernetes.io/component=backend --tail=50

# Register a user to generate real data
kubectl port-forward svc/asset-tracker-backend 8080:8080 &
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}'

# Wait for next metrics interval, then check logs again
kubectl logs -l app.kubernetes.io/component=backend --tail=20 | grep metrics
```

Expected: Log line `metrics: reported num_registered_users=1`.

- [ ] **Step 6: Confirm in Vendor Portal**

Check the Instance Details page in the Vendor Portal for the `num_registered_users` custom metric.

- [ ] **Step 7: Cleanup**

```bash
replicated cluster rm --id <cluster-id>
replicated customer delete --id <customer-id> --app $REPLICATED_APP_SLUG
replicated channel delete --name metrics-test --app $REPLICATED_APP_SLUG
```

- [ ] **Step 8: Commit any final adjustments**

If any code changes were needed during testing, commit them.
