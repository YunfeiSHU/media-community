import { Link } from 'react-router-dom';
import type { LiveRoom } from '@/entities/live/types';
import { formatCount } from '@/shared/lib/datetime';
import { LIVE_STATUS_TEXT, textOf } from '@/shared/lib/status';
import { Icon } from '@/shared/ui/Icon';
import { DefaultCover } from '@/shared/ui/DefaultCover';

interface Props {
  room: LiveRoom;
  ownerName?: string;
}

export function LiveCard({ room, ownerName }: Props) {
  return (
    <Link
      to={`/streams/${room.roomId}`}
      className="group block rounded-md border border-line bg-card p-2 transition-shadow hover:shadow-hover"
    >
      <div className="relative aspect-video w-full overflow-hidden rounded-sm bg-subtle">
        <DefaultCover
          seed={String(room.roomId || room.title)}
          kind="live"
          className="absolute inset-0 h-full w-full"
        />

        {room.coverUrl && (
          <img
            src={room.coverUrl}
            alt={room.title}
            loading="lazy"
            className="relative h-full w-full object-cover transition-transform duration-200 group-hover:scale-[1.03]"
            onError={(event) => {
              event.currentTarget.style.display = 'none';
            }}
          />
        )}

        {room.status === 'LIVING' ? (
          <span className="absolute left-2 top-2 flex items-center gap-1 rounded-xs bg-brand px-1.5 py-0.5 text-micro font-medium text-white">
            <span className="h-1.5 w-1.5 rounded-full bg-white" />
            直播中
          </span>
        ) : (
          <span className="absolute left-2 top-2 rounded-xs bg-black/60 px-1.5 py-0.5 text-micro text-white">
            {textOf(LIVE_STATUS_TEXT, room.status)}
          </span>
        )}

        <span className="absolute bottom-1.5 left-2 flex items-center gap-1 text-micro text-white/90">
          <Icon name="eye" size={13} />
          {formatCount(room.onlineCount)}
        </span>
      </div>

      <div className="mt-2.5 space-y-1 px-1 pb-1">
        <h3 className="line-clamp-1 text-body font-medium text-ink group-hover:text-brand">
          {room.title}
        </h3>
        <p className="flex items-center gap-1.5 text-meta text-ink-tertiary">
          <span className="truncate">{ownerName || `UID ${room.ownerId}`}</span>
          <span>·</span>
          <span className="shrink-0 tabular-nums">{formatCount(room.onlineCount)} 人在线</span>
        </p>
      </div>
    </Link>
  );
}
