# Embedded Cluster v3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable Asset Tracker to install on a bare VM via Replicated Embedded Cluster v3, with a CI workflow to validate it.

**Architecture:** Two Replicated manifests (EC Config + HelmChart v2 CR) are added in a `replicated/` directory. A new GitHub Actions workflow packages these with the Helm chart into a `yaml-dir` release, provisions an EC v3 VM, and verifies all pods run and the app responds.

**Tech Stack:** Replicated Embedded Cluster v3, GitHub Actions, Helm, replicated-actions

---

### Task 1: Create the Embedded Cluster Config

**Files:**
- Create: `replicated/embedded-cluster-config.yaml`

- [ ] **Step 1: Create the `replicated/` directory and EC config file**

```yaml
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  version: "3.0.0-alpha-31+k8s-1.34"
  extensions:
    helmCharts:
      - chart:
          name: ingress-nginx
          chartVersion: "4.11.3"
        releaseName: ingress-nginx
        namespace: ingress-nginx
        weight: 10
        values: |
          controller:
            service:
              type: NodePort
```

This tells EC to install version `3.0.0-alpha-31` with Kubernetes 1.34, and pre-deploy ingress-nginx as a NodePort service before the application.

- [ ] **Step 2: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('replicated/embedded-cluster-config.yaml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add replicated/embedded-cluster-config.yaml
git -c commit.gpgsign=false commit -m "feat: add Embedded Cluster v3 config with ingress-nginx extension"
```

---

### Task 2: Create the HelmChart v2 Custom Resource

**Files:**
- Create: `replicated/helmchart-asset-tracker.yaml`

- [ ] **Step 1: Create the HelmChart v2 CR file**

```yaml
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: asset-tracker
spec:
  chart:
    name: asset-tracker
    chartVersion: 0.1.0
  releaseName: asset-tracker
  weight: 20
  values:
    ingress:
      enabled: true
      className: nginx
    ingress-nginx:
      enabled: false
```

This tells Replicated how to deploy the asset-tracker chart. Key overrides:
- `ingress.enabled: true` + `className: nginx` — routes through EC's ingress-nginx extension
- `ingress-nginx.enabled: false` — the app's bundled ingress-nginx is disabled (EC provides it)
- `weight: 20` — installs after ingress-nginx (weight 10)

- [ ] **Step 2: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('replicated/helmchart-asset-tracker.yaml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add replicated/helmchart-asset-tracker.yaml
git -c commit.gpgsign=false commit -m "feat: add HelmChart v2 CR for Embedded Cluster deployment"
```

---

### Task 3: Create the Embedded Cluster CI Workflow

**Files:**
- Create: `.github/workflows/embedded-cluster-ci.yml`

- [ ] **Step 1: Create the workflow file**

```yaml
name: Embedded Cluster v3 CI

on:
  push:
    branches: [main]
    tags: ["v*"]

env:
  APP_VERSION: 0.1.0-${{ github.run_id }}.${{ github.run_attempt }}
  CHANNEL_NAME: ec-ci-${{ github.run_id }}

jobs:
  create-release:
    runs-on: ubuntu-latest
    outputs:
      channel-slug: ${{ steps.create-release.outputs.channel-slug }}
    steps:
      - uses: actions/checkout@v4

      - name: Install Helm
        uses: azure/setup-helm@v4

      - name: Package Helm chart
        id: package
        run: |
          helm repo add helmforge https://repo.helmforge.dev
          helm repo add jetstack https://charts.jetstack.io
          helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
          sed -i "s/^version:.*/version: ${{ env.APP_VERSION }}/" helm/asset-tracker/Chart.yaml
          helm dependency build helm/asset-tracker
          helm package helm/asset-tracker --destination helm/

      - name: Prepare release directory
        run: |
          mkdir -p release
          cp helm/asset-tracker-*.tgz release/
          cp replicated/embedded-cluster-config.yaml release/
          cp replicated/helmchart-asset-tracker.yaml release/
          sed -i "s/chartVersion: .*/chartVersion: ${{ env.APP_VERSION }}/" release/helmchart-asset-tracker.yaml

      - name: Create release
        id: create-release
        uses: replicatedhq/replicated-actions/create-release@v1.20.0
        with:
          app-slug: ${{ secrets.REPLICATED_APP_SLUG }}
          api-token: ${{ secrets.REPLICATED_API_TOKEN }}
          yaml-dir: release
          promote-channel: ${{ env.CHANNEL_NAME }}
          version: ${{ env.APP_VERSION }}

  ec-install-test:
    needs: create-release
    runs-on: ubuntu-latest
    outputs:
      cluster-id: ${{ steps.create-cluster.outputs.cluster-id }}
      customer-id: ${{ steps.create-customer.outputs.customer-id }}
    steps:
      - name: Create customer
        id: create-customer
        uses: replicatedhq/replicated-actions/create-customer@v1.20.0
        with:
          app-slug: ${{ secrets.REPLICATED_APP_SLUG }}
          api-token: ${{ secrets.REPLICATED_API_TOKEN }}
          customer-name: ec-ci-${{ github.run_id }}
          customer-email: ec-ci-${{ github.run_id }}@example.com
          channel-slug: ${{ needs.create-release.outputs.channel-slug }}
          is-kots-install-enabled: "false"
          expires-in: 1

      - name: Create EC v3 cluster
        id: create-cluster
        uses: replicatedhq/replicated-actions/create-cluster@v1.20.0
        with:
          api-token: ${{ secrets.REPLICATED_API_TOKEN }}
          kubernetes-distribution: embedded-cluster-v3
          license-id: ${{ steps.create-customer.outputs.license-id }}
          cluster-name: ec-ci-${{ github.run_id }}
          ttl: 1h
          export-kubeconfig: "true"

      - name: Verify all pods are Running
        run: |
          kubectl get pods -A
          kubectl wait --for=condition=ready pods --all --all-namespaces --timeout=300s
          echo "--- Final pod status ---"
          kubectl get pods -A

      - name: Expose port
        id: expose-port
        uses: replicatedhq/replicated-actions/expose-port@v1.20.0
        with:
          api-token: ${{ secrets.REPLICATED_API_TOKEN }}
          cluster-id: ${{ steps.create-cluster.outputs.cluster-id }}
          port: 30080
          protocols: https

      - name: Verify app is accessible
        run: |
          BASE_URL="https://${{ steps.expose-port.outputs.hostname }}"
          echo "Testing health endpoint..."
          curl -sf --retry 10 --retry-delay 5 "${BASE_URL}/api/health"
          echo ""
          echo "Testing login page..."
          curl -sf --retry 5 --retry-delay 5 -o /dev/null -w "%{http_code}" "${BASE_URL}/login"
          echo ""
          echo "App is accessible!"

  cleanup:
    needs: [create-release, ec-install-test]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - name: Remove cluster
        if: needs.ec-install-test.outputs.cluster-id != ''
        uses: replicatedhq/replicated-actions/remove-cluster@v1.20.0
        continue-on-error: true
        with:
          api-token: ${{ secrets.REPLICATED_API_TOKEN }}
          cluster-id: ${{ needs.ec-install-test.outputs.cluster-id }}

      - name: Archive customer
        if: needs.ec-install-test.outputs.customer-id != ''
        uses: replicatedhq/replicated-actions/archive-customer@v1.20.0
        continue-on-error: true
        with:
          api-token: ${{ secrets.REPLICATED_API_TOKEN }}
          customer-id: ${{ needs.ec-install-test.outputs.customer-id }}

      - name: Archive channel
        uses: replicatedhq/replicated-actions/archive-channel@v1.20.0
        continue-on-error: true
        with:
          app-slug: ${{ secrets.REPLICATED_APP_SLUG }}
          api-token: ${{ secrets.REPLICATED_API_TOKEN }}
          channel-slug: ${{ needs.create-release.outputs.channel-slug }}
```

Key design decisions in this workflow:
- **`yaml-dir` instead of `chart`**: The `release/` staging directory bundles the chart `.tgz`, EC config, and HelmChart CR together.
- **`sed` updates `chartVersion`**: The HelmChart CR's `chartVersion` must match the dynamically-versioned chart.
- **`kubernetes-distribution: embedded-cluster-v3`**: Provisions an EC v3 VM instead of a k3s cluster.
- **`license-id` required**: EC needs a license to install; passed from the customer creation step.
- **No separate helm-install step**: EC handles the full install (k0s + app) automatically.
- **Verification**: `kubectl wait` for all pods + `curl` for health and login endpoints.

- [ ] **Step 2: Validate workflow YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/embedded-cluster-ci.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/embedded-cluster-ci.yml
git -c commit.gpgsign=false commit -m "feat: add Embedded Cluster v3 CI workflow"
```
