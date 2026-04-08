# Custom Metrics Reporting Design

## Summary

Add periodic reporting of the registered user count from the asset-tracker backend to the Replicated SDK, making it visible on the Instance Details page in the Vendor Portal.

## Architecture

The backend gains a `metrics.Reporter` that runs a background goroutine. On a fixed interval it queries the database for the user count and POSTs it to the Replicated SDK's custom metrics API.

```
┌──────────────┐    SELECT COUNT(*)    ┌────────────┐
│   Reporter   │ ───────────────────── │ PostgreSQL │
│  (goroutine) │                       └────────────┘
│              │    POST /api/v1/app/   ┌────────────────┐
│              │ ── custom-metrics ───▶ │ Replicated SDK │ ──▶ Vendor Portal
└──────────────┘                       └────────────────┘
```

## Components

### `backend/internal/metrics/reporter.go`

- **`Reporter` struct**: holds `*pgxpool.Pool`, SDK endpoint URL, and reporting interval.
- **`New(pool, endpoint, interval)`**: constructor with defaults (endpoint: `http://asset-tracker-sdk:3000/api/v1/app/custom-metrics`, interval: 4 hours).
- **`Run(ctx context.Context)`**: starts the reporting loop. Sends immediately on start, then on each tick. Returns when context is cancelled.
- **`report(ctx)`**: queries `SELECT COUNT(*) FROM users`, POSTs JSON to SDK endpoint. Logs errors but does not fail.

### API contract (Replicated SDK)

```
POST /api/v1/app/custom-metrics
Content-Type: application/json

{
  "data": {
    "num_registered_users": 42
  }
}
```

Returns 200 on success. Non-200 responses are logged and retried on next interval.

### `backend/main.go` changes

- Import and instantiate `metrics.Reporter` after DB pool is ready.
- Launch `reporter.Run(ctx)` in a goroutine.
- Wire up graceful shutdown (cancel context on SIGINT/SIGTERM).

### Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `REPLICATED_SDK_ENDPOINT` | `http://asset-tracker-sdk:3000` | Base URL of the Replicated SDK service |
| `METRICS_INTERVAL` | `4h` | Reporting interval (Go duration format) |

### Helm chart changes

- Add `REPLICATED_SDK_ENDPOINT` and `METRICS_INTERVAL` env vars to the backend deployment template, sourced from `values.yaml`.
- Default values in `values.yaml` matching the defaults above.

## Error handling

- DB query failures: logged, skipped, retried next interval.
- HTTP POST failures: logged, skipped, retried next interval.
- SDK unavailable: no crash, just silent retry.

## Testing

- Unit test for the reporter: mock DB query and HTTP endpoint, verify correct JSON payload.
- Integration: deploy to a test cluster, confirm metric appears in Vendor Portal Instance Details.
