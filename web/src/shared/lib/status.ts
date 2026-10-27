/**
 * 各域状态映射。
 *
 * 后端有三套风格：商品/订单用数字、视频/直播用字符串、用户用字符串。
 * 每个实体维护自己的映射，未知值必须有兜底文案，不能直接渲染原始值。
 */

export const VIDEO_STATUS_TEXT: Record<string, string> = {
  DRAFT: '草稿',
  TRANSCODING: '转码中',
  READY: '待发布',
  PUBLISHED: '已发布',
  OFFLINE: '已下架',
  FAILED: '转码失败',
};

export const LIVE_STATUS_TEXT: Record<string, string> = {
  IDLE: '未开播',
  STARTING: '等待推流',
  LIVING: '直播中',
  ENDED: '已结束',
  FAILED: '开播失败',
};

export const ORDER_STATUS_TEXT: Record<number, string> = {
  0: '待支付',
  1: '已支付',
  2: '已完成',
  3: '已取消',
};

export function textOf(map: Record<string, string>, value?: string | null): string {
  if (!value) return '—';
  return map[value] ?? value;
}

export function textOfNumber(map: Record<number, string>, value?: number | null): string {
  if (value == null) return '—';
  return map[value] ?? String(value);
}
