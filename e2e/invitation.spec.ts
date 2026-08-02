import { asUser } from './gateway-token';
import { expect, test, type APIRequestContext } from '@playwright/test';

test.setTimeout(90_000);

const INVITEE = process.env.E2E_MEMBER_USERNAME ?? 'nahid';
const INVITEE_PASSWORD = process.env.E2E_MEMBER_PASSWORD ?? 'nahid123';

async function createWorkspace(request: APIRequestContext, name: string) {
  const created = await request.post('/gateway/workspaces', {
    headers: asUser('admin'),
    data: { name: `${name} ${Date.now()}` },
  });
  expect(created.ok(), await created.text()).toBe(true);
  return (await created.json()) as { id: string; name: string };
}

async function createInvite(request: APIRequestContext, workspaceID: string) {
  const invited = await request.post(`/gateway/workspaces/${workspaceID}/invitations`, {
    headers: asUser('admin'),
    data: { role: 'member' },
  });
  expect(invited.ok(), await invited.text()).toBe(true);
  return ((await invited.json()) as { token: string }).token;
}

// The point of the link: someone nobody named by ID can join a workspace.
test('an invited person joins by opening the link', async ({ page, request }) => {
  const workspace = await createWorkspace(request, 'Invite flow');
  const token = await createInvite(request, workspace.id);

  await page.goto(`/invite/${encodeURIComponent(token)}`);
  // Signed out, so the middleware bounces to sign-in and back.
  await expect(page).toHaveURL(/\/signin/);
  await page.getByLabel('Username').fill(INVITEE);
  await page.getByLabel('Password').fill(INVITEE_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();

  // The dev server compiles /invite on first request, which can outlast the
  // default wait right after the web container restarts.
  await expect(page.getByRole('heading', { name: `Join ${workspace.name}` })).toBeVisible({
    timeout: 45_000,
  });
  await expect(page.getByText(/admin invited you as a/)).toBeVisible();
  await page.getByRole('button', { name: 'Join workspace' }).click();

  await expect(page).toHaveURL(/\/$/, { timeout: 15_000 });
  const visible = await request.get('/gateway/workspaces', {
    headers: asUser(INVITEE),
  });
  const payload = (await visible.json()) as { workspaces: Array<{ id: string }> };
  expect(payload.workspaces.some((item) => item.id === workspace.id)).toBe(true);
});

// A single-use link must not let a second person in behind the first.
test('a spent link is refused', async ({ page, request }) => {
  const workspace = await createWorkspace(request, 'Spent link');
  const token = await createInvite(request, workspace.id);

  const accepted = await request.post('/gateway/invitations/accept', {
    headers: asUser('alice'),
    data: { token },
  });
  expect(accepted.ok()).toBe(true);

  await page.goto(`/invite/${encodeURIComponent(token)}`);
  await expect(page).toHaveURL(/\/signin/);
  await page.getByLabel('Username').fill(INVITEE);
  await page.getByLabel('Password').fill(INVITEE_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();

  await expect(page.getByRole('heading', { name: 'This invitation cannot be used' })).toBeVisible({
    timeout: 45_000,
  });
});
