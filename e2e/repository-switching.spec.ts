import { expect, test, type Page } from '@playwright/test';
import { createHostFolderFixture, createLocalRepositoryFixture } from './local-repository';
import { fillNewChannelResource } from './channel-dialog';
import { openChannelContext } from './channel-context';

test.setTimeout(60_000);

test('switches repository trees, reads files, and runs visible terminal input', async ({
  page,
}) => {
  const firstURL = createLocalRepositoryFixture({ name: 'first', marker: 'FIRST_REPOSITORY' });
  const secondURL = createLocalRepositoryFixture({ name: 'second', marker: 'SECOND_REPOSITORY' });
  const workspaceName = `Repository switching ${Date.now()}`;
  const channelName = `multi-repository-${Date.now()}`;

  await signIn(page);
  await createWorkspace(page, workspaceName);
  await createChannel(page, channelName, firstURL);

  const context = page.getByRole('complementary', { name: 'Channel context' });
  await context.getByRole('button', { name: 'Connect resource' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('button', { name: /Remote Git/ }).click();
  await dialog.getByLabel('Git HTTPS URL').fill(secondURL);
  const connected = page.waitForResponse(
    (response) => response.url().endsWith('/resources') && response.request().method() === 'POST',
  );
  const cloned = page.waitForResponse(
    (response) => response.url().endsWith('/clone') && response.request().method() === 'POST',
  );
  await dialog.getByRole('button', { name: 'Connect resource' }).click();
  expect((await connected).status()).toBe(201);
  expect((await cloned).status()).toBe(202);
  await dialog.getByRole('button', { name: 'Done' }).click();

  const firstName = repositoryName(firstURL);
  const secondName = repositoryName(secondURL);
  await expect(repositoryItem(context, firstName)).toBeVisible();
  await expect(repositoryItem(context, secondName)).toBeVisible();

  await selectRepositoryAndReadMarker(page, context, secondName, 'SECOND_REPOSITORY');
  await selectRepositoryAndReadMarker(page, context, firstName, 'FIRST_REPOSITORY');
  await selectRepositoryAndReadMarker(page, context, secondName, 'SECOND_REPOSITORY');

  await openWorkspaceTerminal(page, context, secondName);
  const terminal = page.getByLabel('Agent terminal');
  const rows = terminal.locator('.xterm-rows');
  const input = terminal.locator('.xterm-helper-textarea');
  await expect(rows).toContainText('$ connected');
  await input.focus();
  await page.keyboard.type('printf TERMINAL_INPUT_VISIBLE');
  await expect(rows).toContainText('printf TERMINAL_INPUT_VISIBLE');
  await page.keyboard.press('Enter');
  await expect(rows).toContainText('TERMINAL_INPUT_VISIBLE');

  await page.getByRole('button', { name: 'New terminal' }).click();
  const terminalDialog = page.getByRole('dialog');
  await terminalDialog.getByRole('button', { name: /Workspace terminal/ }).click();
  await expect(page.getByRole('tablist', { name: 'Open terminals' }).getByRole('tab')).toHaveCount(
    2,
  );
});

test('switches local mirror trees and runs visible host terminal input', async ({ page }) => {
  const firstPath = createHostFolderFixture({ name: 'alpha', marker: 'LOCAL_ALPHA' });
  const secondPath = createHostFolderFixture({ name: 'beta', marker: 'LOCAL_BETA' });
  const workspaceName = `Local mirror switching ${Date.now()}`;
  const channelName = `local-mirrors-${Date.now()}`;

  await signIn(page);
  await createWorkspace(page, workspaceName);
  await page.getByRole('button', { name: 'Add channel' }).click();
  const channelDialog = page.getByRole('dialog');
  await channelDialog.getByLabel('Channel name').fill(channelName);
  await channelDialog.getByRole('button', { name: 'Create channel' }).click();
  await page.getByRole('button', { name: `Open ${channelName}` }).click();

  const context = await openChannelContext(page);
  await connectLocalMirror(page, context, firstPath);
  await connectLocalMirror(page, context, secondPath);

  const firstName = firstPath.split('\\').at(-1) ?? firstPath;
  const secondName = secondPath.split('\\').at(-1) ?? secondPath;
  await selectRepositoryAndReadMarker(page, context, secondName, 'LOCAL_BETA');
  await selectRepositoryAndReadMarker(page, context, firstName, 'LOCAL_ALPHA');

  const repository = repositoryItem(context, firstName);
  await repository.getByRole('button', { name: 'Terminal' }).click();
  const terminalDialog = page.getByRole('dialog');
  await expect(terminalDialog.getByRole('button', { name: /Host terminal · User/ })).toBeEnabled();
  await terminalDialog.getByRole('button', { name: /Host terminal · User/ }).click();
  const terminal = page.getByLabel('Agent terminal');
  const rows = terminal.locator('.xterm-rows');
  const input = terminal.locator('.xterm-helper-textarea');
  await expect(rows).toContainText('$ connected');
  await input.focus();
  await page.keyboard.type('Write-Output HOST_INPUT_VISIBLE');
  await expect(rows).toContainText('Write-Output HOST_INPUT_VISIBLE');
  await page.keyboard.press('Enter');
  await expect(rows).toContainText('HOST_INPUT_VISIBLE');
});

async function signIn(page: Page) {
  await page.goto('/');
  const workspacesLoaded = page.waitForResponse(
    (response) =>
      response.url().endsWith('/gateway/workspaces') &&
      response.request().method() === 'GET' &&
      response.ok(),
  );
  await page.getByLabel('Username').fill('admin');
  await page.getByLabel('Password').fill('admin');
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page).toHaveURL(/\/$/, { timeout: 15_000 });
  await workspacesLoaded;
}

async function createWorkspace(page: Page, name: string) {
  await page.getByRole('button', { name: 'Switch workspace' }).click();
  await page.getByTestId('new-workspace-button').click();
  await page.getByLabel('Workspace name').fill(name);
  await page.getByRole('button', { name: 'Create workspace' }).click();
  await expect(page.getByRole('button', { name: 'Switch workspace' })).toContainText(name);
}

async function createChannel(page: Page, name: string, repositoryURL: string) {
  await page.getByRole('button', { name: 'Add channel' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Channel name').fill(name);
  await fillNewChannelResource(dialog, repositoryURL);
  await dialog.getByRole('button', { name: 'Create channel' }).click();
  await page.getByRole('button', { name: `Open ${name}` }).click();
  await openChannelContext(page);
}

async function connectLocalMirror(
  page: Page,
  context: ReturnType<Page['getByRole']>,
  path: string,
) {
  await context.getByRole('button', { name: 'Connect resource' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('button', { name: /Local folder/ }).click();
  await dialog.getByLabel('Local resource path').fill(path);
  const connected = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === '/v1/resources' &&
      response.request().method() === 'POST',
  );
  await dialog.getByRole('button', { name: 'Connect resource' }).click();
  expect((await connected).status()).toBe(201);
  await dialog.getByRole('button', { name: 'Done' }).click();
}

async function selectRepositoryAndReadMarker(
  page: Page,
  context: ReturnType<Page['getByRole']>,
  repositoryName: string,
  marker: string,
) {
  await repositoryItem(context, repositoryName).locator('.context-option').click();
  await expect(page.getByRole('treeitem', { name: 'README.md' })).toBeVisible();
  await page.getByRole('treeitem', { name: 'README.md' }).click();
  await expect(page.locator('.view-lines')).toContainText(marker);
}

async function openWorkspaceTerminal(
  page: Page,
  context: ReturnType<Page['getByRole']>,
  repositoryName: string,
) {
  const repository = repositoryItem(context, repositoryName);
  await repository.getByRole('button', { name: 'Terminal' }).click();
  await page
    .getByRole('dialog')
    .getByRole('button', { name: /Workspace terminal/ })
    .click();
}

function repositoryName(url: string): string {
  return url.split('/').at(-1) ?? url;
}

function repositoryItem(context: ReturnType<Page['getByRole']>, repositoryName: string) {
  return context.locator('.repository-context-item').filter({ hasText: repositoryName });
}
