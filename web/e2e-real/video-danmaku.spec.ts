import { expect, test } from '@playwright/test';

/**
 * 真实后端：点播弹幕。
 *
 * 覆盖：播放页弹幕工具条 → WS 发送 → 弹幕层出现 → 刷新后从历史接口回放。
 */
test.describe('点播弹幕', () => {
  test('播放页发送弹幕并能在刷新后回放', async ({ page }) => {
    await page.goto('/login');
    await page.getByPlaceholder('1xxxxxxxxxx').fill('13800138001');
    await page.getByPlaceholder('8–20 位').fill('password123');
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page).toHaveURL('http://localhost:5173/', { timeout: 15_000 });

    await page.goto('/video/9013');
    await expect(page.getByRole('heading', { name: '联调上传视频' })).toBeVisible({
      timeout: 15_000,
    });

    const input = page.getByPlaceholder('发一条弹幕…');
    await expect(input).toBeEnabled({ timeout: 15_000 });
    // 等 WS 握手成功再发（按钮旁边会显示状态）
    await expect(page.getByText('弹幕已连接')).toBeVisible({ timeout: 15_000 });

    const content = `E2E 点播弹幕 ${Date.now()}`;
    await input.fill(content);
    await page.getByRole('button', { name: '发送' }).click();

    // 弹幕层是 aria-hidden 的 pointer-events-none 容器，直接按文本找即可
    await expect(page.getByText(content).first()).toBeVisible({ timeout: 10_000 });

    // 刷新后：历史接口（timeMs=0）应把这条弹幕重新放出来
    await page.reload();
    await expect(page.getByText(content).first()).toBeVisible({ timeout: 15_000 });
  });
});

