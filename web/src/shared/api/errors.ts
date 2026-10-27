/** 统一响应信封：{code,message,data}，code === 0 表示成功。 */
export interface Envelope<T> {
  code: number;
  message: string;
  data: T | null;
}

/** 分页结构（视频/直播/商品都按这个来，Java 侧由 adapter 归一）。 */
export interface Page<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

/** 业务错误。status 是 HTTP 状态，code 是后端业务码，两者可能语义不同（如 40001 复用）。 */
export class ApiError extends Error {
  constructor(
    public readonly code: number,
    message: string,
    public readonly status: number,
    public readonly requestId?: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

const HTTP_MESSAGES: Record<number, string> = {
  400: '请求参数不合法',
  401: '登录已过期，请重新登录',
  403: '没有操作权限',
  404: '内容不存在',
  409: '当前状态不允许该操作',
  429: '操作太频繁，请稍后再试',
  500: '服务暂时不可用',
  502: '服务暂时不可用',
  503: '服务暂时不可用',
};

export function httpMessage(status: number): string {
  return HTTP_MESSAGES[status] ?? '网络异常，请稍后重试';
}

/** 把任意异常转成可展示的文案；ApiError 优先用后端 message。 */
export function describeError(error: unknown): string {
  if (error instanceof ApiError) return error.message || httpMessage(error.status);
  if (error instanceof Error) return error.message || '网络异常';
  return '网络异常';
}
