import { expect, test, type Page } from '@playwright/test';
import { createLocalRepositoryFixture } from './local-repository';
import { fillNewChannelResource, selectNewChannelAgent } from './channel-dialog';
import { openAgentPopover } from './channel-header-popovers';

test.setTimeout(60_000);

const AGENT_ID = 'local_agent_e2e_activity';

/**
 * Drives the Host Agent mock through a deliberately slow turn — a real ACP
 * agent streams tool calls and prose over many seconds, and this is the one
 * spec that actually exercises the polling window in between, rather than
 * resolving so fast the live status line never has a chance to render.
 */
async function stubSlowHostAgent(page: Page) {
  // The very first GET /session happens on mount, before the user has clicked
  // anything — it must read as an empty draft, not a chat with history
  // already in it, or the composer never shows "Start chat" to begin with.
  let promptStarted = false;
  let promptFinished = false;
  let pollsSincePromptStarted = 0;
  const userMessage = { id: 'm1', role: 'user', body: 'run the tests', created_at: '' };
  const toolCall = { id: 'm2', role: 'agent', kind: 'tool_call', body: 'npm test', created_at: '' };
  const reply = { id: 'm3', role: 'agent', body: 'All tests pass.', created_at: '' };

  await page.route('http://127.0.0.1:7799/v1/agents/discover*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: [
          {
            id: AGENT_ID,
            registry_id: 'opencode',
            name: 'OpenCode',
            description: '',
            protocol: 'acp',
            placement: 'host',
            status: 'ready',
            capabilities: [],
            permission_mode: 'default',
          },
        ],
      }),
    }),
  );
  await page.route('http://127.0.0.1:7799/v1/agents/*/models*', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '{"models":[]}' }),
  );
  await page.route('http://127.0.0.1:7799/v1/agent-chats*', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '{"chats":[]}' }),
  );
  await page.route('http://127.0.0.1:7799/v1/agents/*/preferences', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }),
  );
  await page.route('http://127.0.0.1:7799/v1/agents/*/session?*', (route) => {
    const query = new URL(route.request().url()).searchParams;
    if (!promptStarted) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: query.get('task_id'),
          title: '',
          agent_id: AGENT_ID,
          resource_id: query.get('resource_id'),
          thread_id: query.get('thread_id'),
          pause_support: 'cancel-only',
          status: 'draft',
          messages: [],
        }),
      });
    }
    // The controller's run() ignores /prompt's own response body and always
    // re-fetches the session afterward — this final read is what has to
    // carry the completed reply and status, not /prompt's own fulfillment.
    if (promptFinished) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: query.get('task_id'),
          title: 'run the tests',
          agent_id: AGENT_ID,
          resource_id: query.get('resource_id'),
          thread_id: query.get('thread_id'),
          pause_support: 'cancel-only',
          status: 'completed',
          messages: [userMessage, toolCall, reply],
        }),
      });
    }
    pollsSincePromptStarted += 1;
    // Poll 1 while running: nothing back yet. Poll 2+: the tool call has
    // streamed in.
    const messages = pollsSincePromptStarted <= 1 ? [userMessage] : [userMessage, toolCall];
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: query.get('task_id'),
        title: 'run the tests',
        agent_id: AGENT_ID,
        resource_id: query.get('resource_id'),
        thread_id: query.get('thread_id'),
        pause_support: 'cancel-only',
        status: 'running',
        messages,
      }),
    });
  });
  await page.route('http://127.0.0.1:7799/v1/agents/*/prompt', async (route) => {
    promptStarted = true;
    // Long enough for two 900ms poll ticks to land while this is in flight.
    await new Promise((resolve) => setTimeout(resolve, 2600));
    promptFinished = true;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        session_id: 'local_session_e2e_activity',
        outputs: [
          { kind: 'tool_call', text: toolCall.body },
          { kind: 'agent_message_chunk', text: reply.body },
        ],
      }),
    });
  });
}

test('shows live thinking and tool-call status while a turn runs', async ({ page }) => {
  const repositoryURL = createLocalRepositoryFixture({ name: 'activity' });
  await stubSlowHostAgent(page);

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
  const channelName = `activity-${Date.now()}`;
  await page.getByRole('button', { name: 'Add channel' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Channel name').fill(channelName);
  await fillNewChannelResource(dialog, repositoryURL);
  await selectNewChannelAgent(dialog, 'mock');
  await dialog.getByRole('button', { name: 'Create channel' }).click();
  await page.getByRole('button', { name: `Open ${channelName}` }).click();

  const agentPopover = await openAgentPopover(page);
  await agentPopover.getByRole('button', { name: 'Change agent connection' }).click();
  const agentDialog = page.getByRole('dialog');
  await agentDialog.getByRole('button', { name: 'Set active collaborator' }).click();
  await expect(agentDialog).toBeHidden();

  await (await openAgentPopover(page)).getByRole('button', { name: 'New agent chat' }).click();
  const surface = page.locator('.agent-task-surface');
  await surface.getByLabel('Instruction').fill('run the tests');
  await surface.getByRole('button', { name: 'Start chat' }).click();

  await expect(surface.getByText('Agent is thinking…')).toBeVisible();
  await expect(surface.getByText(/Running: npm test/)).toBeVisible({ timeout: 5_000 });
  await expect(surface.getByText('All tests pass.')).toBeVisible({ timeout: 5_000 });
  // The activity line only means something while a turn is running — once
  // the reply lands, it should not linger over an idle composer.
  await expect(surface.getByText('Agent is thinking…')).toHaveCount(0);
  await expect(surface.getByText(/Running: npm test/)).toHaveCount(0);
});
