import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

// 开发环境统一走代理：浏览器只看到 http://localhost:5173，
// 因此页面与接口同源，不触发 CORS 预检（见《后端前置任务》A2）。
export default defineConfig({
  plugins: [react()],
  resolve: {
    // 用 import.meta.url 而不是 __dirname：本文件是 ESM，__dirname 不存在
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    watch: {
      // Windows 上编辑器/脚本工具会在项目内写临时目录，Vite 的 chokidar 对
      // `.tmp-*.tmpdir` 做 watch 会抛 EBUSY 直接把 dev server 打挂（实测）。
      ignored: ['**/*.tmpdir/**', '**/.tmp-*', '**/tmp-*.mjs'],
    },
    proxy: {
      '/api': { target: 'http://localhost:8000', changeOrigin: true },
      '/ws': { target: 'ws://localhost:8000', ws: true },
      '/media': { target: 'http://localhost:9000', changeOrigin: true },
      '/live': { target: 'http://localhost:8088', changeOrigin: true },
    },
  },
  build: {
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: {
          // 播放器体积大，只在播放页动态 import，这里单独拆包避免拖累首屏
          hls: ['hls.js'],
        },
      },
    },
  },
});
