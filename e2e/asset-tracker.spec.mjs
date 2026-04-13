import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'https://assets.assettracker.tech';
const API_URL = BASE_URL + '/api';
const MAILPIT_URL = process.env.MAILPIT_URL || BASE_URL.replace(/:\d+$/, ':8025');

// Helper: fetch the latest verification code from Mailpit for a given email
async function getVerificationCode(request, emailAddr) {
  // Wait briefly for the email to arrive
  await new Promise((r) => setTimeout(r, 1000));

  const resp = await request.get(`${MAILPIT_URL}/api/v1/messages?limit=5`);
  expect(resp.status()).toBe(200);
  const data = await resp.json();

  // Find the message sent to our email address
  const message = data.messages.find((m) =>
    m.To.some((to) => to.Address === emailAddr)
  );
  expect(message).toBeTruthy();

  // Get the full message to read the body
  const msgResp = await request.get(`${MAILPIT_URL}/api/v1/message/${message.ID}`);
  expect(msgResp.status()).toBe(200);
  const msgData = await msgResp.json();

  // Extract 6-digit code from body
  const match = msgData.Text.match(/(\d{6})/);
  expect(match).toBeTruthy();
  return match[1];
}

// Shared state across tests in this file
let token;
let username;
let userId;
const testEmail = `e2e_${Date.now()}@test.assettracker.local`;

test.describe.serial('Asset Tracker', () => {

  test.describe('Health & Infrastructure', () => {
    test('health endpoint returns ok with database connected', async ({ request }) => {
      const resp = await request.get(`${API_URL}/health`);
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.status).toBe('ok');
      expect(body.database).toBe('connected');
      expect(body.timestamp).toBeTruthy();
    });
  });

  test.describe('Authentication', () => {
    test('register a new user', async ({ request }) => {
      username = `e2e_${Date.now()}`;
      const resp = await request.post(`${API_URL}/auth/register`, {
        data: { username, email: testEmail, password: 'TestPass123!' },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(body.user_id).toBeTruthy();
      expect(body.message).toContain('verification');
      userId = body.user_id;
    });

    test('verify email with code from Mailpit', async ({ request }) => {
      const code = await getVerificationCode(request, testEmail);

      const resp = await request.post(`${API_URL}/auth/verify-email`, {
        data: { user_id: userId, code },
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.token).toBeTruthy();
      token = body.token;
    });

    test('login with verified user', async ({ request }) => {
      const resp = await request.post(`${API_URL}/auth/login`, {
        data: { username, password: 'TestPass123!' },
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.token).toBeTruthy();
    });

    test('reject invalid credentials', async ({ request }) => {
      const resp = await request.post(`${API_URL}/auth/login`, {
        data: { username, password: 'wrong' },
      });
      expect(resp.status()).toBe(401);
    });
  });

  test.describe('UI - Login & Register', () => {
    test('login page renders', async ({ page }) => {
      await page.goto('/login');
      await expect(page.locator('h1')).toHaveText('Login');
      await expect(page.locator('.form-group')).toHaveCount(2);
      await expect(page.locator('button[type="submit"]')).toHaveText('Login');
      await expect(page.locator('a:has-text("Register")')).toBeVisible();
    });

    test('register page renders with email field', async ({ page }) => {
      await page.goto('/register');
      await expect(page.locator('h1')).toHaveText('Register');
      await expect(page.locator('.form-group')).toHaveCount(3);
      await expect(page.locator('button[type="submit"]')).toHaveText('Register');
      await expect(page.locator('a:has-text("Login")')).toBeVisible();
    });

    test('redirect to login when not authenticated', async ({ page }) => {
      await page.goto('/assets');
      await expect(page).toHaveURL(/\/login/);
    });
  });

  test.describe('UI - Asset Management', () => {
    test.beforeEach(async ({ page }) => {
      // Inject auth token
      await page.goto('/');
      await page.evaluate((t) => localStorage.setItem('token', t), token);
    });

    test('asset list page renders with Add Asset button', async ({ page }) => {
      await page.goto('/assets');
      await expect(page.locator('h1')).toHaveText('My Assets');
      await expect(page.locator('button:has-text("Add Asset")')).toBeVisible();
      await expect(page.locator('nav.nav-bar button:has-text("Logout")')).toBeVisible();
    });

    test('generate support bundle button is visible and clickable', async ({ page }) => {
      await page.goto('/assets');
      const bundleButton = page.locator('button:has-text("Generate Support Bundle")');
      await expect(bundleButton).toBeVisible();

      await bundleButton.click();

      // Button should show generating state
      await expect(page.locator('button:has-text("Generating...")')).toBeVisible();

      // Wait for the request to complete (success or failure depending on cluster environment)
      await expect(page.locator('button:has-text("Generate Support Bundle")')).toBeVisible({ timeout: 120000 });

      // A status message should appear (either success or failure)
      const statusMessage = page.locator('.success, .error');
      await expect(statusMessage).toBeVisible({ timeout: 5000 });
    });

    test('create an asset via UI', async ({ page }) => {
      await page.goto('/assets');

      await page.locator('button:has-text("Add Asset")').click();
      await expect(page.locator('form')).toBeVisible();

      await page.locator('form .form-group').nth(0).locator('input').type('E2E-ASSET-001', { delay: 10 });
      await page.locator('form .form-group').nth(1).locator('input').type('Test Asset', { delay: 10 });
      await page.locator('form .form-group').nth(2).locator('textarea').type('Created by e2e test', { delay: 10 });

      await page.locator('button:has-text("Create")').click();
      await expect(page.locator('td:has-text("E2E-ASSET-001")')).toBeVisible({ timeout: 5000 });
      await expect(page.locator('td:has-text("Test Asset")')).toBeVisible();
    });

    test('navigate to asset detail page', async ({ page }) => {
      await page.goto('/assets');
      await expect(page.locator('td a:has-text("E2E-ASSET-001")')).toBeVisible({ timeout: 5000 });

      await page.locator('td a:has-text("E2E-ASSET-001")').click();
      await expect(page.locator('h1')).toHaveText('Asset: E2E-ASSET-001');
      await expect(page.locator('button:has-text("Add Value Point")')).toBeVisible();
      await expect(page.locator('button:has-text("Back")')).toBeVisible();
    });

    test('add value points to an asset', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('td a:has-text("E2E-ASSET-001")').click({ timeout: 5000 });
      await expect(page.locator('h1')).toHaveText('Asset: E2E-ASSET-001');

      // Add first value point
      await page.locator('button:has-text("Add Value Point")').click();
      await expect(page.locator('form')).toBeVisible();
      await page.locator('form .form-group').nth(0).locator('input').type('1000', { delay: 10 });
      // Currency defaults to USD
      await page.locator('form button[type="submit"]').click();

      await expect(page.locator('td:has-text("$1,000.00")')).toBeVisible({ timeout: 5000 });
      await expect(page.locator('td:has-text("USD")')).toBeVisible();

      // Add second value point
      await page.locator('button:has-text("Add Value Point")').click();
      await page.locator('form .form-group').nth(0).locator('input').type('1250', { delay: 10 });
      await page.locator('form button[type="submit"]').click();

      await expect(page.locator('td:has-text("$1,250.00")')).toBeVisible({ timeout: 5000 });
    });

    test('back button returns to asset list', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('td a:has-text("E2E-ASSET-001")').click({ timeout: 5000 });
      await expect(page.locator('h1')).toHaveText('Asset: E2E-ASSET-001');

      await page.locator('button:has-text("Back")').click();
      await expect(page.locator('h1')).toHaveText('My Assets');
    });

    test('logout returns to login page', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('nav.nav-bar button:has-text("Logout")').click();
      await expect(page).toHaveURL(/\/login/);
    });
  });

  test.describe('API - Asset CRUD', () => {
    const authHeaders = () => ({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    });

    test('create asset via API', async ({ request }) => {
      const resp = await request.post(`${API_URL}/assets`, {
        headers: authHeaders(),
        data: { id: 'API-TEST-001', name: 'API Test Asset', description: 'Created via API' },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(body.id).toBe('API-TEST-001');
      expect(body.name).toBe('API Test Asset');
      expect(body.created_at).toBeTruthy();
    });

    test('list assets returns created assets', async ({ request }) => {
      const resp = await request.get(`${API_URL}/assets`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const assets = await resp.json();
      expect(assets.length).toBeGreaterThanOrEqual(2);
      const ids = assets.map(a => a.id);
      expect(ids).toContain('API-TEST-001');
      expect(ids).toContain('E2E-ASSET-001');
    });

    test('update asset via API', async ({ request }) => {
      const resp = await request.put(`${API_URL}/assets/API-TEST-001`, {
        headers: authHeaders(),
        data: { name: 'Updated Name', description: 'Updated description' },
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.name).toBe('Updated Name');
    });

    test('create value point via API', async ({ request }) => {
      const resp = await request.post(`${API_URL}/assets/API-TEST-001/values`, {
        headers: authHeaders(),
        data: { value: 5000, currency: 'EUR' },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(Number(body.value)).toBe(5000);
      expect(body.currency).toBe('EUR');
      expect(body.timestamp).toBeTruthy();
    });

    test('list value points returns created values', async ({ request }) => {
      const resp = await request.get(`${API_URL}/assets/API-TEST-001/values`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const values = await resp.json();
      expect(values.length).toBe(1);
      expect(values[0].currency).toBe('EUR');
    });

    test('delete asset via API', async ({ request }) => {
      const resp = await request.delete(`${API_URL}/assets/API-TEST-001`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(204);

      // Verify it's gone
      const listResp = await request.get(`${API_URL}/assets`, {
        headers: authHeaders(),
      });
      const assets = await listResp.json();
      const ids = assets.map(a => a.id);
      expect(ids).not.toContain('API-TEST-001');
    });

    test('reject unauthenticated requests', async ({ request }) => {
      const resp = await request.get(`${API_URL}/assets`);
      expect(resp.status()).toBe(401);
    });
  });

  test.describe('API - Exchange Rates', () => {
    const authHeaders = () => ({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    });

    test('create exchange rate via API', async ({ request }) => {
      const resp = await request.post(`${API_URL}/exchange-rates`, {
        headers: authHeaders(),
        data: { base_currency: 'USD', target_currency: 'EUR', rate: 0.92 },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(body.base_currency).toBe('USD');
      expect(body.target_currency).toBe('EUR');
      expect(Number(body.rate)).toBeCloseTo(0.92);
    });

    test('list exchange rates returns created rate', async ({ request }) => {
      const resp = await request.get(`${API_URL}/exchange-rates`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const rates = await resp.json();
      expect(rates.length).toBeGreaterThanOrEqual(1);
      const usdEur = rates.find(r => r.base_currency === 'USD' && r.target_currency === 'EUR');
      expect(usdEur).toBeTruthy();
    });

    test('upsert exchange rate updates existing', async ({ request }) => {
      const resp = await request.post(`${API_URL}/exchange-rates`, {
        headers: authHeaders(),
        data: { base_currency: 'USD', target_currency: 'EUR', rate: 0.95 },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(Number(body.rate)).toBeCloseTo(0.95);
    });

    test('reject unauthenticated exchange rate requests', async ({ request }) => {
      const resp = await request.get(`${API_URL}/exchange-rates`);
      expect(resp.status()).toBe(401);
    });
  });

  test.describe('API - Analytics', () => {
    const authHeaders = () => ({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    });

    test('portfolio analytics returns data in requested currency', async ({ request }) => {
      const resp = await request.get(`${API_URL}/analytics/portfolio?currency=USD`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.currency).toBe('USD');
      expect(typeof body.total_value).toBe('number');
      expect(Array.isArray(body.series)).toBe(true);
    });

    test('portfolio analytics returns empty series when no data', async ({ request }) => {
      const resp = await request.get(`${API_URL}/analytics/portfolio?currency=JPY`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.currency).toBe('JPY');
      expect(Array.isArray(body.series)).toBe(true);
    });

    test('reject unauthenticated analytics requests', async ({ request }) => {
      const resp = await request.get(`${API_URL}/analytics/portfolio?currency=USD`);
      expect(resp.status()).toBe(401);
    });
  });

  test.describe('UI - Analytics', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/');
      await page.evaluate((t) => localStorage.setItem('token', t), token);
    });

    test('nav bar is visible on assets page', async ({ page }) => {
      await page.goto('/assets');
      await expect(page.locator('nav.nav-bar')).toBeVisible();
      await expect(page.locator('a.nav-link:has-text("Assets")')).toBeVisible();
      await expect(page.locator('a.nav-link:has-text("Analytics")')).toBeVisible();
      await expect(page.locator('a.nav-link:has-text("Exchange Rates")')).toBeVisible();
    });

    test('navigate to analytics page via nav bar', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('a.nav-link:has-text("Analytics")').click();
      await expect(page.locator('h1')).toHaveText('Analytics');
    });

    test('analytics page shows portfolio value', async ({ page }) => {
      await page.goto('/analytics');
      await expect(page.locator('.summary-card')).toBeVisible({ timeout: 5000 });
      await expect(page.locator('.summary-label')).toHaveText('Total Portfolio Value');
    });

    test('navigate to exchange rates page via nav bar', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('a.nav-link:has-text("Exchange Rates")').click();
      await expect(page.locator('h1')).toHaveText('Exchange Rates');
    });

    test('exchange rates page shows fetch button', async ({ page }) => {
      await page.goto('/exchange-rates');
      await expect(page.locator('button:has-text("Fetch Current Rates")')).toBeVisible();
      await expect(page.locator('button:has-text("Add Rate")')).toBeVisible();
    });
  });

  test.describe('Linked Accounts', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/');
      await page.evaluate((t) => localStorage.setItem('token', t), token);
    });

    test('linked accounts nav link is visible when integrations are enabled', async ({ page }) => {
      await page.goto('/assets');

      const navLink = page.locator('a[href="/linked-accounts"]');
      await expect(navLink).toBeVisible();
      await expect(navLink).toHaveText('Linked Accounts');
    });

    test('linked accounts page shows link buttons', async ({ page }) => {
      await page.goto('/linked-accounts');
      await expect(page.locator('h1')).toHaveText('Linked Accounts');

      // Wait for at least one link button to appear (features load async)
      await expect(page.locator('button:has-text("Link with Plaid"), button:has-text("Link with Teller")').first()).toBeVisible();
    });

    test('linked accounts table shows empty state', async ({ page }) => {
      await page.goto('/linked-accounts');
      await expect(page.locator('text=No linked accounts yet')).toBeVisible();
    });
  });
});
