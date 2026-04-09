# Asset Tracker - Development Notes

## Testing with Replicated Clusters

Use the Replicated CLI to create test clusters and validate changes locally instead of waiting for GitHub Actions CI.

### Prerequisites

- `.envrc` contains `REPLICATED_API_TOKEN` — source it before running commands
- `replicated` CLI is installed at `/opt/homebrew/bin/replicated`
- `helm` CLI is available

### Quick Test Workflow

1. **Create a k3s cluster:**
   ```bash
   source .envrc
   replicated cluster create --distribution k3s --name claude-test --ttl 1h --wait 5m
   ```

2. **Get kubeconfig** (adds context to ~/.kube/config):
   ```bash
   replicated cluster kubeconfig <cluster-id>
   # Use the context name: k3s-<cluster-id>-default
   kubectl --context k3s-<cluster-id>-default get nodes
   ```

3. **Build and install the chart:**
   ```bash
   helm dependency build helm/asset-tracker
   helm install asset-tracker helm/asset-tracker \
     --kube-context k3s-<cluster-id>-default \
     --set replicated.enabled=false \
     --set mailpit.enabled=true \
     --set ingress.enabled=true \
     --set ingress.className=nginx \
     --set ingress-nginx.enabled=true \
     --set ingress-nginx.controller.service.type=NodePort \
     --set ingress-nginx.controller.service.nodePorts.http=30080
   ```
   Note: `replicated.enabled=false` is needed when testing without a Replicated license.
   In CI, the SDK is enabled and a license is provisioned via the create-customer action.

4. **Expose and test:**
   ```bash
   replicated cluster port expose <cluster-id> --port 30080 --protocol https
   # Use the returned hostname as BASE_URL for Playwright tests
   ```

5. **Cleanup:**
   ```bash
   replicated cluster rm <cluster-id>
   ```

### Key Helm Values for Testing

- `mailpit.enabled=true` — deploys Mailpit for email testing
- `ingress.enabled=true` — enables Ingress resources
- `postgresql.enabled=true` (default) — uses in-chart PostgreSQL
- `smtp.host=""` (default) — when mailpit enabled, auto-wires to Mailpit service

### CI Workflow

The GitHub Actions CI at `.github/workflows/prepare-cluster.yml`:
- Creates a Replicated release, provisions a k3s cluster, deploys via Helm, runs Playwright e2e tests
- Mailpit is enabled in CI with its API accessible via ingress at `/mailpit/` path
- `MAILPIT_URL` is set to `BASE_URL/mailpit` for tests

## Project Structure

- `backend/` — Go backend (chi router, pgx, JWT auth, email verification)
- `frontend/` — React frontend (Vite, react-router-dom)
- `helm/asset-tracker/` — Helm chart with preflight checks, Mailpit, ingress
- `schemas/` — SchemaHero table definitions + DDL SQL
- `e2e/` — Playwright e2e tests
- `.github/workflows/` — CI/CD pipelines

## Replicated SDK

The Replicated SDK pod will always CrashLoopBackOff on local installs (without a Replicated customer test or dev license). This is expected behavior — the SDK needs a valid license to start. Don't treat this as a bug during local development.

When installing locally, either set `replicated.enabled=false` or simply expect the SDK pod to fail.

## Git

- GPG signing fails in non-interactive shells. Use `git -c commit.gpgsign=false commit` as workaround.
