import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'https://assets.assettracker.tech';
const API_URL = BASE_URL + '/api';
const MAILPIT_URL = process.env.MAILPIT_URL || BASE_URL + '/mailpit';

const TEST_USERNAME = process.env.TEST_USERNAME || 'upgrade-test-user';
const TEST_EMAIL = process.env.TEST_EMAIL || 'upgrade-test@test.assettracker.local';
const TEST_PASSWORD = process.env.TEST_PASSWORD || 'UpgradeTest123!';

// Helper: fetch the latest verification code from Mailpit for a given email
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

test.describe.serial('EC Upgrade - Seed Data', () => {

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

    // Need user_id for verification - get it by attempting login (will fail with 403)
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
      data: { id: 'UPGRADE-TEST-001', name: 'Upgrade Test Asset', description: 'Created before upgrade' },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.id).toBe('UPGRADE-TEST-001');
  });

  test('create value point', async ({ request }) => {
    const resp = await request.post(`${API_URL}/assets/UPGRADE-TEST-001/values`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { value: 42000, currency: 'USD' },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(Number(body.value)).toBe(42000);
  });
});
