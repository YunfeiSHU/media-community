/**
 * 金额一律用「分」传输（Long），展示时再转元。
 * 不要在 JS 里把分转成元后继续做加减——浮点会丢精度。
 */
export function formatCents(cents?: number | null): string {
  if (cents == null) return '—';
  const sign = cents < 0 ? '-' : '';
  const abs = Math.abs(cents);
  return `${sign}¥${(abs / 100).toFixed(2)}`;
}

/** 列表页的「¥xx 起」：有最低价才显示。 */
export function formatPriceFrom(cents?: number | null): string {
  if (cents == null || cents <= 0) return '暂无报价';
  return `${formatCents(cents)} 起`;
}
