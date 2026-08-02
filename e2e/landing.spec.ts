import { expect, test } from '@playwright/test';

test.describe('Runspace landing page', () => {
  test('presents the private-to-shared collaboration model', async ({ page }) => {
    await page.goto('/');

    // The headline is set across several lines, so assert on its phrases rather
    // than one exact string that a line break would break.
    const headline = page.getByRole('heading', { level: 1 });
    await expect(headline).toContainText('Your team’s context.');
    await expect(headline).toContainText('Your agents.');
    await expect(page.getByRole('link', { name: 'Request a pilot' }).first()).toBeVisible();
    await expect(
      page.getByLabel('Runspace workflow showing shared context and private agent work'),
    ).toContainText('Only this result crossed into the channel');
    await expect(
      page.getByRole('heading', { name: 'Connected does not mean centralized.' }),
    ).toBeVisible();
  });

  test('uses a contained mobile navigation', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/');

    const menu = page.getByRole('button', { name: 'Open navigation' });
    await menu.click();

    await expect(menu).toHaveAttribute('aria-expanded', 'true');
    await expect(page.getByRole('navigation', { name: 'Mobile navigation' })).toBeVisible();
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
      .toBeLessThanOrEqual(375);
  });
});
