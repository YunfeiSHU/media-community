import { expect, test } from '@playwright/test';

/**
 * 真实后端：直播开播闭环。
 *
 * 覆盖：创建直播间 → 进入房间 → owner 控制台可见 → 开始直播（拿推流地址）→ 结束直播。
 * 不依赖 OBS：状态会停在 STARTING（等 SRS on_publish），本用例只验证控制台与接口。
 */
test.describe('直播开播', () => {
  test('创建直播间并完成开始/结束直播', async ({ page }) => {
    await page.goto('/login');
    await page.getByPlaceholder('1xxxxxxxxxx').fill('13800138001');
    await page.getByPlaceholder('8–20 位').fill('password123');
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page).toHaveURL('http://localhost:5173/', { timeout: 15_000 });

    await page.goto('/streams/manage');
    await expect(page.getByRole('heading', { name: '开播管理' })).toBeVisible();

    const title = `E2E 直播间 ${Date.now()}`;
    await page.getByPlaceholder('例如：今晚聊聊重构').fill(title);
    await page.getByRole('button', { name: '创建并进入直播间' }).click();

    await expect(page).toHaveURL(/\/streams\/\d+$/, { timeout: 15_000 });
    await expect(page.getByRole('heading', { name: title })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole('heading', { name: '主播控制台' })).toBeVisible({ timeout: 15_000 });

    await page.getByRole('button', { name: '开始直播' }).click();
    await expect(page.getByRole('button', { name: '结束直播' })).toBeVisible({ timeout: 15_000 });
    // 推流信息必须拆成「服务器 / 串流密钥」两栏，各自可复制
    await expect(page.getByText('串流密钥（OBS「串流密钥」栏，含 ?session=）')).toBeVisible({
      timeout: 15_000,
    });
    await expect(page.getByText('rtmp://127.0.0.1:1936/live', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: '复制', exact: true }).first()).toBeVisible();

    await page.getByRole('button', { name: '结束直播' }).click();
    await expect(page.getByRole('button', { name: '开始直播' })).toBeVisible({ timeout: 15_000 });
  });
});

