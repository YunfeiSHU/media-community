import { expect, test } from '@playwright/test';

/**
 * 真实后端：用户域 + 创作管理 + 管理后台。
 *
 * 覆盖：资料编辑 → 公开主页/粉丝关注 tab；视频管理页；商品管理页（创建商品）；
 * 管理后台（管理员账号 13800138001）用户查询。
 */
test.describe('用户域与管理后台', () => {
  test('资料编辑 → 公开主页 → 视频/商品管理 → 管理后台', async ({ page }) => {
    test.setTimeout(120_000);

    await page.goto('/login');
    await page.getByPlaceholder('1xxxxxxxxxx').fill('13800138001');
    await page.getByPlaceholder('8–20 位').fill('password123');
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page).toHaveURL('http://localhost:5173/', { timeout: 15_000 });

    // 1) 资料编辑：改简介并保存 → 跳到公开主页
    await page.goto('/settings');
    const bio = `E2E 简介 ${Date.now()}`;
    await page.getByPlaceholder('介绍一下自己…').fill(bio);
    await page.getByRole('button', { name: '保存', exact: true }).click();
    await expect(page).toHaveURL(/\/user\/.+/, { timeout: 15_000 });
    await expect(page.getByText(bio)).toBeVisible({ timeout: 15_000 });

    // 2) 主页 tab：粉丝 / 关注
    await page.getByRole('button', { name: /粉丝/ }).click();
    await expect(
      page.getByText('还没有粉丝').or(page.locator('ul li').first()),
    ).toBeVisible({ timeout: 15_000 });
    await page.getByRole('button', { name: /^关注 / }).click();

    // 3) 视频管理
    await page.goto('/studio/videos');
    await expect(page.getByRole('heading', { name: '视频管理' })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole('button', { name: '编辑' }).first()).toBeVisible({
      timeout: 15_000,
    });

    // 4) 商品管理：创建带默认 SKU 的商品
    await page.goto('/studio/products');
    await expect(page.getByRole('heading', { name: '商品管理' })).toBeVisible({ timeout: 15_000 });
    const productName = `E2E 商品 ${Date.now()}`;
    await page.getByPlaceholder('商品名').fill(productName);
    await page.getByRole('button', { name: '创建商品（含默认 SKU）' }).click();
    await expect(page.getByText(productName)).toBeVisible({ timeout: 15_000 });

    // 5) 管理后台：管理员可见，并能按 userId 查询
    await page.goto('/admin');
    await expect(page.getByRole('heading', { name: '管理后台' })).toBeVisible({ timeout: 15_000 });
    await page.getByRole('button', { name: '用户查询' }).click();
    await page.getByPlaceholder('userId（数字主键，不是 uid）').fill('2');
    await page.getByRole('button', { name: '查询', exact: true }).click();
    await expect(page.getByText(/"uid"/)).toBeVisible({ timeout: 15_000 });
  });
});
