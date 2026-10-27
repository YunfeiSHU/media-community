import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from '@/app/App';
// 样式按顺序引入：Tailwind base 先，设计 token 与 reset 后（后者覆盖前者）
import '@/index.css';
import '@/shared/styles/tokens.css';
import '@/shared/styles/reset.css';

const container = document.getElementById('root');
if (!container) {
  throw new Error('#root 节点不存在');
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
