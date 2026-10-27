import { expect, test } from '@playwright/test';

/**
 * 真实后端：浏览器内完成一次投稿（选文件 → 分片上传 → 转码 → 发布）。
 *
 * 这条用例覆盖前端 UploadPage 的状态机；转码是真实 ffmpeg，可能耗时十几秒，
 * 因此单独放宽超时。
 */
test.describe('真实后台上传', () => {
  test('浏览器上传视频并发布', async ({ page }) => {
    test.setTimeout(180_000);

    await page.goto('/login');
    await page.getByPlaceholder('1xxxxxxxxxx').fill('13800138001');
    await page.getByPlaceholder('8–20 位').fill('password123');
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page).toHaveURL('http://localhost:5173/', { timeout: 15_000 });

    await page.goto('/upload');
    await page.getByPlaceholder('给视频起个标题').fill('浏览器上传视频');
    await page.getByPlaceholder('介绍一下这个视频').fill('E2E 上传测试');

    // 视频文件输入（封面输入也是 file，用 accept 区分）；路径相对 web/ 工作目录
    await page
      .locator('input[type="file"][accept*=".mp4"]')
      .setInputFiles('../.tmp-build/e2e-video.mp4');

    await page.getByRole('button', { name: '开始上传' }).click();

    // 上传 → 转码 → 可发布；最长等 2 分钟
    const publishButton = page.getByRole('button', { name: '发布', exact: true });
    await expect(publishButton).toBeVisible({ timeout: 120_000 });
    await publishButton.click();
    await expect(page.getByText('发布成功')).toBeVisible({ timeout: 20_000 });
  });
});

