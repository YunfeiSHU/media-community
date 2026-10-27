/**
 * 会话与刷新单飞。
 *
 * 设计（见《后端前置任务》认证方案）：
 *  - access token 只放内存，刷新页面后重新用 refresh token 换；
 *  - refresh token 放 localStorage，多标签通过 BroadcastChannel 同步；
 *  - 刷新必须单飞 + 跨标签加锁，否则 refresh 的一次性轮换会把其他标签踢下线。
 */

const REFRESH_KEY = 'mp.refreshToken';

let accessToken: string | null = null;
let accessExpiresAt = 0;
let inflight: Promise<string | null> | null = null;

const channel =
  typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('mp-auth') : null;

export function getAccessToken(): string | null {
  if (accessToken && Date.now() < accessExpiresAt) return accessToken;
  return null;
}

export function setSession(access: string, refresh: string, expiresInSec: number): void {
  accessToken = access;
  accessExpiresAt = Date.now() + expiresInSec * 1000 - 30_000; // 提前 30s 视为过期
  localStorage.setItem(REFRESH_KEY, refresh);
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY);
}

export function clearSession(): void {
  accessToken = null;
  accessExpiresAt = 0;
  localStorage.removeItem(REFRESH_KEY);
}

export function broadcastLogout(): void {
  clearSession();
  channel?.postMessage({ type: 'logout' });
}

channel?.addEventListener('message', (event: MessageEvent) => {
  if ((event.data as { type?: string })?.type === 'logout') {
    accessToken = null;
    accessExpiresAt = 0;
  }
});

interface RefreshResponse {
  accessToken: string;
  refreshToken: string;
  /** auth-service 实际字段名 */
  accessTokenExpiresIn?: number;
  /** 兼容其他服务可能使用的命名 */
  expiresIn?: number;
}

/**
 * 刷新一次 access token（单飞 + 跨标签锁）。
 * 返回新的 access token；失败返回 null（调用方应清会话并跳登录）。
 */
export function refreshOnce(): Promise<string | null> {
  if (inflight) return inflight;

  inflight = withCrossTabLock(async () => {
    const refreshToken = getRefreshToken();
    if (!refreshToken) return null;

    try {
      // 用原生 fetch 而不是 axios 实例：避免刷新请求再走 401 拦截器造成递归
      const response = await fetch('/api/v1/auth/token/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refreshToken }),
      });
      if (!response.ok) {
        // 只有「刷新令牌确实无效」才清会话；5xx/网络抖动清掉会把用户莫名踢下线。
        // 注意：POST /auth/token/refresh 对无效令牌返回 401。
        if (response.status === 401) clearSession();
        return null;
      }
      const body = (await response.json()) as {
        code: number;
        data: RefreshResponse | null;
      };
      if (body.code !== 0 || !body.data) {
        clearSession();
        return null;
      }

      // 后端字段是 accessTokenExpiresIn；写成 expiresIn 会得到 undefined，
      // 进而让 access token 立刻被当成过期（每个请求都 401 → 刷新风暴 → 被踢下线）。
      const ttl = body.data.accessTokenExpiresIn ?? body.data.expiresIn ?? 1800;
      setSession(body.data.accessToken, body.data.refreshToken, ttl);
      channel?.postMessage({ type: 'refresh', refreshToken: body.data.refreshToken });
      return body.data.accessToken;
    } catch {
      return null;
    }
  }).finally(() => {
    inflight = null;
  });

  return inflight;
}

async function withCrossTabLock<T>(fn: () => Promise<T>): Promise<T> {
  const locks = (navigator as Navigator & { locks?: LockManager }).locks;
  if (locks) {
    return locks.request('mp-auth-refresh', fn) as Promise<T>;
  }
  return fn();
}
