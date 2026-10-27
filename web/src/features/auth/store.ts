import { create } from 'zustand';
import { fetchMe, type MeProfile } from '@/entities/user/api';
import {
  broadcastLogout,
  clearSession,
  getRefreshToken,
  refreshOnce,
  setSession,
} from '@/shared/api/session';
import * as authApi from './api';

export type AuthStatus = 'idle' | 'loading' | 'authed' | 'guest';

interface AuthState {
  status: AuthStatus;
  user: MeProfile | null;
  /** 启动时用 refresh token 换 access，再拉 /users/me；失败则按游客处理 */
  bootstrap: () => Promise<void>;
  login: (type: authApi.LoginType, target: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  status: 'idle',
  user: null,

  bootstrap: async () => {
    if (get().status !== 'idle') return;
    set({ status: 'loading' });

    if (!getRefreshToken()) {
      set({ status: 'guest', user: null });
      return;
    }

    // refreshOnce 内部会写回新的 refresh/access；失败会清会话
    const token = await refreshOnce();
    if (!token) {
      set({ status: 'guest', user: null });
      return;
    }

    try {
      const user = await fetchMe();
      set({ status: 'authed', user });
    } catch {
      // 登录态有效但资料服务不可用：不把用户踢下线，只是暂时没有昵称
      set({ status: 'authed', user: null });
    }
  },

  login: async (type, target, password) => {
    const result = await authApi.login(type, target, password);
    setSession(result.accessToken, result.refreshToken, result.accessTokenExpiresIn);

    try {
      const user = await fetchMe();
      set({ status: 'authed', user });
    } catch {
      set({ status: 'authed', user: null });
    }
  },

  logout: async () => {
    const refreshToken = getRefreshToken();
    try {
      // 后端不可用时也要清本地，否则用户会卡在“已登录”状态
      await authApi.logout(refreshToken);
    } catch {
      // ignore：登出是幂等操作，本地清理由 broadcastLogout 兜底
    }
    broadcastLogout();
    clearSession();
    set({ status: 'guest', user: null });
  },

  refreshUser: async () => {
    if (get().status !== 'authed') return;
    try {
      set({ user: await fetchMe() });
    } catch {
      // 保持现状
    }
  },
}));
