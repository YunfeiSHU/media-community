import { useId } from 'react';

type CoverKind = 'video' | 'live' | 'product';

interface Props {
  /** 用视频 / 直播间 / 商品 id 做稳定哈希，同一内容每次渲染颜色一致 */
  seed: string;
  kind?: CoverKind;
  /** 商品名，用于按关键词推断品类色 */
  hint?: string;
  className?: string;
}

/** 默认封面配色：低饱和渐变 + 同色系主色，避免首页一片灰。 */
const PALETTES = [
  { from: '#ffe9f1', to: '#ffc9db', accent: '#fb7299' }, // 粉：服饰 / 美妆
  { from: '#e9f7ff', to: '#c5e9ff', accent: '#00aeec' }, // 蓝：数码
  { from: '#edf7ee', to: '#cceccd', accent: '#2ba471' }, // 绿：家居
  { from: '#f4efff', to: '#ddd0ff', accent: '#8a5cf6' }, // 紫：图书 / 课程
  { from: '#fff5e8', to: '#ffe2ba', accent: '#ff9f0a' }, // 橙：食品
  { from: '#eaf7f6', to: '#c9ebe7', accent: '#14b8a6' }, // 青：运动 / 户外
];

const PRODUCT_HINTS: Array<{ keys: string[]; palette: number }> = [
  { keys: ['手机', '数码', '耳机', '键盘', '电脑', '相机', '显示器', '充电', '音箱'], palette: 1 },
  { keys: ['衣', '裤', '鞋', '帽', '包', '饰', '美妆', '口红', '香水'], palette: 0 },
  { keys: ['家', '桌', '椅', '灯', '杯', '收纳', '床', '毯', '厨'], palette: 2 },
  { keys: ['书', '课', '教程', '学习', '笔记', '文具'], palette: 3 },
  { keys: ['食', '茶', '咖啡', '零食', '果', '饮', '糖'], palette: 4 },
  { keys: ['运动', '健身', '户外', '跑', '球', '露营'], palette: 5 },
];

function hashOf(value: string) {
  let hash = 0;
  for (let i = 0; i < value.length; i += 1) {
    hash = (hash * 31 + value.charCodeAt(i)) >>> 0;
  }
  return hash;
}

function pickPalette(seed: string, kind: CoverKind, hint?: string) {
  if (kind === 'product' && hint) {
    const matched = PRODUCT_HINTS.find((group) =>
      group.keys.some((key) => hint.includes(key)),
    );
    if (matched) return PALETTES[matched.palette];
  }
  return PALETTES[hashOf(seed || kind) % PALETTES.length];
}

/**
 * 无封面时的默认图：
 * - 视频：粉/蓝/绿/紫/橙/青渐变 + 播放三角 + VIDEO
 * - 直播：同色系 + 广播信号点 + LIVE
 * - 商品：品类色（数码蓝、服饰粉、食品橙…）+ 购物袋 + PRODUCT
 *
 * 不用「标题/商品名首字」：中文首字在缩略图里既不好看，也无法区分内容。
 */
export function DefaultCover({
  seed,
  kind = 'video',
  hint,
  className = 'h-full w-full',
}: Props) {
  const gradientId = `cover-${useId().replace(/:/g, '')}`;
  const palette = pickPalette(seed, kind, hint);

  return (
    <svg
      viewBox="0 0 320 180"
      preserveAspectRatio="xMidYMid slice"
      className={className}
      role="img"
      aria-label={kind === 'product' ? '默认商品图' : '默认封面'}
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor={palette.from} />
          <stop offset="100%" stopColor={palette.to} />
        </linearGradient>
      </defs>

      <rect width="320" height="180" fill={`url(#${gradientId})`} />

      {/* 透明几何装饰，让纯色底不空 */}
      <circle cx="282" cy="18" r="74" fill="#ffffff" fillOpacity="0.24" />
      <circle cx="24" cy="176" r="92" fill={palette.accent} fillOpacity="0.10" />
      <circle cx="246" cy="154" r="34" fill="#ffffff" fillOpacity="0.18" />
      <path
        d="M0 142c52-26 96 12 154-8s100-24 166 8v38H0Z"
        fill="#ffffff"
        fillOpacity="0.26"
      />

      {kind === 'live' && (
        <g>
          <circle cx="160" cy="84" r="27" fill="#ffffff" fillOpacity="0.42" />
          <circle cx="160" cy="84" r="5" fill={palette.accent} />
          <path
            d="M141 65a27 27 0 0 0 0 38M179 65a27 27 0 0 1 0 38"
            stroke={palette.accent}
            strokeWidth="3"
            strokeLinecap="round"
            fill="none"
            opacity="0.85"
          />
        </g>
      )}

      {kind === 'video' && (
        <g>
          <circle cx="160" cy="84" r="28" fill="#ffffff" fillOpacity="0.44" />
          <path d="M152 71l20 13-20 13Z" fill={palette.accent} fillOpacity="0.95" />
        </g>
      )}

      {kind === 'product' && (
        <g>
          <circle cx="160" cy="84" r="28" fill="#ffffff" fillOpacity="0.44" />
          <path
            d="M148 73h24l2.5 23h-29Z"
            fill={palette.accent}
            fillOpacity="0.95"
          />
          <path
            d="M154 73v-4.5a6 6 0 0 1 12 0V73"
            stroke={palette.accent}
            strokeWidth="3"
            strokeLinecap="round"
            fill="none"
          />
        </g>
      )}

      <text
        x={kind === 'live' ? 306 : kind === 'product' ? 160 : 14}
        y="168"
        textAnchor={kind === 'live' ? 'end' : kind === 'product' ? 'middle' : 'start'}
        fill={palette.accent}
        fillOpacity="0.7"
        fontSize="10"
        fontWeight="600"
        letterSpacing="1.2"
      >
        {kind === 'live' ? 'LIVE' : kind === 'product' ? 'PRODUCT' : 'VIDEO'}
      </text>
    </svg>
  );
}
