import { request } from '@/shared/api/http';

export type LoginType = 'PHONE' | 'EMAIL';

export interface LoginResult {
  uid: string;
  accessToken: string;
  refreshToken: string;
  accessTokenExpiresIn: number;
  refreshTokenExpiresIn: number;
}

/** 登录：target 是手机号或邮箱，type 决定校验方式。 */
export function login(type: LoginType, target: string, password: string): Promise<LoginResult> {
  return request<LoginResult>({
    method: 'POST',
    url: '/auth/login',
    data: { type, target, password },
  });
}

/** 登出：带上 refreshToken，后端会连同刷新会话一起删除。 */
export function logout(refreshToken: string | null): Promise<void> {
  return request<void>({
    method: 'POST',
    url: '/auth/logout',
    data: refreshToken ? { refreshToken } : {},
  });
}

export interface VerificationInfo {
  purpose: string;
  type: string;
  target: string;
  /** 学习模式会返回验证码；生产模式为 undefined */
  code?: string;
}

export function sendVerification(type: LoginType, target: string): Promise<VerificationInfo> {
  return request<VerificationInfo>({
    method: 'POST',
    url: '/auth/verification/send',
    data: { type, purpose: 'REGISTER', target },
  });
}

/** 注册：注册接口返回 void，成功后由页面用同一账号密码自动登录。 */
export function register(
  type: LoginType,
  target: string,
  code: string,
  password: string,
): Promise<void> {
  const path = type === 'PHONE' ? '/auth/register/phone' : '/auth/register/email';
  const body = type === 'PHONE' ? { phone: target, code, password } : { email: target, code, password };
  return request<void>({ method: 'POST', url: path, data: body });
}
