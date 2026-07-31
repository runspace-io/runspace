import { expect, type Browser } from '@playwright/test';

export async function sendCollaboratorMessage(input: {
  browser: Browser;
  workspaceName: string;
  channelName: string;
  message: string;
}) {
  const context = await input.browser.newContext();
  const page = await context.newPage();
  try {
    await page.goto('/');
    await page.getByLabel('Username').fill('alice');
    await page.getByLabel('Password').fill('alice');
    const workspacesLoaded = page.waitForResponse(
      (response) =>
        response.url().endsWith('/gateway/workspaces') &&
        response.request().method() === 'GET' &&
        response.ok(),
    );
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page).toHaveURL(/\/$/, { timeout: 15_000 });
    await workspacesLoaded;
    const switcher = page.getByRole('button', { name: 'Switch workspace' });
    await switcher.click();
    const workspaceRow = page.locator('.dialog-list-item').filter({
      has: page.getByText(input.workspaceName, { exact: true }),
    });
    await expect(workspaceRow).toHaveCount(1);
    await workspaceRow.click();
    await expect(switcher).toContainText(input.workspaceName);
    const channel = page.getByRole('button', { name: `Open ${input.channelName}` });
    await expect(channel).toBeVisible();
    await channel.click();
    await expect(page.getByRole('heading', { name: input.channelName })).toBeVisible();
    const composer = page.getByRole('textbox', { name: 'Message this channel' });
    await composer.fill(input.message);
    const sent = page.waitForResponse(
      (response) => response.url().includes('/messages') && response.request().method() === 'POST',
    );
    await page.getByRole('button', { name: 'Send message' }).click();
    expect((await sent).status()).toBe(201);
    await expect(page.getByText(input.message, { exact: true })).toBeVisible();
  } finally {
    await context.close();
  }
}
