import { expect, test } from '@playwright/test';

test('configured member account can sign in and access its shared workspace', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/signin/);

  const workspacesLoaded = page.waitForResponse(
    (response) =>
      response.url().endsWith('/gateway/workspaces') &&
      response.request().method() === 'GET' &&
      response.ok(),
  );
  await page.getByLabel('Username').fill(process.env.E2E_MEMBER_USERNAME ?? 'nahid');
  await page.getByLabel('Password').fill(process.env.E2E_MEMBER_PASSWORD ?? 'nahid123');
  await page.getByRole('button', { name: 'Sign in' }).click();

  await expect(page).toHaveURL(/\/$/, { timeout: 15_000 });
  const response = await workspacesLoaded;
  const payload = (await response.json()) as { workspaces: Array<{ id: string }> };
  expect(payload.workspaces.some((workspace) => workspace.id === 'ws_2')).toBe(true);
  await expect(page.getByRole('button', { name: 'Switch workspace' })).toBeVisible();
});
