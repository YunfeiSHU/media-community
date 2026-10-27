import { defineConfig, devices } from '@playwright/test';

/**
 * E2E 配置。
 *
 * 当前开发沙箱无法启动浏览器（子进程受限），这份配置用于本机执行：
 *   pnpm e2e
 *
 * 默认跑 mock 模式（VITE_USE_MOCK=1），因此不依赖后端；
 * 需要联调真实接口时，先启动后端并去掉 .env.development 里的 mock 开关。
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  fullyParallel: true,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'pnpm dev',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 120_000,
  },
});
