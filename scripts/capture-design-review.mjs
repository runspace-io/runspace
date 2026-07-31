import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { chromium } from '@playwright/test';

const baseURL = process.env.RUNSPACE_URL ?? 'http://localhost:3000';
const output = resolve('.design/mvp-engineering-workspace/screenshots');
await mkdir(output, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({
  viewport: { width: 1280, height: 800 },
  reducedMotion: 'reduce',
});
await page.goto(baseURL);
if (page.url().includes('/signin')) {
  await page.getByLabel('Username').fill('admin');
  await page.getByLabel('Password').fill('admin');
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForURL(`${baseURL}/`, { timeout: 15_000 });
}
await page.getByRole('button', { name: 'Switch workspace' }).click();
const workspaceName = await findPopulatedWorkspace(page);
const workspace = page.locator('.dialog-list-item').filter({ hasText: workspaceName }).last();
await workspace.click();
const firstChannel = page
  .getByRole('tree', { name: 'Workspace channels' })
  .locator('button[aria-label^="Open "]')
  .first();
await firstChannel.waitFor({ state: 'visible' });
await firstChannel.click();
await page.waitForLoadState('networkidle');

const changesButton = page.getByRole('button', { name: 'Changes', exact: true });
if (await changesButton.isEnabled()) {
  await changesButton.click();
  await page.getByRole('region', { name: 'Repository changes' }).waitFor();
  await page.locator('.change-diff .monaco-diff-editor').waitFor({ state: 'visible' });
  await page.screenshot({
    path: resolve(output, 'review-workspace-desktop-changes-1280.png'),
    fullPage: true,
  });
  await changesButton.click();
}

for (const viewport of [
  { width: 1280, height: 800, suffix: 'desktop-1280' },
  { width: 768, height: 1024, suffix: 'tablet-768' },
  { width: 375, height: 812, suffix: 'mobile-375' },
]) {
  await page.setViewportSize(viewport);
  await page.screenshot({
    path: resolve(output, `review-workspace-${viewport.suffix}.png`),
    fullPage: true,
  });
  if (viewport.width <= 900) {
    await page.getByRole('button', { name: 'Open navigation' }).click();
    await page.waitForFunction(
      () =>
        document.querySelector('.left-rail')?.classList.contains('is-open') &&
        document.querySelector('.left-rail')?.getBoundingClientRect().left === 0,
    );
    await page.screenshot({
      path: resolve(output, `review-workspace-${viewport.suffix}-navigation-open.png`),
      fullPage: true,
    });
    await page.getByRole('button', { name: 'Close navigation' }).click();
    await page.waitForFunction(
      () =>
        !document.querySelector('.left-rail')?.classList.contains('is-open') &&
        (document.querySelector('.left-rail')?.getBoundingClientRect().right ?? 0) <= 0,
    );
  }
}

await browser.close();

async function findPopulatedWorkspace(currentPage) {
  const headers = { 'x-user-id': 'admin' };
  const response = await currentPage.request.get(`${baseURL}/gateway/workspaces`, { headers });
  const { workspaces } = await response.json();
  for (const workspace of [...workspaces].reverse()) {
    const channelsResponse = await currentPage.request.get(
      `${baseURL}/gateway/workspaces/${encodeURIComponent(workspace.id)}/channels`,
      { headers },
    );
    const { channels } = await channelsResponse.json();
    if (channels?.length) return workspace.name;
  }
  throw new Error('No workspace with a channel is available for design review');
}
