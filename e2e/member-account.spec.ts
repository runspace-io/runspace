import { asUser } from './gateway-token';
import { expect, test, type APIRequestContext } from '@playwright/test';

const MEMBER = process.env.E2E_MEMBER_USERNAME ?? 'nahid';
const MEMBER_PASSWORD = process.env.E2E_MEMBER_PASSWORD ?? 'nahid123';

/**
 * Creates a workspace owned by admin and adds the member to it.
 *
 * This used to assert that `ws_2` existed, which only held because an earlier
 * manual run happened to leave that row behind — the suite was reading database
 * residue. Seeding here keeps the test true against an empty database.
 */
async function shareWorkspaceWithMember(request: APIRequestContext) {
  const created = await request.post('/gateway/workspaces', {
    headers: asUser('admin'),
    data: { name: `Shared with ${MEMBER} ${Date.now()}` },
  });
  expect(created.ok(), await created.text()).toBe(true);
  const workspace = (await created.json()) as { id: string };
  const member = await request.post(`/gateway/workspaces/${workspace.id}/members`, {
    headers: asUser('admin'),
    data: { user_id: MEMBER, role: 'member' },
  });
  expect(member.ok(), await member.text()).toBe(true);
  return workspace.id;
}

test('configured member account can sign in and access its shared workspace', async ({
  page,
  request,
}) => {
  const workspaceID = await shareWorkspaceWithMember(request);

  await page.goto('/');
  await expect(page).toHaveURL(/\/signin/);

  const workspacesLoaded = page.waitForResponse(
    (response) =>
      response.url().endsWith('/gateway/workspaces') &&
      response.request().method() === 'GET' &&
      response.ok(),
  );
  await page.getByLabel('Username').fill(MEMBER);
  await page.getByLabel('Password').fill(MEMBER_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();

  await expect(page).toHaveURL(/\/$/, { timeout: 15_000 });
  const response = await workspacesLoaded;
  const payload = (await response.json()) as { workspaces: Array<{ id: string }> };
  expect(payload.workspaces.some((workspace) => workspace.id === workspaceID)).toBe(true);
  await expect(page.getByRole('button', { name: 'Switch workspace' })).toBeVisible();
});

// A member must not see workspaces they were never added to.
test('member cannot see a workspace they are not a member of', async ({ request }) => {
  const created = await request.post('/gateway/workspaces', {
    headers: asUser('admin'),
    data: { name: `Admin only ${Date.now()}` },
  });
  expect(created.ok()).toBe(true);
  const workspace = (await created.json()) as { id: string };

  const visible = await request.get('/gateway/workspaces', {
    headers: asUser(MEMBER),
  });
  const payload = (await visible.json()) as { workspaces: Array<{ id: string }> };
  expect(payload.workspaces.some((item) => item.id === workspace.id)).toBe(false);
});
