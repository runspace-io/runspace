import { chromium } from '@playwright/test';

/**
 * Warms the dev server's routes before the suite runs.
 *
 * `next dev` compiles a route the first time it is requested, which can take
 * tens of seconds right after the web container restarts. Whichever test
 * happened to touch a route first paid that cost and failed on an otherwise
 * reasonable timeout, so the suite looked flaky when it was only cold.
 *
 * Signing in once here compiles the routes behind the auth middleware too —
 * they cannot be reached, let alone compiled, while signed out.
 */
export default async function globalSetup() {
  const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:3000';
  const browser = await chromium.launch();
  const page = await browser.newPage({ baseURL });
  page.setDefaultTimeout(120_000);
  try {
    await page.goto('/signin', { timeout: 120_000 });
    await page.getByLabel('Username').fill('admin');
    await page.getByLabel('Password').fill('admin');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL(/\/$/u, { timeout: 120_000 });
    // Compile the invitation route as well; an invalid token is enough, the
    // page still has to be built to render its "cannot be used" state.
    await page.goto('/invite/warmup-not-a-real-token', { timeout: 120_000 });
    await page.waitForLoadState('networkidle', { timeout: 120_000 });
  } finally {
    await browser.close();
  }
}
