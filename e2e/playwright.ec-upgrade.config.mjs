import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: 'ec-upgrade-*.spec.mjs',
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
