import { Link } from 'react-router-dom';
import { useLivingRooms } from '@/entities/live/queries';
import { useProfiles } from '@/entities/user/queries';
import { useAuthStore } from '@/features/auth/store';
import { CardGridSkeleton, EmptyState } from '@/widgets/empty-state/EmptyState';
import { LiveCard } from '@/widgets/live-card/LiveCard';

export default function LiveListPage() {
  const roomsQuery = useLivingRooms(24);
  const rooms = roomsQuery.data ?? [];
  const profilesQuery = useProfiles(rooms.map((room) => room.ownerId));
  const authStatus = useAuthStore((state) => state.status);

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-h1 font-semibold text-ink">正在直播</h1>
        <div className="flex items-center gap-3 text-meta text-ink-tertiary">
          <span>{rooms.length} 个直播间</span>
          {authStatus === 'authed' && (
            <Link to="/streams/manage" className="ui-btn-primary h-8 px-3 text-xs">
              开播 / 我的直播间
            </Link>
          )}
        </div>
      </div>

      {roomsQuery.isPending && <CardGridSkeleton count={8} />}

      {roomsQuery.isError && (
        <EmptyState
          title="直播列表加载失败"
          hint="直播服务可能没有启动"
          action={
            <button type="button" onClick={() => void roomsQuery.refetch()} className="ui-btn-primary">
              重试
            </button>
          }
        />
      )}

      {roomsQuery.isSuccess && rooms.length === 0 && (
        <EmptyState title="当前没有正在直播的房间" hint="主播开播后会出现在这里" />
      )}

      {roomsQuery.isSuccess && rooms.length > 0 && (
        <div className="grid grid-cols-1 gap-x-5 gap-y-7 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {rooms.map((room) => (
            <LiveCard
              key={room.roomId}
              room={room}
              ownerName={profilesQuery.data?.find((profile) => profile.uid === room.ownerId)?.nickname}
            />
          ))}
        </div>
      )}
    </div>
  );
}
