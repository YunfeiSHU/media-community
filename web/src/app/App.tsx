import { useEffect } from 'react';
import { BrowserRouter } from 'react-router-dom';
import { useAuthStore } from '@/features/auth/store';
import { useThemeStore } from '@/shared/store/theme';
import { Providers } from './providers';
import { AppRoutes } from './router';

/** 启动骨架：避免「先显示未登录、再跳登录」的闪动。 */
function BootScreen() {
  return (
    <div className="grid min-h-screen place-items-center bg-page">
      <div className="flex items-center gap-3 text-sm text-ink-tertiary">
        <span className="h-4 w-4 animate-spin rounded-full border-2 border-line border-t-brand" />
        正在恢复登录状态…
      </div>
    </div>
  );
}

function Shell() {
  const status = useAuthStore((state) => state.status);
  const bootstrap = useAuthStore((state) => state.bootstrap);
  const theme = useThemeStore((state) => state.theme);

  // 主题只影响根节点 data-theme，CSS 变量负责其余部分
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  if (status === 'idle' || status === 'loading') {
    return <BootScreen />;
  }

  return <AppRoutes />;
}

export function App() {
  return (
    <Providers>
      <BrowserRouter>
        <Shell />
      </BrowserRouter>
    </Providers>
  );
}
