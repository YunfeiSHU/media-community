import { expect, test } from '@playwright/test';

/**
 * 收藏列表回归：
 * 播放页收藏 → 个人空间「我的收藏」出现 → 取消收藏 → 列表消失。
 *
 * 用 9013（已发布、可播放的联调视频）做固定目标，避免依赖首页排序。
 */
test.describe('收藏列表', () => {
  test('收藏 → 列表出现 → 取消后消失', async ({ page }) => {
    await page.goto('/login');
    await page.getByPlaceholder('1xxxxxxxxxx').fill('13800138001');
    await page.getByPlaceholder('8–20 位').fill('password123');
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page).toHaveURL('http://localhost:5173/', { timeout: 15_000 });

    const videoId = '9013';
    await page.goto(`/video/${videoId}`);
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 15_000 });

    const favoriteButton = page.getByRole('button', { name: /收藏/ });
    await expect(favoriteButton).toBeVisible({ timeout: 10_000 });

    // 先把状态归一化为「未收藏」，避免上一次失败用例留下脏数据
    if ((await favoriteButton.getAttribute('aria-pressed')) === 'true') {
      await favoriteButton.click();
      await expect(favoriteButton).toHaveAttribute('aria-pressed', 'false', { timeout: 10_000 });
    }

    // 收藏
    await favoriteButton.click();
    await expect(favoriteButton).toHaveAttribute('aria-pressed', 'true', { timeout: 10_000 });

    // 我的收藏里出现这张卡
    await page.goto('/space?tab=favorite');
    const favoriteCard = page.locator(`a[href="/video/${videoId}"]`);
    await expect(favoriteCard).toBeVisible({ timeout: 15_000 });

    // 从列表点回播放页取消收藏
    await favoriteCard.click();
    await expect(favoriteButton).toHaveAttribute('aria-pressed', 'true', { timeout: 10_000 });
    await favoriteButton.click();
    await expect(favoriteButton).toHaveAttribute('aria-pressed', 'false', { timeout: 10_000 });

    // 再回收藏列表，卡片消失
    await page.goto('/space?tab=favorite');
    await expect(page.locator(`a[href="/video/${videoId}"]`)).toHaveCount(0, { timeout: 15_000 });
  });
});
