import { useId, useState } from 'react';

interface Props {
  src?: string | null;
  alt?: string;
  /** 尺寸类，例如 h-8 w-8 / h-14 w-14 */
  className?: string;
}

/**
 * 统一头像：
 * - 有图且加载成功 → 圆形图片
 * - 没有图 / 加载失败 → 粉蓝渐变底 + 白色小人剪影
 *
 * 不再用「昵称首字」当默认头像：中文首字在头像里辨识度低，
 * 还容易出现“未”“我”这种没有意义的字。
 */
export function Avatar({ src, alt = '', className = 'h-8 w-8' }: Props) {
  const gradientId = `avatar-${useId().replace(/:/g, '')}`;
  const [failed, setFailed] = useState(false);

  if (src && !failed) {
    return (
      <img
        src={src}
        alt={alt}
        loading="lazy"
        decoding="async"
        className={`${className} rounded-full object-cover ring-1 ring-black/5`}
        onError={() => setFailed(true)}
      />
    );
  }

  return (
    <svg
      viewBox="0 0 64 64"
      className={`${className} rounded-full ring-1 ring-black/5`}
      role="img"
      aria-label={alt || '默认头像'}
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#ffd7e4" />
          <stop offset="100%" stopColor="#ddeeff" />
        </linearGradient>
      </defs>
      <rect width="64" height="64" fill={`url(#${gradientId})`} />
      <circle cx="32" cy="24" r="11" fill="#ffffff" fillOpacity="0.92" />
      <path
        d="M9 64c1.4-12.4 10.6-19.5 23-19.5S53.6 51.6 55 64Z"
        fill="#ffffff"
        fillOpacity="0.92"
      />
    </svg>
  );
}
