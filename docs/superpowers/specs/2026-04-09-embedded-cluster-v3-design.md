# Embedded Cluster v3 Support

## Goal

Enable the Asset Tracker app to be installed on a bare VM using Replicated Embedded Cluster v3. After install, `sudo k0s kubectl get pods -A` shows all pods Running and the app is accessible in a browser.

## Architecture

### New Files

Three new files in a `replicated/` directory at the repo root:

#### 1. `replicated/embedded-cluster-config.yaml`

Embedded Cluster Config CR that tells EC what version to install and what infrastructure extensions to pre-deploy.

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

- **ingress-nginx as EC extension** (weight 10): deployed before the app so ingress is ready. The app's bundled ingress-nginx dependency stays disabled.
- **NodePort service type**: on a bare VM, users access via `http://<VM-IP>:<nodePort>`.

#### 2. `replicated/helmchart-asset-tracker.yaml`

HelmChart v2 CR that tells Replicated how to deploy the asset-tracker Helm chart.

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

- **Minimal values**: only overrides what differs from defaults. `ingress-nginx.enabled: false` because EC provides it. `ingress.enabled: true` + `className: nginx` to route through the EC-provided controller.
- **weight 20**: installs after ingress-nginx (weight 10).

#### 3. `.github/workflows/embedded-cluster-ci.yml`

Separate CI workflow for EC v3 testing.

### CI Workflow: `.github/workflows/embedded-cluster-ci.yml`

**Trigger:** Push to `main` and version tags (same as existing CI).

**Jobs:**

1. **create-release** — Package Helm chart, create Replicated release using `yaml-dir` (which includes the chart `.tgz` + EC config + HelmChart CR).
2. **ec-install-test** — Create a customer (with embedded cluster entitlement), provision an EC VM via `create-cluster` with `kubernetes-distribution: embedded-cluster-v3`, verify pods are running, expose port, hit the app.
3. **cleanup** — Remove cluster, archive customer, archive channel.

**Key differences from existing Helm CI:**

| Aspect | Existing Helm CI | New EC CI |
|--------|-----------------|-----------|
| Release creation | `chart` param only | `yaml-dir` with chart + EC manifests |
| Customer | `is-kots-install-enabled: false` | Needs EC entitlement |
| Cluster | `k3s` distribution | `embedded-cluster-v3` distribution |
| Install | `helm-install` action | Handled by EC installer (no separate helm step) |
| Verification | Playwright e2e tests | Pod status check + HTTP health check |

**Release packaging approach:**

The workflow will:
1. Package the Helm chart as `.tgz`
2. Copy the `.tgz` into a staging directory alongside the `replicated/` YAML files
3. Update `chartVersion` in the HelmChart CR to match the dynamic version
4. Use `create-release` with `yaml-dir` pointing to the staging directory

**Verification steps:**

After EC install completes:
1. `sudo k0s kubectl get pods -A` — all pods Running
2. HTTP request to the app's health endpoint — 200 OK
3. HTTP request to the app's login page — 200 OK

## Changes to Existing Files

### `CLAUDE.md`

Already updated with the "All embedded cluster tasks use Embedded Cluster v3" note.

### No changes to the Helm chart

The Helm chart itself needs no modifications. All EC-specific configuration is in the separate `replicated/` manifests and the HelmChart v2 CR values overrides.

## Out of Scope

- Air-gap installation
- Multi-node / HA setup
- cert-manager / TLS for EC installs
- Playwright e2e tests on EC (just pod + HTTP verification for now)
