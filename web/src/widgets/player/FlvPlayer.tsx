import { useEffect, useRef, useState } from 'react';
import { LiveControls } from './LiveControls';

interface Props {
  src: string;
  poster?: string;
}

/**
 * HTTP-FLV 播放器（低延迟 2–3s）。
 *
 * 只在用户主动切到「低延迟」时动态加载 flv.js，默认仍是 HLS（兼容性更好）。
 */
export function FlvPlayer({ src, poster }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !src) return;
    // 显式非空：TS 在嵌套函数里不会保留收窄
    const el: HTMLVideoElement = video;

    setError(null);
    let disposed = false;
    let player: { destroy: () => void } | null = null;

    async function attach() {
      // flv.js 是 UMD 包，动态 import 后取 default
      const { default: flvjs } = await import('flv.js');
      if (disposed) return;

      if (!flvjs.isSupported()) {
        setError('当前浏览器不支持 HTTP-FLV，请切换回 HLS');
        return;
      }

      const instance = flvjs.createPlayer(
        { type: 'flv', url: src, isLive: true },
        // 直播不要攒缓冲，否则延迟会越积越大
        { enableStashBuffer: false, stashInitialSize: 128 },
      );
      player = instance;
      instance.attachMediaElement(el);
      instance.load();
      void el.play().catch(() => {
        // 自动播放被拦是预期行为，用户点一下播放键即可
      });
    }

    void attach();

    return () => {
      disposed = true;
      try {
        player?.destroy();
      } catch {
        // destroy 在已卸载时可能抛错，忽略
      }
      el.removeAttribute('src');
      el.load();
    };
  }, [src]);

  return (
    <div className="relative aspect-video w-full overflow-hidden rounded-md bg-black">
      <video
        ref={videoRef}
        poster={poster}
        playsInline
        className="h-full w-full"
        onClick={() => {
          const video = videoRef.current;
          if (!video) return;
          if (video.paused) void video.play().catch(() => {});
          else video.pause();
        }}
      />

      <LiveControls videoRef={videoRef} />

      {error && (
        <div className="absolute inset-0 grid place-items-center bg-black/70 px-4 text-center text-sm text-white">
          {error}
        </div>
      )}
    </div>
  );
}
