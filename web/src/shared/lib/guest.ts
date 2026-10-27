/**
 * 游客标识：直播心跳需要 X-Session-Id 才能把人算进在线人数。
 * 存 localStorage，多标签共享同一个游客身份（否则一个人会被算成多个人）。
 */
const KEY = 'mp.guestSessionId';

export function getGuestSessionId(): string {
  let value = localStorage.getItem(KEY);
  if (!value) {
    value = crypto.randomUUID();
    localStorage.setItem(KEY, value);
  }
  return value;
}
