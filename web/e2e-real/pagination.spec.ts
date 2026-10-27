import { expect, test } from '@playwright/test';

/** 首页分页：12 条/页，页码写入 URL，切页换一批视频。 */
test.describe('首页分页', () => {
  test('首页 12 条/页，切到第 2 页换了列表', async ({ page }) => {
    await page.goto('/');

    const pagination = page.getByRole('navigation', { name: '分页' });
    await expect(pagination).toBeVisible({ timeout: 15_000 });

    const cards = page.locator('a[href^="/video/"]');
    await expect(cards.first()).toBeVisible({ timeout: 15_000 });
    const firstHref = await cards.first().getAttribute('href');
    expect(firstHref).toBeTruthy();

    await pagination.getByRole('button', { name: '2', exact: true }).click();
    await expect(page).toHaveURL(/page=2/);
    await expect(cards.first()).not.toHaveAttribute('href', firstHref!);
    await expect(
      pagination.getByRole('button', { name: '2', exact: true }),
    ).toHaveAttribute('aria-current', 'page');
  });
});
