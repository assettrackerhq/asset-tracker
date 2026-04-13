# External PostgreSQL Toggle for EC v3

## Summary

Add a KOTS Config screen that lets users choose between the embedded (bundled) PostgreSQL and an external PostgreSQL instance. Wire the config values into the HelmChart resource. Add a CI job that tests the external PostgreSQL path end-to-end on a CMX VM.

## 1. KOTS Config Screen

**New file:** `replicated/config.yaml`

A `kots.io/v1beta1` Config resource with one group:

### Database group

| Field | Type | Name | Default | When visible |
|---|---|---|---|---|
| `postgres_type` | `select_one` | PostgreSQL | `embedded` | Always |
| `postgres_host` | `text` | Host | — | `postgres_type = external` |
| `postgres_port` | `text` | Port | `5432` | `postgres_type = external` |
| `postgres_database` | `text` | Database | `asset_tracker` | `postgres_type = external` |
| `postgres_username` | `text` | Username | — | `postgres_type = external` |
| `postgres_password` | `password` | Password | — | `postgres_type = external` |

The `select_one` has two options: `embedded` ("Embedded (recommended)") and `external` ("External").

## 2. HelmChart Wiring

**Modified file:** `replicated/helmchart-asset-tracker.yaml`

Add to `spec.values`:

```yaml
postgresql:
  enabled: '{{repl if eq (ConfigOption "postgres_type") "external" }}false{{repl else }}true{{repl end }}'
externalDatabase:
  host: '{{repl ConfigOption "postgres_host" }}'
  port: '{{repl ConfigOption "postgres_port" }}'
  database: '{{repl ConfigOption "postgres_database" }}'
  username: '{{repl ConfigOption "postgres_username" }}'
  password: '{{repl ConfigOption "postgres_password" }}'
```

These map directly to the existing Helm chart conditionals in `_helpers.tpl` — no Helm template changes needed.

## 3. CI Job: `ec-external-postgres-test`

**Modified file:** `.github/workflows/ci.yml`

New job `ec-external-postgres-test` (depends on `create-release`):

### Steps

1. Install Replicated CLI, Helm, Node.js, Playwright
2. Create channel + promote release
3. Create customer with EC entitlement
4. Download license
5. Generate SSH key pair
6. Create CMX VM (Ubuntu 22.04, 50GB disk, 2h TTL)
7. Wait for VM running
8. Parse SSH endpoint
9. **Install PostgreSQL on VM** via SSH:
   - `apt-get install -y postgresql`
   - Configure `pg_hba.conf` to allow connections from any IP (for k8s pods)
   - Configure `postgresql.conf` to listen on all interfaces
   - Create database `asset_tracker` and user `asset_tracker` with password
   - Restart PostgreSQL
10. Copy license to VM
11. **Create config-values YAML** with external postgres settings (host = VM's internal IP `$(hostname -I | awk '{print $1}')`)
12. Copy config-values to VM
13. **Install EC with `--config-values`**: `sudo ./assettracker install --license ~/license.yaml --headless --config-values ~/config-values.yaml --installer-password changeme --ignore-host-preflights --yes`
14. Verify pods — confirm no `postgresql` pod exists, all other pods ready
15. Expose port 30080
16. Run Playwright tests with `BASE_URL` and `MAILPIT_URL`

### Cleanup

Add to the existing `cleanup` job: remove VM, customer, channel for this job.

## 4. Playwright Test

**New file:** `e2e/ec-external-postgres.spec.mjs`

Reuses patterns from `asset-tracker.spec.mjs`. Tests (serial):

1. Health check — `GET /api/health` returns 200
2. Register user — POST to `/api/register`, verify 201
3. Verify email — fetch code from Mailpit API, POST to `/api/verify-email`
4. Login — POST to `/api/login`, get JWT token
5. Create asset — POST to `/api/assets`
6. List assets — GET `/api/assets`, verify the created asset exists
7. UI smoke — navigate to `/login`, verify page loads

**Playwright config:** `e2e/playwright.ec-external-postgres.config.mjs` — runs only `ec-external-postgres.spec.mjs`.

## Files Changed

| File | Action |
|---|---|
| `replicated/config.yaml` | Create |
| `replicated/helmchart-asset-tracker.yaml` | Modify |
| `.github/workflows/ci.yml` | Modify (add job + cleanup) |
| `e2e/ec-external-postgres.spec.mjs` | Create |
| `e2e/playwright.ec-external-postgres.config.mjs` | Create |

## No Changes Needed

- Helm chart templates — already support `postgresql.enabled: false` + `externalDatabase.*`
- Backend code — reads `DATABASE_URL` env var regardless of source
- SchemaHero job — runs migrations against whatever database URL is provided
- Preflight checks — already have conditional external postgres check
