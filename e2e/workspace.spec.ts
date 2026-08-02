import { expect, test } from '@playwright/test';
import { createLocalRepositoryFixture, restartGateway } from './local-repository';
import { sendCollaboratorMessage } from './collaboration';
import { fillNewChannelResource, selectNewChannelAgent } from './channel-dialog';
import { openChannelContext } from './channel-context';

test.setTimeout(60_000);

test('local admin can build and collaborate in a channel', async ({ page, browser }) => {
  const repositoryURL = createLocalRepositoryFixture();
  const workspaceName = `E2E Workspace ${Date.now()}`;
  await page.goto('/');
  await expect(page).toHaveURL(/\/signin/);
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
  const switcher = page.getByRole('button', { name: 'Switch workspace' });
  await expect(switcher).toBeVisible();
  await switcher.click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await page.getByTestId('new-workspace-button').click();
  await expect(page.getByRole('heading', { name: 'Create workspace' })).toBeVisible();
  await page.getByLabel('Workspace name').fill(workspaceName);
  const creation = page.waitForResponse(
    (response) => response.url().includes('/workspaces') && response.request().method() === 'POST',
  );
  await page.getByRole('button', { name: 'Create workspace' }).click();
  expect((await creation).status()).toBe(201);

  await expect(page.getByRole('button', { name: 'Switch workspace' })).toContainText(workspaceName);
  await page.getByRole('button', { name: 'Switch workspace' }).click();
  await expect(page.getByRole('button', { name: new RegExp(workspaceName) })).toBeVisible();
  await page.getByRole('button', { name: 'Close workspace dialog' }).click();
  await expect(page.locator('.left-rail').getByText('Files', { exact: true })).toHaveCount(0);

  const channelName = `general-${Date.now()}`;
  await page.getByRole('button', { name: 'Add channel' }).click();
  const channelDialog = page.getByRole('dialog');
  await expect(channelDialog.getByRole('heading', { name: 'Create channel' })).toBeVisible();
  await channelDialog.getByLabel('Channel name').fill(channelName);
  await fillNewChannelResource(channelDialog, repositoryURL);
  await selectNewChannelAgent(channelDialog, 'mock');
  const repositoryConnected = page.waitForResponse(
    (response) => response.url().endsWith('/resources') && response.request().method() === 'POST',
  );
  const repositoryCloned = page.waitForResponse(
    (response) => response.url().endsWith('/clone') && response.request().method() === 'POST',
  );
  const channelCreated = page.waitForResponse(
    (response) => response.url().includes('/channels') && response.request().method() === 'POST',
  );
  await channelDialog.getByRole('button', { name: 'Create channel' }).click();
  expect((await repositoryConnected).status()).toBe(201);
  expect((await repositoryCloned).status()).toBe(202);
  expect((await channelCreated).status()).toBe(201);
  const channel = page.getByRole('button', { name: `Open ${channelName}` });
  await expect(channel).toBeVisible();
  await channel.click();
  await expect(page.getByText(new RegExp(`CHANNEL /`))).toBeVisible();
  const channelContext = await openChannelContext(page);
  await expect(channelContext.getByRole('heading', { name: 'Resources' })).toBeVisible();
  await expect(channelContext.getByText('Built-in agent', { exact: true })).toBeVisible();
  await expect(page.locator('.left-rail').getByText('Files', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: /Resource Center/ }).click();
  await expect(page.getByRole('heading', { name: 'Resource Center' })).toBeVisible();
  await expect(page.getByPlaceholder('Search shared resources')).toBeVisible();
  await expect(page.getByText('Shared by Admin').first()).toBeVisible();
  await channel.click();
  await channelContext.getByRole('button', { name: 'Connect resource' }).click();
  const repositoryDialog = page.getByRole('dialog');
  await expect(repositoryDialog.getByRole('heading', { name: 'Connect resource' })).toBeVisible();
  await expect(repositoryDialog.getByRole('button', { name: /Remote Git/ })).toBeVisible();
  await repositoryDialog.getByRole('button', { name: /Local folder/ }).click();
  await expect(repositoryDialog.getByLabel('Local resource path')).toBeVisible();
  await expect(repositoryDialog.getByText(/No upload or Docker mount is required/)).toBeVisible();
  await repositoryDialog.getByRole('button', { name: 'Close connection dialog' }).click();

  await channelContext.getByRole('button', { name: 'Change agent connection' }).click();
  const agentDialog = page.getByRole('dialog');
  await expect(agentDialog.getByRole('heading', { name: 'Connect agent' })).toBeVisible();
  await expect(agentDialog.getByRole('radio', { name: /OpenCode/ })).toBeVisible();
  await expect(agentDialog.getByRole('radio', { name: /Codex/ })).toBeVisible();
  await expect(agentDialog.getByRole('radio', { name: /Claude Agent/ })).toBeVisible();
  await expect(agentDialog.getByLabel('Agent model')).toBeVisible();
  const permissionMode = agentDialog.getByLabel('Agent permission mode');
  await permissionMode.selectOption('yolo');
  await expect(
    agentDialog.getByText(/execute commands and modify files without asking/),
  ).toBeVisible();
  await permissionMode.selectOption('default');
  const agentSettingsSaved = page.waitForResponse(
    (response) => response.url().includes('/channels/') && response.request().method() === 'PATCH',
  );
  await agentDialog.getByRole('button', { name: 'Set active collaborator' }).click();
  expect((await agentSettingsSaved).status()).toBe(200);
  await expect(agentDialog).toBeHidden();
  await expect(channelContext.getByRole('button', { name: 'New agent chat' })).toBeVisible();
  await expect(channelContext.getByText('OpenCode', { exact: true })).toBeVisible();
  await expect(channelContext.getByText(/Admin-owned · Host Agent · ready/)).toBeVisible();
  const navigationPanel = page.locator('.navigation-panel');
  const initialNavigationWidth = (await navigationPanel.boundingBox())?.width ?? 0;
  const resizeHandle = page.getByRole('separator', { name: 'Resize navigation panel' });
  const resizeBox = await resizeHandle.boundingBox();
  expect(resizeBox).toBeTruthy();
  await page.mouse.move(resizeBox!.x + resizeBox!.width / 2, resizeBox!.y + 40);
  await page.mouse.down();
  await page.mouse.move(resizeBox!.x + 54, resizeBox!.y + 40);
  await page.mouse.up();
  await expect
    .poll(async () => (await navigationPanel.boundingBox())?.width ?? 0)
    .toBeGreaterThan(initialNavigationWidth + 20);
  const persistedNavigationWidth = (await navigationPanel.boundingBox())?.width ?? 0;
  const codeButton = page.getByRole('button', { name: 'Code' });
  const terminalButton = page.getByRole('button', { name: 'Terminal' });
  await expect(codeButton).toBeEnabled();
  await expect(terminalButton).toBeEnabled();
  await page.getByRole('treeitem', { name: 'README.md' }).click();
  await expect(page.getByLabel('Code preview')).toContainText('README.md');
  const sourceDirectory = page.getByRole('treeitem', { name: 'src' });
  await sourceDirectory.click();
  await expect(sourceDirectory).toHaveAttribute('aria-expanded', 'true');
  await page.getByRole('treeitem', { name: 'index.ts' }).click();
  await expect(page.getByLabel('Code preview')).toContainText('src/index.ts');
  await sourceDirectory.click();
  await expect(sourceDirectory).toHaveAttribute('aria-expanded', 'false');
  await expect(page.getByRole('treeitem', { name: 'index.ts' })).toHaveCount(0);
  const terminalConnected = page.waitForEvent('websocket', (socket) =>
    socket.url().includes('/terminal'),
  );
  await terminalButton.click();
  const terminalDialog = page.getByRole('dialog');
  await expect(terminalDialog.getByRole('heading', { name: 'Open terminal' })).toBeVisible();
  await terminalDialog.getByRole('button', { name: /Workspace terminal/ }).click();
  await terminalConnected;
  const terminal = page.getByLabel('Agent terminal');
  await expect(terminal).toBeVisible();
  const terminalRows = terminal.locator('.xterm-rows');
  await expect(terminalRows).toContainText('$ connected');
  await terminal.locator('.xterm-helper-textarea').focus();
  await page.keyboard.press('KeyP');
  await page.keyboard.press('KeyW');
  await page.keyboard.press('KeyD');
  await page.keyboard.press('Enter');
  await expect(terminalRows).toContainText('/var/lib/runspace/repositories/');
  await expect(terminalRows).not.toContainText("can't open");
  await terminal.locator('.xterm-helper-textarea').focus();
  await page.keyboard.type("printf '\\nE2E change\\n' >> README.md && echo RUNSPACE_CHANGE_READY");
  await page.keyboard.press('Enter');
  await expect(terminalRows).toContainText('RUNSPACE_CHANGE_READY');
  const secondTerminalConnected = page.waitForEvent('websocket', (socket) =>
    socket.url().includes('/terminal'),
  );
  await page.getByRole('button', { name: 'New terminal' }).click();
  await page
    .getByRole('dialog')
    .getByRole('button', { name: /Workspace terminal/ })
    .click();
  await secondTerminalConnected;
  await expect(page.getByRole('tablist', { name: 'Open terminals' }).getByRole('tab')).toHaveCount(
    2,
  );

  const changesLoaded = page.waitForResponse(
    (response) => response.url().endsWith('/changes') && response.ok(),
  );
  const diffLoaded = page.waitForResponse(
    (response) => response.url().includes('/diff?path=README.md') && response.ok(),
  );
  await page.getByRole('button', { name: 'Changes' }).click();
  const changesBody = (await (await changesLoaded).json()) as {
    changes: Array<{ path: string; status: string }>;
  };
  expect(changesBody.changes).toContainEqual({ path: 'README.md', status: 'modified' });
  const diffBody = (await (await diffLoaded).json()) as {
    path: string;
    original: string;
    modified: string;
  };
  expect(diffBody.path).toBe('README.md');
  expect(diffBody.original).not.toContain('E2E change');
  expect(diffBody.modified).toContain('E2E change');
  await expect(page.getByRole('region', { name: 'Resource changes' })).toBeVisible();
  await expect(page.getByRole('button', { name: /M README\.md/ })).toBeVisible();

  const message = `Hello from Playwright ${Date.now()}`;
  const composer = page.getByRole('textbox', { name: 'Message this channel' });
  await expect(composer).toBeVisible();
  await composer.fill(message);
  const sent = page.waitForResponse(
    (response) => response.url().includes('/messages') && response.request().method() === 'POST',
  );
  await page.getByRole('button', { name: 'Send message' }).click();
  expect((await sent).status()).toBe(201);
  await expect(page.getByText(message)).toBeVisible();

  const localAgentReply = `Local ACP reply ${Date.now()}`;
  let localChatID = '';
  let localChatTitle = '';
  let localChatResourceID = '';
  let localChatAgentID = '';
  const privateTaskMessages: Array<{
    id: string;
    role: 'user' | 'agent';
    body: string;
    created_at: string;
  }> = [];
  await page.route('http://127.0.0.1:7799/v1/agents/*/session?*', async (route) => {
    const requestURL = new URL(route.request().url());
    localChatResourceID = requestURL.searchParams.get('resource_id') ?? '';
    localChatAgentID = requestURL.pathname.split('/agents/')[1]?.split('/')[0] ?? '';
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: requestURL.searchParams.get('task_id'),
        title: localChatTitle,
        agent_id: localChatAgentID,
        resource_id: requestURL.searchParams.get('resource_id'),
        thread_id: requestURL.searchParams.get('thread_id'),
        status: privateTaskMessages.length > 0 ? 'completed' : 'draft',
        pause_support: 'cancel-only',
        messages: privateTaskMessages,
      }),
    });
  });
  await page.route('http://127.0.0.1:7799/v1/agents/*/prompt', async (route) => {
    const payload = route.request().postDataJSON() as { prompt: string; task_id: string };
    localChatID = payload.task_id;
    localChatTitle = payload.prompt;
    const now = new Date().toISOString();
    privateTaskMessages.push(
      { id: 'private-user-1', role: 'user', body: payload.prompt, created_at: now },
      { id: 'private-agent-1', role: 'agent', body: localAgentReply, created_at: now },
    );
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        session_id: payload.task_id,
        outputs: [{ kind: 'agent_message_chunk', text: localAgentReply }],
      }),
    });
  });
  await page.route('http://127.0.0.1:7799/v1/agent-chats?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        chats: localChatID
          ? [
              {
                id: localChatID,
                title: localChatTitle,
                agent_id: localChatAgentID,
                resource_id: localChatResourceID,
                status: 'completed',
                updated_at: new Date().toISOString(),
              },
            ]
          : [],
      }),
    });
  });
  await channelContext.getByRole('button', { name: 'New agent chat' }).click();
  const taskWorkspace = page.locator('.agent-task-surface');
  await expect(taskWorkspace.getByRole('heading', { name: 'New agent chat' })).toBeVisible();
  await expect(taskWorkspace.getByText('Private · share explicitly')).toBeVisible();
  const agentPrompt = `Agent task ${Date.now()}`;
  await taskWorkspace.getByLabel('Instruction').fill(agentPrompt);
  await taskWorkspace.getByRole('button', { name: 'Start chat' }).click();
  await expect(taskWorkspace.getByRole('heading', { name: agentPrompt })).toBeVisible();
  await expect(taskWorkspace.getByText(localAgentReply, { exact: true })).toBeVisible();
  await expect(
    taskWorkspace.getByRole('list', { name: 'Private agent chat' }).getByText(agentPrompt),
  ).toBeVisible();
  const chatShared = page.waitForResponse(
    (response) => response.url().includes('/agent-tasks/') && response.request().method() === 'PUT',
  );
  await channelContext.getByRole('button', { name: `Share ${agentPrompt} with channel` }).click();
  const chatSharedResponse = await chatShared;
  expect(chatSharedResponse.status(), await chatSharedResponse.text()).toBe(200);
  await expect(channelContext.getByText('Shared with this channel', { exact: true })).toBeVisible();
  await expect(channelContext.getByRole('button', { name: new RegExp(agentPrompt) })).toBeVisible();
  await expect(page.getByRole('button', { name: `Open shared task ${agentPrompt}` })).toBeVisible();
  const agentMessageCreated = page.waitForResponse(
    (response) =>
      response.url().includes('/agent-messages') && response.request().method() === 'POST',
  );
  await taskWorkspace.getByRole('button', { name: 'Share', exact: true }).click();
  expect((await agentMessageCreated).status()).toBe(201);
  await taskWorkspace.getByRole('button', { name: 'Back to channel' }).click();
  await expect(page.getByText(localAgentReply)).toBeVisible();
  await expect(page.locator('.timeline').getByText(agentPrompt, { exact: true })).toHaveCount(0);

  await page.getByRole('button', { name: 'Channel settings', exact: true }).click();
  const settings = page.getByRole('dialog');
  await expect(settings.getByRole('heading', { name: channelName })).toBeVisible();
  const secretName = `E2E_KEY_${Date.now()}`;
  await settings.getByLabel('Secret name').fill(secretName);
  await settings.getByLabel('Secret value').fill('e2e-secret-value');
  const secretSaved = page.waitForResponse(
    (response) => response.url().includes('/secrets/') && response.request().method() === 'PUT',
  );
  await settings.getByRole('button', { name: 'Add secret' }).click();
  expect((await secretSaved).status()).toBe(204);
  await expect(settings.getByText(secretName)).toBeVisible();
  const renamedChannel = `${channelName}-renamed`;
  await settings.getByLabel('Channel name').fill(renamedChannel);
  const settingsSaved = page.waitForResponse(
    (response) => response.url().includes('/channels/') && response.request().method() === 'PATCH',
  );
  await settings.getByRole('button', { name: 'Save settings' }).click();
  expect((await settingsSaved).status()).toBe(200);
  await expect(page.getByRole('heading', { name: renamedChannel })).toBeVisible();

  await page.getByRole('button', { name: 'Members' }).click();
  const members = page.getByRole('dialog');
  await expect(members.getByText('admin', { exact: true })).toBeVisible();
  const memberID = 'alice';
  await members.getByLabel('Member ID').fill(memberID);
  const memberAdded = page.waitForResponse(
    (response) => response.url().endsWith('/members') && response.request().method() === 'POST',
  );
  await members.getByRole('button', { name: 'Add member' }).click();
  expect((await memberAdded).status()).toBe(201);
  await expect(members.getByText(memberID, { exact: true })).toBeVisible();
  await members.getByRole('button', { name: 'Close members' }).click();

  await page.getByRole('button', { name: `Open shared task ${agentPrompt}` }).click();
  const sharedTaskWorkspace = page.locator('.agent-task-surface');
  await sharedTaskWorkspace.getByRole('button', { name: 'Access' }).click();
  await sharedTaskWorkspace.getByLabel('Chat member').selectOption(memberID);
  await sharedTaskWorkspace.getByLabel('Chat role').selectOption('operator');
  const accessGranted = page.waitForResponse(
    (response) =>
      response.url().includes('/agent-tasks/') &&
      response.url().includes('/grants/') &&
      response.request().method() === 'PUT',
  );
  await sharedTaskWorkspace.getByRole('button', { name: 'Grant access' }).click();
  expect((await accessGranted).status()).toBe(200);
  await expect(sharedTaskWorkspace.getByText('task.control', { exact: false })).toBeVisible();
  await sharedTaskWorkspace.getByRole('button', { name: 'Back to channel' }).click();

  await composer.fill('@ali');
  await expect(page.getByRole('option', { name: /@alice/ })).toBeVisible();
  await composer.press('Enter');
  await expect(composer).toHaveValue('@alice ');
  await composer.fill('');

  const collaboratorMessage = `Alice joined ${Date.now()}`;
  await sendCollaboratorMessage({
    browser,
    workspaceName,
    channelName: renamedChannel,
    message: collaboratorMessage,
  });
  await expect(page.getByText(collaboratorMessage, { exact: true })).toBeVisible();
  await expect(
    page.locator('.message-row-header').getByText('Alice', { exact: true }),
  ).toBeVisible();

  restartGateway();
  await expect
    .poll(async () => {
      try {
        return (await fetch('http://127.0.0.1:8080/healthz')).status;
      } catch {
        return 0;
      }
    })
    .toBe(200);
  const workspaceReloaded = page.waitForResponse(
    (response) =>
      response.url().endsWith('/gateway/workspaces') &&
      response.request().method() === 'GET' &&
      response.ok(),
  );
  await page.reload();
  await workspaceReloaded;
  await expect
    .poll(async () => (await navigationPanel.boundingBox())?.width ?? 0)
    .toBeGreaterThan(persistedNavigationWidth - 5);
  await page.getByRole('button', { name: 'Switch workspace' }).click();
  await page.getByRole('button', { name: new RegExp(workspaceName) }).click();
  const persistedChannel = page.getByRole('button', { name: `Open ${renamedChannel}` });
  await expect(persistedChannel).toBeVisible();
  await persistedChannel.click();
  await expect(page.getByText(message, { exact: true })).toBeVisible();
  await expect(page.getByText(collaboratorMessage, { exact: true })).toBeVisible();
  await expect(page.getByText(localAgentReply, { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Channel settings', exact: true }).click();
  await expect(page.getByRole('dialog').getByText(secretName, { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Close channel settings' }).click();
  const restoredMembersButton = page.getByRole('button', { name: 'Members' });
  await restoredMembersButton.click();
  await expect(page.getByRole('dialog').getByText(memberID, { exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toBeHidden();
  await expect(restoredMembersButton).toBeFocused();
});
