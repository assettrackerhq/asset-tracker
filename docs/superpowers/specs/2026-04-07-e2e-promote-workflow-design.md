# E2E Test + Release Promotion Workflow

## Overview

Replace the existing `prepare-cluster.yml` GitHub workflow with a combined workflow that:
1. Provisions a k3s cluster and installs the app
2. Runs Playwright e2e tests against it
3. On success, creates a release promoted to the `unstable` channel

## Trigger

```yaml
on:
  push:
    branches: [main]
    tags: ["v*"]
```

## Jobs

### Job 1: `prepare-cluster`

Packages the Helm chart and provisions a k3s cluster with the app installed, then exposes the frontend via `expose-port`.

**Steps:**
1. Checkout
2. Install Helm
3. Add helm repos (`helmforge`, `jetstack`, `ingress-nginx`) and package chart
4. Run `replicatedhq/replicated-actions/prepare-cluster@v1.20.0` with:
   - `kubernetes-distribution: k3s`
   - `ttl: 1h`
   - `namespace: default`
   - `export-kubeconfig: "true"`
   - `helm-run-preflights: "false"`
   - `helm-chart-name: asset-tracker`
   - Additional helm values to enable ingress with NodePort:
     - `ingress.enabled=true`
     - `ingress-nginx.enabled=true`
     - `ingress-nginx.controller.service.type=NodePort`
     - `ingress-nginx.controller.service.nodePorts.http=30080`
5. Run `replicatedhq/replicated-actions/expose-port` with:
   - `cluster-id` from prepare-cluster output
   - `port: 30080`
   - `protocols: https`
6. Upload packaged chart as artifact (for job 3)

**Outputs:** `cluster-id`, `hostname` (from expose-port)

### Job 2: `e2e-tests`

**Depends on:** `prepare-cluster`

Runs the existing Playwright test suite against the exposed cluster.

**Steps:**
1. Checkout
2. Setup Node.js
3. `npm ci`
4. Install Playwright browsers (chromium)
5. Run `npx playwright test` with `BASE_URL=https://<hostname>`
6. Upload Playwright report as artifact (always, for debugging)

### Job 3: `create-release`

**Depends on:** `e2e-tests`

Creates a Replicated release and promotes it to `unstable`.

**Steps:**
1. Download chart artifact from job 1
2. Run `replicatedhq/replicated-actions/create-release` with:
   - `chart`: the packaged `.tgz` file
   - `promote-channel: unstable`
   - `version`: the git tag (e.g. `v1.2.3`) if triggered by a tag push, otherwise the short SHA

## Secrets Required

- `REPLICATED_APP_SLUG`
- `REPLICATED_API_TOKEN`

(Both already exist from the current `prepare-cluster.yml`)

## Helm Value Overrides for CI

The `prepare-cluster` step passes these values to enable ingress-nginx with NodePort exposure on the k3s cluster:

| Value | Setting | Purpose |
|-------|---------|---------|
| `ingress.enabled` | `true` | Enable the ingress resource |
| `ingress-nginx.enabled` | `true` | Deploy ingress-nginx controller |
| `ingress-nginx.controller.service.type` | `NodePort` | Required for expose-port |
| `ingress-nginx.controller.service.nodePorts.http` | `30080` | Fixed port for expose-port |

No changes to the Helm chart templates are needed. The `ingress.host` defaults to `""` which matches all hosts.

## Artifacts

| Artifact | Producer | Consumer | Purpose |
|----------|----------|----------|---------|
| `helm-chart` | Job 1 | Job 3 | Packaged `.tgz` for release creation |
| `playwright-report` | Job 2 | — | Debugging test failures |
