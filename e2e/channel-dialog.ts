import type { Locator } from '@playwright/test';

/**
 * Fills the "Connect a new Git resource" field inside the channel-creation
 * dialog's Resources section.
 *
 * The field is collapsed by default — channel creation now shows only the
 * channel name up front, with Resources and Agent runtime as disclosures a
 * person opens on demand rather than five fields shown at once.
 */
export async function fillNewChannelResource(dialog: Locator, repositoryURL: string) {
  await dialog.getByRole('button', { name: 'Resources' }).click();
  await dialog.getByLabel('Connect a new Git resource').fill(repositoryURL);
}

export async function selectNewChannelAgent(dialog: Locator, protocol: string) {
  await dialog.getByRole('button', { name: 'Agent runtime' }).click();
  await dialog.getByLabel('Runtime').selectOption(protocol);
}
