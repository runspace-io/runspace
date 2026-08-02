import type { Locator, Page } from '@playwright/test';

/**
 * Resources and Agent live in header popovers, not a docked sidebar: each
 * opens on click and floats over the chat rather than reserving width.
 */
export async function openResourcePopover(page: Page): Promise<Locator> {
  return openPopover(page, 'Resources');
}

export async function openAgentPopover(page: Page): Promise<Locator> {
  return openPopover(page, 'Agent');
}

/**
 * The trigger toggles rather than always opening, so this only clicks when
 * the popover isn't already open (e.g. reopening after an inner dialog closed).
 */
async function openPopover(page: Page, name: 'Resources' | 'Agent'): Promise<Locator> {
  const trigger = page.getByTitle(name, { exact: true });
  if ((await trigger.getAttribute('aria-expanded')) !== 'true') {
    await trigger.click();
  }
  const panel = page.getByRole('group', { name });
  await panel.waitFor();
  return panel;
}
