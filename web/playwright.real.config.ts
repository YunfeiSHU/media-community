import { defineConfig, devices } from '@playwright/test';

/**
 * 真实后端联调用的 E2E 配置。
 *
 * 与 playwright.config.ts 的区别：webServer 里强制 VITE_USE_MOCK=0，
 * 所有请求走 Vite 代理 → 网关(8000) → 各服务，验证的是真实契约。
 *
 * 前置条件：后端 8 个服务已启动，且数据库里已有测试账号 13800138001 / password123。
 */
export default defineConfig({
  testDir: './e2e-real',
  timeout: 60_000,
  fullyParallel: false,
  // 真实后端 + 冷启动的 Vite 首次编译较慢，串行跑避免多个用例同时触发编译/登录
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'pnpm dev',
    url: 'http://localhost:5173',
    reuseExistingServer: false,
    timeout: 120_000,
    env: { VITE_USE_MOCK: '0' },
  },
});
