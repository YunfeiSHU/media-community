import { expect, test } from '@playwright/test';

test.describe('播放页', () => {
  test('首页 → 点击卡片 → 播放页展示标题与操作区', async ({ page }) => {
    await page.goto('/');

    const firstCard = page.locator('a[href^="/video/"]').first();
    await expect(firstCard).toBeVisible();
    await firstCard.click();

    await expect(page).toHaveURL(/\/video\/\d+/);
    await expect(page.getByRole('button', { name: /点赞/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /收藏/ })).toBeVisible();
  });

  test('搜索页展示“搜索视频”空态', async ({ page }) => {
    await page.goto('/search');
    // 用 heading 定位：页面里还有同名的搜索框 label（sr-only），getByText 会命中两个
    await expect(page.getByRole('heading', { name: '搜索视频' })).toBeVisible();
  });
});
