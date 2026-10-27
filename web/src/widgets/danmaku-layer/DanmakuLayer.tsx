import type { DanmakuItem } from '@/entities/danmaku/api';

interface Props {
  items: DanmakuItem[];
  /** 同屏最多显示多少条，超出丢弃旧的（低优先级） */
  maxVisible?: number;
}

/**
 * 弹幕层（轻量 DOM 版）。
 *
 * 用 CSS 动画而不是逐帧 rAF 计算：同屏条数控制在 15 条以内，
 * 浏览器合成器就能跑满帧；等真的需要几百条同屏时再换 Canvas。
 */
export function DanmakuLayer({ items, maxVisible = 15 }: Props) {
  const visible = items.slice(-maxVisible);

  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden>
      {visible.map((item, index) => {
        const key = item.clientMsgId || String(item.danmakuId) || `${index}`;
        if (item.position === 'TOP' || item.position === 'BOTTOM') {
          return (
            <span
              key={key}
              className={`danmaku-fixed ${item.position === 'TOP' ? 'top-4' : 'bottom-16'}`}
              style={{ color: item.color }}
            >
              {item.content}
            </span>
          );
        }
        return (
          <span
            key={key}
            className="danmaku-scroll"
            style={{
              top: `${10 + (index % 8) * 9}%`,
              color: item.color,
              // 每条时长略有差异，避免整屏同步移动看起来像闪烁
              animationDuration: `${9 + (index % 4)}s`,
              animationDelay: `${(index % 5) * 0.15}s`,
            }}
          >
            {item.content}
          </span>
        );
      })}
    </div>
  );
}
