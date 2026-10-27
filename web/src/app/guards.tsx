import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/features/auth/store';

/**
 * 需要登录的页面守卫。
 *
 * <p>只做 UI 跳转；真正的鉴权仍在后端——前端隐藏入口不是安全边界。
 */
export function RequireAuth({ children }: { children: ReactNode }) {
  const status = useAuthStore((state) => state.status);
  const location = useLocation();

  if (status === 'idle' || status === 'loading') {
    return <div className="h-64 animate-[skeleton-pulse_1.2s_ease-in-out_infinite] rounded-md bg-card" />;
  }

  if (status !== 'authed') {
    const returnTo = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?returnTo=${returnTo}`} replace />;
  }

  return <>{children}</>;
}
