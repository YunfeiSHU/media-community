import { useEffect, useState, type RefObject } from 'react';

interface Props {
  videoRef: RefObject<HTMLVideoElement | null>;
}

/**
 * 直播控制条：只保留 播放/暂停、静音、音量、全屏。
 *
 * 刻意**没有进度条**：直播是实时流，seek 回看没有意义，原生 controls 的
 * 进度条还会让观众误以为可以拖动。点播继续用 VideoPlayer 的原生 controls。
 */
export function LiveControls({ videoRef }: Props) {
  const [paused, setPaused] = useState(true);
  const [muted, setMuted] = useState(false);
  const [volume, setVolume] = useState(1);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const sync = () => {
      setPaused(video.paused);
      setMuted(video.muted);
      setVolume(video.volume);
    };
    sync();
    video.addEventListener('play', sync);
    video.addEventListener('pause', sync);
    video.addEventListener('volumechange', sync);
    return () => {
      video.removeEventListener('play', sync);
      video.removeEventListener('pause', sync);
      video.removeEventListener('volumechange', sync);
    };
  }, [videoRef]);

  function togglePlay() {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) void video.play().catch(() => {});
    else video.pause();
  }

  function toggleMute() {
    const video = videoRef.current;
    if (!video) return;
    video.muted = !video.muted;
  }

  function changeVolume(value: number) {
    const video = videoRef.current;
    if (!video) return;
    video.volume = value;
    video.muted = value === 0;
  }

  function toggleFullscreen() {
    const video = videoRef.current;
    if (!video) return;
    if (document.fullscreenElement) {
      void document.exitFullscreen();
      return;
    }
    void video.parentElement?.requestFullscreen?.().catch(() => {});
  }

  return (
    <div className="pointer-events-none absolute inset-0 z-10">
      {paused && (
        <button
          type="button"
          onClick={togglePlay}
          aria-label="播放直播"
          className="pointer-events-auto absolute inset-0 grid place-items-center"
        >
          <span className="grid h-14 w-14 place-items-center rounded-full bg-black/55 text-2xl text-white backdrop-blur-sm">
            ▶
          </span>
        </button>
      )}

      <div className="pointer-events-auto absolute inset-x-0 bottom-0 flex items-center gap-3 bg-gradient-to-t from-black/75 to-transparent px-3 pb-2 pt-8 text-white">
        <button
          type="button"
          onClick={togglePlay}
          aria-label={paused ? '播放' : '暂停'}
          className="grid h-8 w-8 place-items-center rounded text-lg hover:bg-white/10"
        >
          {paused ? '▶' : '❚❚'}
        </button>

        <span className="rounded-sm bg-brand px-1.5 py-0.5 text-[10px] font-medium">直播</span>

        <button
          type="button"
          onClick={toggleMute}
          aria-label={muted ? '取消静音' : '静音'}
          className="grid h-8 w-8 place-items-center rounded text-base hover:bg-white/10"
        >
          {muted ? '🔇' : '🔊'}
        </button>

        <input
          type="range"
          min={0}
          max={1}
          step={0.05}
          value={muted ? 0 : volume}
          onChange={(event) => changeVolume(Number(event.target.value))}
          aria-label="音量"
          className="w-20 accent-[var(--brand-pink)]"
        />

        <div className="flex-1" />

        <button
          type="button"
          onClick={toggleFullscreen}
          aria-label="全屏"
          className="grid h-8 w-8 place-items-center rounded text-base hover:bg-white/10"
        >
          ⛶
        </button>
      </div>
    </div>
  );
}
