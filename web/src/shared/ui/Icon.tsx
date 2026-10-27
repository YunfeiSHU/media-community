import type { SVGProps } from 'react';

export type IconName =
  | 'home'
  | 'live'
  | 'shop'
  | 'order'
  | 'upload'
  | 'settings'
  | 'search'
  | 'sun'
  | 'moon'
  | 'user'
  | 'logout'
  | 'play'
  | 'pause'
  | 'volume'
  | 'mute'
  | 'fullscreen'
  | 'like'
  | 'star'
  | 'share'
  | 'chevron-right'
  | 'plus'
  | 'trash'
  | 'edit'
  | 'check'
  | 'alert'
  | 'clock'
  | 'eye'
  | 'message';

const PATHS: Record<IconName, string> = {
  home: 'M3 10.5 12 3l9 7.5M5 9.5V21h5v-6h4v6h5V9.5',
  live: 'M4 6h16v12H4zM8 21h8M12 18v3M9 11l3-2v6l-3-2z',
  shop: 'M4 7h16l-1 13H5L4 7zM8 7a4 4 0 0 1 8 0',
  order: 'M6 3h12v18H6zM9 8h6M9 12h6M9 16h4',
  upload: 'M12 16V4m0 0L7 9m5-5 5 5M4 16v4h16v-4',
  settings: 'M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z M19 12a7 7 0 0 0-.1-1l2-1.5-2-3.4-2.4 1a7 7 0 0 0-1.7-1L14.5 3h-5l-.3 2.1a7 7 0 0 0-1.7 1l-2.4-1-2 3.4L5.1 11a7 7 0 0 0 0 2l-2 1.5 2 3.4 2.4-1a7 7 0 0 0 1.7 1l.3 2.1h5l.3-2.1a7 7 0 0 0 1.7-1l2.4 1 2-3.4-2-1.5c.1-.3.1-.7.1-1Z',
  search: 'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14ZM20 20l-4-4',
  sun: 'M12 4V2m0 20v-2m8-8h2M2 12h2m13.7-5.7 1.4-1.4M4.9 19.1l1.4-1.4m11.4 0 1.4 1.4M4.9 4.9l1.4 1.4M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z',
  moon: 'M20 14.5A8.5 8.5 0 0 1 9.5 4 8 8 0 1 0 20 14.5Z',
  user: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM4 21a8 8 0 0 1 16 0',
  logout: 'M15 4h4v16h-4M10 8l-4 4 4 4M6 12h10',
  play: 'M8 5v14l11-7-11-7Z',
  pause: 'M8 5v14M16 5v14',
  volume: 'M4 10v4h3l4 3V7L7 10H4ZM16 9a4 4 0 0 1 0 6M18.5 6.5a7.5 7.5 0 0 1 0 11',
  mute: 'M4 10v4h3l4 3V7L7 10H4ZM16 9l4 6M20 9l-4 6',
  fullscreen: 'M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5',
  like: 'M7 21H4V10h3m0 11h9.5a2 2 0 0 0 2-1.6l1.3-7A2 2 0 0 0 17.8 10H14l.7-3.5A2.5 2.5 0 0 0 12.3 4L7 10',
  star: 'm12 3 2.8 5.7 6.2.9-4.5 4.4 1 6.2-5.5-3-5.5 3 1-6.2L3 9.6l6.2-.9L12 3Z',
  share: 'M14 4h6v6M20 4l-9 9M10 5H5v14h14v-5',
  'chevron-right': 'm9 5 7 7-7 7',
  plus: 'M12 5v14M5 12h14',
  trash: 'M4 7h16M9 7V4h6v3M6 7l1 14h10l1-14',
  edit: 'M4 20h4L20 8l-4-4L4 16v4ZM14 6l4 4',
  check: 'm5 12 4 4L19 6',
  alert: 'M12 3 2 21h20L12 3ZM12 9v5M12 17h.01',
  clock: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18ZM12 7v5l3 2',
  eye: 'M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12ZM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z',
  message: 'M4 5h16v11H8l-4 4V5Z',
};

interface Props extends Omit<SVGProps<SVGSVGElement>, 'name'> {
  name: IconName;
  size?: number;
}

/** 统一线性图标：stroke 2px、currentColor，避免全站 emoji 图标风格不一致。 */
export function Icon({ name, size = 18, className, ...rest }: Props) {
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      className={className}
      {...rest}
    >
      <path d={PATHS[name]} />
    </svg>
  );
}
