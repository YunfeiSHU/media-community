import { useEffect, useRef, useState } from 'react';
import { LiveControls } from './LiveControls';

interface Props {
  src: string;
  poster?: string;
  /** 播放进度回调（秒），点播弹幕用来对齐 timeMs。 */
  onTimeUpdate?: (seconds: number) => void;
  /**
   * 直播模式：关闭原生 controls（没有进度条/seek），改用 LiveControls。
   * 点播保持原生 controls。
   */
  live?: boolean;
}

/**
 * HLS 播放器。
 *
 * hls.js 约 500KB，用动态 import 只在播放页加载；Safari 原生支持 HLS 则直接用 video.src。
 */
export function VideoPlayer({ src, poster, onTimeUpdate, live = false }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [waiting, setWaiting] = useState(false);

  useEffect(() => {
    // 用 const 捕获：闭包里 TS 不会保留对 ref.current 的空值收窄
    const media = videoRef.current;
    if (!media || !src) return;
    // 显式标注为非空：TS 在嵌套函数里不会保留对 media 的收窄
    const el: HTMLVideoElement = media;

    setError(null);
    setWaiting(false);
    let disposed = false;
    let hls: { destroy: () => void } | null = null;

    // 直播默认静音并自动起播：浏览器只允许“静音自动播放”，
    // 且用户在等待直播流期间点播放时 src 还没挂上，这里补一次起播。
    if (live) el.muted = true;
    const onLoadedMetadata = () => {
      if (live) void el.play().catch(() => {});
    };
    el.addEventListener('loadedmetadata', onLoadedMetadata);

    const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

    /**
     * 直播 HLS 是边推边生成的：推流刚开始的 2–3 秒内 m3u8 还是 404。
     * 直接交给 hls.js 会因 manifest 404 判 fatal 并停住（表现为“HLS 用不了，
     * FLV 正常”），所以先轮询到 200 再 attach。
     */
    async function waitForLiveManifest(): Promise<boolean> {
      for (let attempt = 0; attempt < 30; attempt += 1) {
        if (disposed) return false;
        try {
          const response = await fetch(src, { cache: 'no-store' });
          if (response.ok) return true;
        } catch {
          // 网络抖动继续重试
        }
        await sleep(1000);
      }
      return false;
    }

    async function attach() {
      if (live) {
        setWaiting(true);
        const ready = await waitForLiveManifest();
        if (disposed) return;
        if (!ready) {
          setWaiting(false);
          setError('直播流暂未就绪，请稍后刷新重试');
          return;
        }
        setWaiting(false);
      }

      // 优先 hls.js：Chromium 对 `canPlayType('application/vnd.apple.mpegurl')`
      // 会返回 "maybe"，但实际并不能原生播放 HLS；先判 native 会走进死路
      // （表现为“HLS 没反应、FLV 正常”）。只有 hls.js 不支持时才退回原生。
      const { default: Hls } = await import('hls.js');
      if (disposed) return;

      if (!Hls.isSupported()) {
        if (el.canPlayType('application/vnd.apple.mpegurl')) {
          el.src = src;
          return;
        }
        setError('当前浏览器不支持 HLS 播放');
        return;
      }

      const instance = new Hls({
        enableWorker: true,
        // 直播切片偶发 404/断流时多试几次，不要一次失败就判死
        manifestLoadingMaxRetry: 6,
        manifestLoadingRetryDelay: 1000,
        levelLoadingMaxRetry: 6,
        levelLoadingRetryDelay: 1000,
        fragLoadingMaxRetry: 6,
        fragLoadingRetryDelay: 1000,
      });
      hls = instance;
      instance.loadSource(src);
      instance.attachMedia(el);
      instance.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) setError('播放失败，请刷新重试');
      });
    }

    void attach();

    return () => {
      disposed = true;
      el.removeEventListener('loadedmetadata', onLoadedMetadata);
      hls?.destroy();
      el.removeAttribute('src');
      el.load();
    };
  }, [src, live]);

  return (
    <div className="relative aspect-video w-full overflow-hidden rounded-md bg-black">
      <video
        ref={videoRef}
        poster={poster}
        controls={!live}
        playsInline
        preload="metadata"
        className="h-full w-full"
        onClick={() => {
          if (!live) return;
          const video = videoRef.current;
          if (!video) return;
          if (video.paused) void video.play().catch(() => {});
          else video.pause();
        }}
        onTimeUpdate={(event) => onTimeUpdate?.(event.currentTarget.currentTime)}
        onSeeked={(event) => onTimeUpdate?.(event.currentTarget.currentTime)}
      />

      {live && <LiveControls videoRef={videoRef} />}

      {waiting && !error && (
        <div className="absolute inset-0 z-20 grid place-items-center bg-black/70 text-sm text-white">
          正在等待直播流…
        </div>
      )}

      {error && (
        <div className="absolute inset-0 grid place-items-center bg-black/70 text-sm text-white">
          {error}
        </div>
      )}

      {!src && !error && (
        <div className="absolute inset-0 grid place-items-center bg-[#18191c] text-sm text-ink-tertiary">
          暂无播放地址
        </div>
      )}
    </div>
  );
}
