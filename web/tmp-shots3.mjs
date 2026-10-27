import { chromium } from '@playwright/test';

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));

async function shot(name, path) {
  await page.goto(`http://localhost:5173${path}`);
  await page.waitForTimeout(2000);
  await page.screenshot({ path: `../.tmp-build/ui-${name}.png` });
  console.log(name, page.url());
}

await shot('login', '/login');
await shot('shop', '/shop');
await shot('product', '/shop/P20BBCLERAIRU42L7H');

await page.goto('http://localhost:5173/login');
await page.getByPlaceholder('1xxxxxxxxxx').fill('13800138001');
await page.getByPlaceholder('8–20 位').fill('password123');
await page.getByRole('button', { name: '登录' }).click();
await page.waitForURL('http://localhost:5173/', { timeout: 15000 });

await shot('space', '/space');
await shot('studio-videos', '/studio/videos');
await shot('studio-products', '/studio/products');
await shot('admin', '/admin');

console.log('ERRORS', errors);
await browser.close();
