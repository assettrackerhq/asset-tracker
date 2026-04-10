import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'https://assets.assettracker.tech';
const API_URL = BASE_URL + '/api';

const TEST_USERNAME = process.env.TEST_USERNAME || 'upgrade-test-user';
const TEST_PASSWORD = process.env.TEST_PASSWORD || 'UpgradeTest123!';

let token;

test.describe.serial('EC Upgrade - Verify Data After Upgrade', () => {

  test('health endpoint is ok', async ({ request }) => {
    const resp = await request.get(`${API_URL}/health`);
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.status).toBe('ok');
    expect(body.database).toBe('connected');
  });

  test('login with pre-upgrade user', async ({ request }) => {
    const resp = await request.post(`${API_URL}/auth/login`, {
      data: { username: TEST_USERNAME, password: TEST_PASSWORD },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.token).toBeTruthy();
    token = body.token;
  });

  test('asset created before upgrade still exists', async ({ request }) => {
    const resp = await request.get(`${API_URL}/assets`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(resp.status()).toBe(200);
    const assets = await resp.json();
    const asset = assets.find((a) => a.id === 'UPGRADE-TEST-001');
    expect(asset).toBeTruthy();
    expect(asset.name).toBe('Upgrade Test Asset');
    expect(asset.description).toBe('Created before upgrade');
  });

  test('value point created before upgrade still exists', async ({ request }) => {
    const resp = await request.get(`${API_URL}/assets/UPGRADE-TEST-001/values`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(resp.status()).toBe(200);
    const values = await resp.json();
    expect(values.length).toBe(1);
    expect(Number(values[0].value)).toBe(42000);
    expect(values[0].currency).toBe('USD');
  });
});
