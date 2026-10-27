import axios, { AxiosError, type AxiosRequestConfig } from 'axios';
import { ApiError, httpMessage, type Envelope } from './errors';
import { getAccessToken, refreshOnce } from './session';
import { installMockAdapter } from './mock';

const baseURL = import.meta.env.VITE_API_BASE ?? '/api/v1';

export const client = axios.create({
  baseURL,
  timeout: 15_000,
  // 不设置默认 Content-Type：axios 会对普通对象自动用 application/json，
  // 而 FormData 需要让浏览器自己带 boundary —— 一旦写死 JSON 头，封面上传就会失败。
});

// 没有后端时（VITE_USE_MOCK=1）用内存数据跑通页面，联调时去掉即可
if (import.meta.env.VITE_USE_MOCK === '1') {
  installMockAdapter(client);
}

interface RetriableConfig extends AxiosRequestConfig {
  _retried?: boolean;
}

client.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) config.headers.set('Authorization', `Bearer ${token}`);
  // 后端日志按 X-Request-Id 串联，前端生成的 ID 也要能回传展示
  if (!config.headers.get('X-Request-Id')) {
    config.headers.set('X-Request-Id', crypto.randomUUID());
  }
  return config;
});

client.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const config = error.config as RetriableConfig | undefined;
    const status = error.response?.status ?? 0;
    const body = error.response?.data as Partial<Envelope<unknown>> | undefined;

    // 401 只重放一次；刷新失败则清会话，由路由守卫跳登录
    if (status === 401 && config && !config._retried && !isAuthPath(config.url)) {
      config._retried = true;
      const token = await refreshOnce();
      if (token) {
        config.headers = { ...(config.headers ?? {}), Authorization: `Bearer ${token}` };
        return client.request(config);
      }
    }

    const code = typeof body?.code === 'number' ? body.code : status;
    throw new ApiError(
      code,
      body?.message ?? httpMessage(status),
      status,
      error.response?.headers?.['x-request-id'] as string | undefined,
    );
  },
);

function isAuthPath(url?: string): boolean {
  if (!url) return false;
  return url.includes('/auth/login') || url.includes('/auth/token/refresh');
}

/** 解包 {code,message,data}；code !== 0 抛 ApiError。 */
export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const response = await client.request<Envelope<T>>(config);
  const body = response.data;
  if (body && typeof body.code === 'number' && body.code !== 0) {
    throw new ApiError(body.code, body.message ?? '请求失败', response.status);
  }
  return body?.data as T;
}

export function get<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  return request<T>({ ...config, method: 'GET', url });
}
