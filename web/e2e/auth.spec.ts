import { expect, test } from '@playwright/test';

test.describe('登录', () => {
  test('未登录点登录 → 登录页 → 登录成功回到首页', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('link', { name: '登录' }).click();
    await expect(page.getByRole('heading', { name: '登录' })).toBeVisible();

    // mock 模式下任意账号密码都能登录
    await page.getByPlaceholder('1xxxxxxxxxx').fill('13800000000');
    await page.getByPlaceholder('8–20 位').fill('password123');
    await page.getByRole('button', { name: '登录' }).click();

    await expect(page.getByRole('link', { name: '演示用户' })).toBeVisible();
  });

  test('未登录访问受保护页面会带 returnTo 跳登录', async ({ page }) => {
    await page.goto('/space');
    await expect(page).toHaveURL(/\/login\?returnTo=/);
  });
});
