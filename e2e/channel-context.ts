import type { Locator, Page } from '@playwright/test';

/**
 * The channel-context panel is closed by default and opened on demand from
 * the header toggle, rather than pinned open on every channel.
 */
export async function openChannelContext(page: Page): Promise<Locator> {
  await page.getByRole('button', { name: 'Show channel context' }).click();
  const context = page.getByRole('complementary', { name: 'Channel context' });
  await context.waitFor();
  return context;
}
