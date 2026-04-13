# External PostgreSQL Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a KOTS Config screen toggle for embedded vs external PostgreSQL, wire it into the HelmChart resource, and add a CI job with Playwright tests for the external PostgreSQL path.

**Architecture:** The Helm chart already supports `postgresql.enabled: true/false` with `externalDatabase.*` values. We add a KOTS Config screen that surfaces this choice to users, wire the config values through the HelmChart CR, then add a CI job that installs PostgreSQL on a CMX VM before running an EC v3 headless install with `--config-values` pointing at the external instance.

**Tech Stack:** KOTS Config (kots.io/v1beta1), Replicated template functions, GitHub Actions, Playwright, PostgreSQL on Ubuntu.

---

### Task 1: Create KOTS Config screen

**Files:**
- Create: `replicated/config.yaml`

- [ ] **Step 1: Create the KOTS Config resource**

Create `replicated/config.yaml`:

```yaml
apiVersion: kots.io/v1beta1
kind: Config
metadata:
  name: asset-tracker
spec:
  groups:
    - name: database
      title: Database
      description: Configure the PostgreSQL database for Asset Tracker.
      items:
        - name: postgres_type
          title: PostgreSQL
          type: select_one
          default: embedded
          items:
            - name: embedded
              title: Embedded (recommended)
            - name: external
              title: External
        - name: postgres_host
          title: Host
          type: text
          when: '{{repl ConfigOptionEquals "postgres_type" "external" }}'
          required: true
        - name: postgres_port
          title: Port
          type: text
          default: "5432"
          when: '{{repl ConfigOptionEquals "postgres_type" "external" }}'
          required: true
        - name: postgres_database
          title: Database Name
          type: text
          default: "asset_tracker"
          when: '{{repl ConfigOptionEquals "postgres_type" "external" }}'
          required: true
        - name: postgres_username
          title: Username
          type: text
          when: '{{repl ConfigOptionEquals "postgres_type" "external" }}'
          required: true
        - name: postgres_password
          title: Password
          type: password
          when: '{{repl ConfigOptionEquals "postgres_type" "external" }}'
          required: true
```

- [ ] **Step 2: Verify YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('replicated/config.yaml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add replicated/config.yaml
git -c commit.gpgsign=false commit -m "feat: add KOTS Config screen for embedded/external postgres toggle"
```

---

### Task 2: Wire config values into HelmChart resource

**Files:**
- Modify: `replicated/helmchart-asset-tracker.yaml`

- [ ] **Step 1: Add postgresql and externalDatabase values to the HelmChart**

Edit `replicated/helmchart-asset-tracker.yaml` to add the database config values. The full file should become:

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
      enabled: true
      controller:
        service:
          type: '{{repl if eq Distribution "embedded-cluster" }}NodePort{{repl else }}LoadBalancer{{repl end }}'
          nodePorts:
            http: '{{repl if eq Distribution "embedded-cluster" }}30080{{repl else }}{{repl end }}'
    backend:
      analyticsEnabled: '{{repl LicenseFieldValue "analytics_enabled" }}'
    mailpit:
      enabled: true
    postgresql:
      enabled: '{{repl if ConfigOptionEquals "postgres_type" "external" }}false{{repl else }}true{{repl end }}'
    externalDatabase:
      host: '{{repl ConfigOption "postgres_host" }}'
      port: '{{repl ConfigOption "postgres_port" }}'
      database: '{{repl ConfigOption "postgres_database" }}'
      username: '{{repl ConfigOption "postgres_username" }}'
      password: '{{repl ConfigOption "postgres_password" }}'
```

- [ ] **Step 2: Verify YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('replicated/helmchart-asset-tracker.yaml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add replicated/helmchart-asset-tracker.yaml
git -c commit.gpgsign=false commit -m "feat: wire KOTS config values into HelmChart for external postgres"
```

---

### Task 3: Add the `prepare-cluster.yml` copy to CI release step

**Files:**
- Modify: `.github/workflows/ci.yml` (line 44, the `Prepare release directory` step)

The `create-release` job copies replicated YAML files into the release directory. The new `config.yaml` must be included.

- [ ] **Step 1: Add config.yaml to the release copy**

In `.github/workflows/ci.yml`, find the `Prepare release directory` step (around line 40-47) and add a copy for `config.yaml`. The step should become:

```yaml
      - name: Prepare release directory
        run: |
          mkdir -p release
          cp helm/asset-tracker-*.tgz release/
          cp replicated/embedded-cluster-config.yaml release/
          cp replicated/helmchart-asset-tracker.yaml release/
          cp replicated/application.yaml release/
          cp replicated/config.yaml release/
          sed -i "s/chartVersion: .*/chartVersion: ${{ env.APP_VERSION }}/" release/helmchart-asset-tracker.yaml
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git -c commit.gpgsign=false commit -m "ci: include KOTS config.yaml in release directory"
```

---

### Task 4: Create Playwright test for external PostgreSQL

**Files:**
- Create: `e2e/ec-external-postgres.spec.mjs`
- Create: `e2e/playwright.ec-external-postgres.config.mjs`

- [ ] **Step 1: Create Playwright config**

Create `e2e/playwright.ec-external-postgres.config.mjs`:

```javascript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: 'ec-external-postgres.spec.mjs',
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: process.env.BASE_URL || 'https://assets.assettracker.tech',
    ignoreHTTPSErrors: true,
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
```

- [ ] **Step 2: Create the test file**

Create `e2e/ec-external-postgres.spec.mjs`:

```javascript
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'https://assets.assettracker.tech';
const API_URL = BASE_URL + '/api';
const MAILPIT_URL = process.env.MAILPIT_URL || BASE_URL + '/mailpit';

const TEST_USERNAME = 'extpg-test-user';
const TEST_EMAIL = 'extpg-test@test.assettracker.local';
const TEST_PASSWORD = 'ExtPgTest123!';

async function getVerificationCode(request, emailAddr) {
  await new Promise((r) => setTimeout(r, 2000));

  const resp = await request.get(`${MAILPIT_URL}/api/v1/messages?limit=5`);
  expect(resp.status()).toBe(200);
  const data = await resp.json();

  const message = data.messages.find((m) =>
    m.To.some((to) => to.Address === emailAddr)
  );
  expect(message).toBeTruthy();

  const msgResp = await request.get(`${MAILPIT_URL}/api/v1/message/${message.ID}`);
  expect(msgResp.status()).toBe(200);
  const msgData = await msgResp.json();

  const match = msgData.Text.match(/(\d{6})/);
  expect(match).toBeTruthy();
  return match[1];
}

let token;

test.describe.serial('EC External PostgreSQL', () => {

  test('health endpoint is ok', async ({ request }) => {
    const resp = await request.get(`${API_URL}/health`);
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.status).toBe('ok');
    expect(body.database).toBe('connected');
  });

  test('register user', async ({ request }) => {
    const resp = await request.post(`${API_URL}/auth/register`, {
      data: { username: TEST_USERNAME, email: TEST_EMAIL, password: TEST_PASSWORD },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.user_id).toBeTruthy();
  });

  test('verify email', async ({ request }) => {
    const code = await getVerificationCode(request, TEST_EMAIL);

    const loginResp = await request.post(`${API_URL}/auth/login`, {
      data: { username: TEST_USERNAME, password: TEST_PASSWORD },
    });
    expect(loginResp.status()).toBe(403);
    const loginBody = await loginResp.json();
    const userId = loginBody.user_id;

    const resp = await request.post(`${API_URL}/auth/verify-email`, {
      data: { user_id: userId, code },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.token).toBeTruthy();
    token = body.token;
  });

  test('login succeeds after verification', async ({ request }) => {
    const resp = await request.post(`${API_URL}/auth/login`, {
      data: { username: TEST_USERNAME, password: TEST_PASSWORD },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.token).toBeTruthy();
    token = body.token;
  });

  test('create asset', async ({ request }) => {
    const resp = await request.post(`${API_URL}/assets`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { id: 'EXTPG-001', name: 'External PG Asset', description: 'Created with external postgres' },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.id).toBe('EXTPG-001');
  });

  test('list assets includes created asset', async ({ request }) => {
    const resp = await request.get(`${API_URL}/assets`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    const asset = body.find((a) => a.id === 'EXTPG-001');
    expect(asset).toBeTruthy();
    expect(asset.name).toBe('External PG Asset');
  });

  test('login page loads', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`);
    await expect(page).toHaveTitle(/Asset Tracker/i);
  });
});
```

- [ ] **Step 3: Commit**

```bash
git add e2e/ec-external-postgres.spec.mjs e2e/playwright.ec-external-postgres.config.mjs
git -c commit.gpgsign=false commit -m "test: add Playwright tests for external PostgreSQL EC install"
```

---

### Task 5: Add `ec-external-postgres-test` CI job

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add the new job**

In `.github/workflows/ci.yml`, add the following job after the `ec-airgap-test` job (before the `promote-release` job). This is a large block — insert it as a new job in the `jobs:` section.

```yaml
  # ─── Embedded Cluster External PostgreSQL Test ─────────────────────
  ec-external-postgres-test:
    needs: create-release
    runs-on: ubuntu-latest
    outputs:
      vm-id: ${{ steps.create-vm.outputs.vm-id }}
      customer-id: ${{ steps.create-customer.outputs.customer-id }}
      channel-slug: ${{ steps.create-channel.outputs.channel-slug }}
    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: npm

      - name: Install dependencies
        run: npm ci

      - name: Install Playwright browsers
        run: npx playwright install --with-deps chromium

      - name: Install Replicated CLI
        run: |
          curl -s https://api.github.com/repos/replicatedhq/replicated/releases/latest \
            | grep "browser_download_url.*linux_amd64.tar.gz" \
            | cut -d '"' -f 4 \
            | xargs curl -sL -o replicated.tar.gz
          mkdir -p /tmp/replicated-cli
          tar xzf replicated.tar.gz -C /tmp/replicated-cli replicated
          sudo mv /tmp/replicated-cli/replicated /usr/local/bin/

      - name: Create channel and promote release
        id: create-channel
        run: |
          CHANNEL_OUTPUT=$(replicated channel create \
            --app "$APP_SLUG" \
            --name "ec-extpg-${{ github.run_id }}" \
            --output json)
          CHANNEL_SLUG=$(echo "$CHANNEL_OUTPUT" | jq -r '.[0].channelSlug')
          echo "channel-slug=$CHANNEL_SLUG" >> "$GITHUB_OUTPUT"
          replicated release promote \
            --app "$APP_SLUG" \
            "${{ needs.create-release.outputs.release-sequence }}" \
            "ec-extpg-${{ github.run_id }}" \
            --version "${{ env.APP_VERSION }}"

      - name: Create customer with Embedded Cluster entitlement
        id: create-customer
        run: |
          OUTPUT=$(replicated customer create \
            --app "$APP_SLUG" \
            --name "ec-extpg-${{ github.run_id }}" \
            --email "ec-extpg-${{ github.run_id }}@example.com" \
            --channel "ec-extpg-${{ github.run_id }}" \
            --embedded-cluster-download \
            --expires-in 24h \
            --type test \
            --output json)
          CUSTOMER_ID=$(echo "$OUTPUT" | jq -r '.id')
          LICENSE_ID=$(echo "$OUTPUT" | jq -r '.installationId')
          echo "::add-mask::$LICENSE_ID"
          echo "customer-id=$CUSTOMER_ID" >> "$GITHUB_OUTPUT"
          echo "license-id=$LICENSE_ID" >> "$GITHUB_OUTPUT"

      - name: Download license
        run: |
          replicated customer download-license \
            --app "$APP_SLUG" \
            --customer "${{ steps.create-customer.outputs.customer-id }}" \
            --output license.yaml

      - name: Generate SSH key pair
        id: ssh-key
        run: |
          ssh-keygen -t ed25519 -f ec-ci-key -N "" -q
          SSH_USERNAME=$(awk '{print $3}' ec-ci-key.pub | cut -d'@' -f1)
          echo "username=$SSH_USERNAME" >> "$GITHUB_OUTPUT"

      - name: Create VM
        id: create-vm
        run: |
          OUTPUT=$(replicated vm create \
            --distribution ubuntu \
            --version 22.04 \
            --name "ec-extpg-${{ github.run_id }}" \
            --disk 50 \
            --ttl 2h \
            --ssh-public-key ec-ci-key.pub \
            --output json)
          VM_ID=$(echo "$OUTPUT" | jq -r '.[0].id')
          echo "vm-id=$VM_ID" >> "$GITHUB_OUTPUT"

      - name: Wait for VM to be running
        timeout-minutes: 10
        run: |
          VM_ID="${{ steps.create-vm.outputs.vm-id }}"
          for i in $(seq 1 60); do
            STATUS=$(replicated vm ls --output json | jq -r ".[] | select(.id == \"$VM_ID\") | .status")
            echo "VM status: $STATUS (attempt $i/60)"
            if [ "$STATUS" = "running" ]; then
              echo "VM is running"
              exit 0
            fi
            sleep 10
          done
          echo "VM did not reach running state in time"
          exit 1

      - name: Parse SSH endpoint
        id: ssh
        run: |
          ENDPOINT=$(replicated vm ssh-endpoint "${{ steps.create-vm.outputs.vm-id }}" --username "${{ steps.ssh-key.outputs.username }}")
          USER_HOST_PORT=$(echo "$ENDPOINT" | sed 's|ssh://||')
          SSH_USER=$(echo "$USER_HOST_PORT" | cut -d'@' -f1)
          HOST_PORT=$(echo "$USER_HOST_PORT" | cut -d'@' -f2)
          SSH_HOST=$(echo "$HOST_PORT" | cut -d':' -f1)
          SSH_PORT=$(echo "$HOST_PORT" | cut -d':' -f2)
          echo "user=$SSH_USER" >> "$GITHUB_OUTPUT"
          echo "host=$SSH_HOST" >> "$GITHUB_OUTPUT"
          echo "port=$SSH_PORT" >> "$GITHUB_OUTPUT"
          mkdir -p ~/.ssh
          chmod 700 ~/.ssh
          echo "StrictHostKeyChecking no" >> ~/.ssh/config
          echo "UserKnownHostsFile /dev/null" >> ~/.ssh/config
          chmod 600 ~/.ssh/config
          chmod 600 ec-ci-key

      - name: Install PostgreSQL on VM
        timeout-minutes: 5
        run: |
          ssh -i ec-ci-key -p "${{ steps.ssh.outputs.port }}" \
            "${{ steps.ssh.outputs.user }}@${{ steps.ssh.outputs.host }}" << 'PG_SCRIPT'
          set -euo pipefail
          sudo apt-get update -qq
          sudo apt-get install -y -qq postgresql postgresql-contrib
          sudo systemctl enable postgresql
          sudo systemctl start postgresql

          # Configure PostgreSQL to listen on all interfaces
          PG_CONF=$(sudo -u postgres psql -t -c "SHOW config_file" | xargs)
          PG_HBA=$(sudo -u postgres psql -t -c "SHOW hba_file" | xargs)
          sudo sed -i "s/#listen_addresses = 'localhost'/listen_addresses = '*'/" "$PG_CONF"
          echo "host all all 0.0.0.0/0 md5" | sudo tee -a "$PG_HBA"

          # Create database and user
          sudo -u postgres psql -c "CREATE USER asset_tracker WITH PASSWORD 'asset_tracker';"
          sudo -u postgres psql -c "CREATE DATABASE asset_tracker OWNER asset_tracker;"
          sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE asset_tracker TO asset_tracker;"

          sudo systemctl restart postgresql
          echo "PostgreSQL is ready"
          PG_SCRIPT

      - name: Get VM internal IP
        id: vm-ip
        run: |
          VM_IP=$(ssh -i ec-ci-key -p "${{ steps.ssh.outputs.port }}" \
            "${{ steps.ssh.outputs.user }}@${{ steps.ssh.outputs.host }}" \
            "hostname -I | awk '{print \$1}'")
          echo "ip=$VM_IP" >> "$GITHUB_OUTPUT"
          echo "VM internal IP: $VM_IP"

      - name: Create config-values file
        run: |
          cat > config-values.yaml << EOF
          apiVersion: kots.io/v1beta1
          kind: ConfigValues
          spec:
            values:
              postgres_type:
                value: "external"
              postgres_host:
                value: "${{ steps.vm-ip.outputs.ip }}"
              postgres_port:
                value: "5432"
              postgres_database:
                value: "asset_tracker"
              postgres_username:
                value: "asset_tracker"
              postgres_password:
                value: "asset_tracker"
          EOF

      - name: Copy files to VM
        run: |
          scp -i ec-ci-key -P "${{ steps.ssh.outputs.port }}" \
            license.yaml "${{ steps.ssh.outputs.user }}@${{ steps.ssh.outputs.host }}:~/license.yaml"
          scp -i ec-ci-key -P "${{ steps.ssh.outputs.port }}" \
            config-values.yaml "${{ steps.ssh.outputs.user }}@${{ steps.ssh.outputs.host }}:~/config-values.yaml"

      - name: Install Embedded Cluster with external PostgreSQL
        timeout-minutes: 15
        run: |
          ssh -i ec-ci-key -p "${{ steps.ssh.outputs.port }}" \
            "${{ steps.ssh.outputs.user }}@${{ steps.ssh.outputs.host }}" << 'INSTALL_SCRIPT'
          set -euo pipefail
          LICENSE_ID=$(grep licenseID ~/license.yaml | awk '{print $2}')
          curl -fL -o assettracker.tgz \
            "https://replicated.app/embedded/assettracker/ec-extpg-${{ github.run_id }}" \
            -H "Authorization: $LICENSE_ID"
          tar xzf assettracker.tgz
          sudo ./assettracker install \
            --license ~/license.yaml \
            --config-values ~/config-values.yaml \
            --headless \
            --installer-password changeme \
            --ignore-host-preflights \
            --yes
          INSTALL_SCRIPT

      - name: Verify no embedded PostgreSQL pod
        timeout-minutes: 5
        run: |
          ssh -i ec-ci-key -p "${{ steps.ssh.outputs.port }}" \
            "${{ steps.ssh.outputs.user }}@${{ steps.ssh.outputs.host }}" << 'VERIFY_SCRIPT'
          set -euo pipefail
          echo "--- Checking for PostgreSQL pods (should be none) ---"
          PG_PODS=$(sudo assettracker shell -c "k0s kubectl get pods -A" | grep -c postgresql || true)
          if [ "$PG_PODS" -gt 0 ]; then
            echo "ERROR: Found PostgreSQL pods when using external database"
            sudo assettracker shell -c "k0s kubectl get pods -A" | grep postgresql
            exit 1
          fi
          echo "No PostgreSQL pods found (expected)"

          echo "--- Waiting for all pods to be ready ---"
          sudo assettracker shell -c "k0s kubectl wait --for=condition=ready pods --all --all-namespaces --field-selector=status.phase!=Succeeded --timeout=300s"

          echo "--- Final pod status ---"
          sudo assettracker shell -c "k0s kubectl get pods -A"
          VERIFY_SCRIPT

      - name: Expose app port
        id: expose-port
        run: |
          OUTPUT=$(replicated vm port expose "${{ steps.create-vm.outputs.vm-id }}" \
            --port 30080 \
            --protocol https \
            --output json)
          HOSTNAME=$(echo "$OUTPUT" | jq -r '.hostname')
          if [ -z "$HOSTNAME" ] || [ "$HOSTNAME" = "null" ]; then
            echo "ERROR: Could not determine hostname from port expose output"
            echo "$OUTPUT"
            exit 1
          fi
          echo "hostname=$HOSTNAME" >> "$GITHUB_OUTPUT"

      - name: Wait for app to be accessible
        run: |
          BASE_URL="https://${{ steps.expose-port.outputs.hostname }}"
          echo "Waiting for app at $BASE_URL ..."
          curl -sf --retry 15 --retry-delay 5 --max-time 10 "${BASE_URL}/api/health"
          echo ""
          echo "App is accessible!"

      - name: Run Playwright tests
        env:
          BASE_URL: https://${{ steps.expose-port.outputs.hostname }}
          MAILPIT_URL: https://${{ steps.expose-port.outputs.hostname }}/mailpit
        run: npx playwright test --config e2e/playwright.ec-external-postgres.config.mjs

      - name: Upload Playwright report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: ec-external-postgres-playwright-report
          path: playwright-report/
          retention-days: 7
```

- [ ] **Step 2: Update the `cleanup` job needs and add cleanup steps**

In `.github/workflows/ci.yml`, update the `cleanup` job's `needs` array to include `ec-external-postgres-test`:

Change:
```yaml
    needs: [create-release, e2e-tests, ec-install-test, ec-upgrade-test, ec-airgap-test]
```
To:
```yaml
    needs: [create-release, e2e-tests, ec-install-test, ec-upgrade-test, ec-airgap-test, ec-external-postgres-test]
```

Then add cleanup steps after the EC airgap cleanup block (after the `Archive EC airgap channel` step). Add:

```yaml
      # EC external postgres cleanup
      - name: Remove EC external postgres VM
        if: needs.ec-external-postgres-test.outputs.vm-id != ''
        continue-on-error: true
        run: replicated vm rm "${{ needs.ec-external-postgres-test.outputs.vm-id }}"

      - name: Archive EC external postgres customer
        if: needs.ec-external-postgres-test.outputs.customer-id != ''
        uses: replicatedhq/replicated-actions/archive-customer@v1.20.0
        continue-on-error: true
        with:
          api-token: ${{ env.REPLICATED_API_TOKEN }}
          customer-id: ${{ needs.ec-external-postgres-test.outputs.customer-id }}

      - name: Archive EC external postgres channel
        if: needs.ec-external-postgres-test.outputs.channel-slug != ''
        uses: replicatedhq/replicated-actions/archive-channel@v1.20.0
        continue-on-error: true
        with:
          app-slug: ${{ env.APP_SLUG }}
          api-token: ${{ env.REPLICATED_API_TOKEN }}
          channel-slug: ${{ needs.ec-external-postgres-test.outputs.channel-slug }}
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git -c commit.gpgsign=false commit -m "ci: add ec-external-postgres-test job with Playwright tests"
```

---

### Task 6: Verify locally and final commit

- [ ] **Step 1: Validate all YAML files parse correctly**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('replicated/config.yaml')); print('config.yaml OK')"
python3 -c "import yaml; yaml.safe_load(open('replicated/helmchart-asset-tracker.yaml')); print('helmchart OK')"
```
Expected: Both print OK

- [ ] **Step 2: Verify Playwright test syntax**

Run: `node -c e2e/ec-external-postgres.spec.mjs && node -c e2e/playwright.ec-external-postgres.config.mjs`
Expected: No output (valid JavaScript)

- [ ] **Step 3: Verify CI workflow YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('ci.yml OK')"`
Expected: Prints `ci.yml OK`
