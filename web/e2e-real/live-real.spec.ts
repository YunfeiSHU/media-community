import { expect, test } from '@playwright/test';

/**
 * 真实后端联调冒烟：登录 → 首页 → 播放页点赞 → 商城 → 直播间发弹幕。
 *
 * 这条用例把「前端 → Vite 代理 → 网关 → 各微服务」整条链路跑一遍，
 * 任何契约不一致（字段名、信封、鉴权头）都会在这里暴露。
 */
test.describe('真实后端联调', () => {
  test('登录 → 首页 → 播放页点赞 → 商城 → 直播间发弹幕', async ({ page }) => {
    // 1) 登录
    await page.goto('/login');
    await page.getByPlaceholder('1xxxxxxxxxx').fill('13800138001');
    await page.getByPlaceholder('8–20 位').fill('password123');
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page).toHaveURL('http://localhost:5173/', { timeout: 15_000 });

    // 2) 首页有种子视频（后端 seed-media.sql 写入 12 条）
    const firstCard = page.locator('a[href^="/video/"]').first();
    await expect(firstCard).toBeVisible({ timeout: 15_000 });

    // 3) 播放页：点赞按钮可见，点击后进入已点赞状态
    await firstCard.click();
    const likeButton = page.getByRole('button', { name: /点赞/ });
    await expect(likeButton).toBeVisible();
    await likeButton.click();
    await expect(likeButton).toHaveAttribute('aria-pressed', 'true', { timeout: 10_000 });

    // 4) 商城：能看到联调时创建并上架的商品
    await page.goto('/shop');
    await expect(page.getByText('联调测试商品')).toBeVisible({ timeout: 15_000 });

    // 5) 播放真实转码产物：上传+转码+发布后的 9013，HLS master 必须 200
    const masterResponse = page.waitForResponse(
      (response) =>
        response.url().includes('master.m3u8') && response.request().method() === 'GET',
      { timeout: 20_000 },
    );
    await page.goto('/video/9013');
    await expect(page.getByRole('heading', { name: '联调上传视频' })).toBeVisible({ timeout: 15_000 });
    const master = await masterResponse;
    // hls.js 会带 Range 头，MinIO 返回 206 Partial Content 也是成功
    expect([200, 206]).toContain(master.status());

    // 6) 直播间：弹幕 WebSocket 连上并能发中文弹幕
    await page.goto('/streams/9501');
    await expect(page.getByRole('heading', { name: '深夜写代码，顺便聊聊重构' })).toBeVisible({
      timeout: 15_000,
    });
    // 等 WS 真正握手完成再发，避免“连接未就绪”的偶发失败
    await expect(page.getByText('已连接')).toBeVisible({ timeout: 15_000 });
    const input = page.getByPlaceholder('发一条弹幕…');
    await expect(input).toBeEnabled({ timeout: 15_000 });
    await input.fill('浏览器 E2E 中文弹幕');
    await page.getByRole('button', { name: '发送' }).click();
    // 弹幕同时出现在弹幕层与右侧聊天列表，取第一个即可（strict mode 会命中两个）
    await expect(page.getByText('浏览器 E2E 中文弹幕').first()).toBeVisible({ timeout: 10_000 });
  });
});



