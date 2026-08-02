import { expect, test, type Page } from '@playwright/test';
import { createLocalRepositoryFixture } from './local-repository';
import { fillNewChannelResource, selectNewChannelAgent } from './channel-dialog';

test.setTimeout(60_000);

const AGENT_ID = 'local_agent_e2e_question';

type QuestionState = { prompted: boolean; open: boolean; answeredWith: string | undefined };

/**
 * Stubs the Host Agent so the whole question flow is hermetic: the browser talks
 * to 127.0.0.1:7799 directly, and this spec must not depend on a real ACP
 * runtime being installed on the machine running the suite.
 */
async function stubHostAgent(page: Page, state: QuestionState) {
  const messages = [
    { id: 'm1', role: 'user', body: 'clean the build', created_at: new Date().toISOString() },
    { id: 'm2', role: 'agent', body: 'Reading main.go', created_at: new Date().toISOString() },
  ];
  const question = {
    id: 'q_1_7',
    title: 'Run rm -rf build/',
    options: [
      { id: 'once', name: 'Allow once', kind: 'allow_once' },
      { id: 'reject', name: 'Reject', kind: 'reject_once' },
    ],
    asked_at: new Date().toISOString(),
  };
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
    const turn = state.prompted
      ? {
          status: state.open ? 'waiting_approval' : 'completed',
          messages: state.answeredWith
            ? [...messages, { id: 'm3', role: 'agent', body: 'Removed build/', created_at: '' }]
            : messages,
          ...(state.open ? { question } : {}),
        }
      : { status: 'draft', messages: [] };
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: query.get('task_id'),
        title: state.prompted ? 'clean the build' : '',
        agent_id: AGENT_ID,
        resource_id: query.get('resource_id'),
        thread_id: query.get('thread_id'),
        pause_support: 'cancel-only',
        ...turn,
      }),
    });
  });
  await page.route('http://127.0.0.1:7799/v1/agents/*/prompt', (route) => {
    state.prompted = true;
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        session_id: 'local_session_e2e',
        status: 'waiting_approval',
        outputs: [{ kind: 'agent_message_chunk', text: 'Reading main.go' }],
        question,
      }),
    });
  });
  await page.route('http://127.0.0.1:7799/v1/agents/*/session/answer', async (route) => {
    const body = route.request().postDataJSON() as { option_id: string };
    state.open = false;
    state.answeredWith = body.option_id;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: '{"status":"answered"}',
    });
  });
}

async function openChannelWithAgent(page: Page) {
  const repositoryURL = createLocalRepositoryFixture({ name: 'question' });
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
  // The channel dialog is gated on an active workspace, so wait for the list
  // rather than racing it.
  await workspacesLoaded;
  await expect(page.getByRole('button', { name: 'Switch workspace' })).toBeVisible();
  const channelName = `questions-${Date.now()}`;
  await page.getByRole('button', { name: 'Add channel' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('heading', { name: 'Create channel' })).toBeVisible();
  await dialog.getByLabel('Channel name').fill(channelName);
  await fillNewChannelResource(dialog, repositoryURL);
  await selectNewChannelAgent(dialog, 'mock');
  await dialog.getByRole('button', { name: 'Create channel' }).click();
  await page.getByRole('button', { name: `Open ${channelName}` }).click();
  const context = page.getByRole('complementary', { name: 'Channel context' });
  await expect(context).toBeVisible();
  await context.getByRole('button', { name: 'Change agent connection' }).click();
  const agentDialog = page.getByRole('dialog');
  await agentDialog.getByRole('button', { name: 'Set active collaborator' }).click();
  await expect(agentDialog).toBeHidden();
  return context;
}

test('a parked agent question is shown and can be answered', async ({ page }) => {
  const state: QuestionState = { prompted: false, open: true, answeredWith: undefined };
  await stubHostAgent(page, state);
  const context = await openChannelWithAgent(page);

  await context.getByRole('button', { name: 'New agent chat' }).click();
  const surface = page.locator('.agent-task-surface');
  await surface.getByLabel('Instruction').fill('clean the build');
  await surface.getByRole('button', { name: 'Start chat' }).click();

  // The agent is blocked, so the card must explain why and offer the choices.
  const card = surface.locator('.agent-task-question');
  await expect(card).toBeVisible();
  await expect(card).toContainText('Run rm -rf build/');
  await expect(card.getByRole('button', { name: 'Allow once' })).toBeVisible();
  await expect(card.getByRole('button', { name: 'Reject' })).toBeVisible();
  await expect(surface.getByText('waiting approval')).toBeVisible();

  const answered = page.waitForRequest(
    (request) => request.url().includes('/session/answer') && request.method() === 'POST',
  );
  await card.getByRole('button', { name: 'Allow once' }).click();
  const request = await answered;
  expect((request.postDataJSON() as { option_id: string }).option_id).toBe('once');

  // Once answered the card must go away, or it would invite a second answer.
  await expect(surface.locator('.agent-task-question')).toBeHidden();
  await expect(surface.getByText('Removed build/')).toBeVisible();
});
