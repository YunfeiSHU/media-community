import { useEffect, useRef, useState } from 'react';
import type { DanmakuItem } from '@/entities/danmaku/api';

/** 弹幕在屏幕上停留的时间（与 DanmakuLayer 的 CSS 动画时长对齐）。 */
const LIFETIME_MS = 10_000;
/** 提前 0.3s 弹出，避免刷新率/时间片导致“刚好错过”。 */
const WINDOW_AFTER_MS = 300;
/** 允许显示刚过去 2s 的弹幕：拖动进度条落点不精确时也能看到。 */
const WINDOW_BEFORE_MS = 2_000;
const MAX_VISIBLE = 40;

interface Visible {
  item: DanmakuItem;
  until: number;
}

/**
 * 把「带 timeMs 的弹幕列表 + 当前播放进度」转成「此刻应该出现在屏幕上的弹幕」。
 *
 * - 每条弹幕只触发一次（按 clientMsgId / danmakuId 去重）
 * - 向后 seek 超过 2s 会重置，允许重看同一段弹幕
 * - 显示 10s 后自动移除，避免 DOM 无限增长
 */
export function useScheduledDanmaku(
  items: DanmakuItem[],
  currentTimeSec: number,
  enabled: boolean,
): DanmakuItem[] {
  const [visible, setVisible] = useState<Visible[]>([]);
  const shownRef = useRef<Set<string>>(new Set());
  const lastMsRef = useRef(0);

  useEffect(() => {
    if (!enabled) {
      shownRef.current.clear();
      lastMsRef.current = 0;
      setVisible([]);
      return;
    }

    const nowMs = Math.max(0, Math.round(currentTimeSec * 1000));
    // 时间倒退超过窗口 → 视为 seek，清空已展示记录
    if (nowMs + WINDOW_BEFORE_MS < lastMsRef.current) {
      shownRef.current.clear();
      setVisible([]);
    }
    lastMsRef.current = nowMs;

    const due: DanmakuItem[] = [];
    for (const item of items) {
      const key = item.clientMsgId || (item.danmakuId ? `id:${item.danmakuId}` : '');
      if (!key || shownRef.current.has(key)) continue;
      if (item.timeMs <= nowMs + WINDOW_AFTER_MS && item.timeMs >= nowMs - WINDOW_BEFORE_MS) {
        shownRef.current.add(key);
        due.push(item);
      }
    }

    setVisible((prev) => {
      const now = Date.now();
      const alive = prev.filter((entry) => entry.until > now);
      if (due.length === 0 && alive.length === prev.length) return prev;
      const until = now + LIFETIME_MS;
      return [...alive, ...due.map((item) => ({ item, until }))].slice(-MAX_VISIBLE);
    });
  }, [items, currentTimeSec, enabled]);

  return visible.map((entry) => entry.item);
}
