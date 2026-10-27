/**
 * 后端时间格式适配。
 *
 * 现状（见《后端前置任务》A4 / I-004）三种并存：
 *  - user/auth：`2026-09-27T10:30:00`（无时区）
 *  - product/order：`2026-09-27 10:30:00`（空格、无时区）
 *  - Go 三服务：`2026-09-27T10:30:00+08:00`（RFC3339）
 *
 * 无时区的一律按 Asia/Shanghai 解释；后端统一后这里的兼容分支可以删掉。
 */

const SPACE_FORMAT = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/;
const LOCAL_ISO = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?$/;

export function parseServerDate(input?: string | null): Date | null {
  if (!input) return null;

  if (SPACE_FORMAT.test(input)) {
    return toDate(input.replace(' ', 'T') + '+08:00');
  }
  if (LOCAL_ISO.test(input)) {
    return toDate(input + '+08:00');
  }
  return toDate(input);
}

function toDate(value: string): Date | null {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function formatDate(date: Date | null, withTime = false): string {
  if (!date) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  const base = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  return withTime ? `${base} ${pad(date.getHours())}:${pad(date.getMinutes())}` : base;
}

/** 相对时间：刚刚 / 12 分钟前 / 3 天前 / 具体日期。 */
export function relativeTime(date: Date | null, now = new Date()): string {
  if (!date) return '';
  const diffSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (diffSeconds < 60) return '刚刚';
  if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)} 分钟前`;
  if (diffSeconds < 86_400) return `${Math.floor(diffSeconds / 3600)} 小时前`;
  if (diffSeconds < 86_400 * 30) return `${Math.floor(diffSeconds / 86_400)} 天前`;
  return formatDate(date);
}

/** 秒数转 mm:ss / hh:mm:ss（封面右下角时长角标）。 */
export function formatDuration(seconds?: number | null): string {
  if (!seconds || seconds <= 0) return '';
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, '0');
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

/** 播放量：1.2万 / 3.4亿。 */
export function formatCount(value?: number | null): string {
  if (!value || value <= 0) return '0';
  if (value < 10_000) return String(value);
  if (value < 100_000_000) return `${(value / 10_000).toFixed(1)}万`;
  return `${(value / 100_000_000).toFixed(1)}亿`;
}
