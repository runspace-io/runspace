import { defineConfig, devices } from '@playwright/test';

const APP_URL = process.env.E2E_BASE_URL ?? 'http://localhost:3000';
const LANDING_PORT = process.env.LANDING_PORT ?? '4321';

export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/global-setup.ts',
  timeout: 30_000,
  fullyParallel: false,
  reporter: 'list',
  use: {
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  // The landing site is a separate Hugo build, so it needs its own origin. Both
  // suites previously shared the app's baseURL, which the landing tests could
  // never satisfy.
  webServer: {
    command: 'node scripts/serve-landing.mjs',
    url: `http://localhost:${LANDING_PORT}`,
    reuseExistingServer: true,
    stdout: 'ignore',
  },
  projects: [
    {
      name: 'app',
      testIgnore: /landing\.spec\.ts/,
      use: { ...devices['Desktop Chrome'], baseURL: APP_URL },
    },
    {
      name: 'landing',
      testMatch: /landing\.spec\.ts/,
      use: { ...devices['Desktop Chrome'], baseURL: `http://localhost:${LANDING_PORT}` },
    },
  ],
});
