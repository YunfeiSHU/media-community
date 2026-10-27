import { Link } from 'react-router-dom';
import type { Video } from '@/entities/video/types';
import { formatCount, formatDuration, relativeTime } from '@/shared/lib/datetime';
import { DefaultCover } from '@/shared/ui/DefaultCover';

interface Props {
  video: Video;
  ownerName?: string;
}

/**
 * 视频卡片：白卡 + 16:9 缩略图。
 *
 * 没有封面时用 DefaultCover（按 id 生成稳定渐变 + 播放三角），
 * 不再显示「标题首字」那种灰底大字。
 */
export function VideoCard({ video, ownerName }: Props) {
  return (
    <Link
      to={`/video/${video.id}`}
      className="group block rounded-md border border-line bg-card p-2 transition-shadow hover:shadow-hover"
    >
      <div className="relative aspect-video w-full overflow-hidden rounded-sm bg-subtle">
        <DefaultCover
          seed={String(video.id || video.title)}
          className="absolute inset-0 h-full w-full"
        />

        {video.coverUrl && (
          <img
            src={video.coverUrl}
            alt={video.title}
            loading="lazy"
            decoding="async"
            className="relative h-full w-full object-cover transition-transform duration-200 group-hover:scale-[1.03]"
            onError={(event) => {
              // 封面挂掉时隐藏图片，露出底下的 DefaultCover，避免破图/空白
              event.currentTarget.style.display = 'none';
            }}
          />
        )}

        {video.durationSec > 0 && (
          <span className="absolute bottom-1.5 right-1.5 rounded-xs bg-black/75 px-1.5 py-0.5 text-micro tabular-nums text-white">
            {formatDuration(video.durationSec)}
          </span>
        )}
      </div>

      <div className="mt-2.5 space-y-1 px-1 pb-1">
        <h3 className="line-clamp-2 text-body font-medium text-ink group-hover:text-brand">
          {video.title}
        </h3>
        <p className="flex items-center gap-1.5 text-meta text-ink-tertiary">
          <span className="truncate">{ownerName || `UID ${video.ownerId}`}</span>
          <span>·</span>
          <span className="shrink-0 tabular-nums">{formatCount(video.viewCount)} 播放</span>
          {video.publishedAt && (
            <>
              <span>·</span>
              <span className="shrink-0">{relativeTime(video.publishedAt)}</span>
            </>
          )}
        </p>
      </div>
    </Link>
  );
}
