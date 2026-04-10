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

### Deploying to a CMX EKS Cluster

1. **Create an EKS cluster:**
   ```bash
   source .envrc
   replicated cluster create --distribution eks --name production-eks --ttl 48h --wait 10m
   ```

2. **Get kubeconfig:**
   ```bash
   replicated cluster kubeconfig <cluster-id>
   ```

3. **Set default StorageClass** (EKS clusters have `gp2` but it's not default):
   ```bash
   kubectl annotate storageclass gp2 storageclass.kubernetes.io/is-default-class=true
   ```

4. **Log in to the Replicated registry:**
   ```bash
   helm registry login registry.replicated.com --username <email> --password <license-id>
   ```

5. **Install with ingress, cert-manager, and Mailpit:**
   ```bash
   helm install asset-tracker oci://registry.replicated.com/assettracker/asset-tracker --version <version> \
     --set ingress.enabled=true \
     --set ingress.className=nginx \
     --set ingress.host=assets.assettracker.tech \
     --set ingress-nginx.enabled=true \
     --set ingress-nginx.controller.service.type=LoadBalancer \
     --set cert-manager.enabled=true \
     --set cert-manager.installCRDs=true \
     --set certManager.staging=false \
     --set certManager.email=<email> \
     --set mailpit.enabled=true
   ```

6. **Point DNS** to the LoadBalancer hostname:
   ```bash
   kubectl get svc | grep LoadBalancer
   # Create a CNAME record for assets.assettracker.tech pointing to the ELB hostname
   ```

7. **Verify:**
   - `https://assets.assettracker.tech/login` — app UI
   - `https://assets.assettracker.tech/mailpit/` — Mailpit UI for email verification
   - `https://assets.assettracker.tech/api/health` — backend health check

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

## Embedded Cluster

- All embedded cluster tasks use Embedded Cluster v3.
- EC v3 docs (deploy preview): https://deploy-preview-3877--replicated-docs.netlify.app/embedded-cluster/v3/embedded-overview
  - Key EC v3 pages:
    - Overview: `/embedded-cluster/v3/embedded-overview`
    - Configure EC: `/embedded-cluster/v3/embedded-using`
    - EC Config reference: `/embedded-cluster/v3/embedded-config`
    - Installation requirements: `/embedded-cluster/v3/installing-embedded-requirements`
    - Manage nodes: `/embedded-cluster/v3/embedded-manage-nodes`
    - Updates: `/embedded-cluster/v3/updating-embedded`
    - Troubleshooting: `/embedded-cluster/v3/embedded-troubleshooting`
    - Commands: `/embedded-cluster/v3/embedded-cluster-completion`
    - Migrate from v2: `/embedded-cluster/v3/embedded-v3-migrate`
- App icon and name in the EC installer are configured via the KOTS Application custom resource (`replicated/application.yaml`), not the Embedded Cluster Config. Use `spec.title` for the app name and `spec.icon` for the icon (base64-encoded PNG/JPG for air gap support).

## Replicated Documentation

- Always use https://docs.replicated.com as the reference for solving Replicated-related tasks (support bundles, preflight checks, KOTS, SDK, etc.)
- For EC v3 specifically, use the deploy preview at https://deploy-preview-3877--replicated-docs.netlify.app/embedded-cluster/v3/ until docs are merged

## Git

- GPG signing fails in non-interactive shells. Use `git -c commit.gpgsign=false commit` as workaround.
