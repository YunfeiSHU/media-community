import { Link, useSearchParams } from 'react-router-dom';
import { useCategories } from '@/entities/category/queries';
import { useLivingRooms } from '@/entities/live/queries';
import { useProfiles } from '@/entities/user/queries';
import { useVideos } from '@/entities/video/queries';
import { EmptyState, CardGridSkeleton } from '@/widgets/empty-state/EmptyState';
import { LiveCard } from '@/widgets/live-card/LiveCard';
import { VideoCard } from '@/widgets/video-card/VideoCard';
import { Pagination } from '@/shared/ui/Pagination';

const PAGE_SIZE = 12;

/**
 * 首页：分类胶囊 + 推荐视频 + 正在直播。
 *
 * 每一块独立骨架/错误态——直播接口挂掉不应该让整页白屏。
 */
export default function HomePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const categoryId = Number(searchParams.get('categoryId') ?? '') || undefined;
  const page = Math.max(1, Number(searchParams.get('page') ?? '1') || 1);

  const categoriesQuery = useCategories();
  const videosQuery = useVideos({ page, pageSize: PAGE_SIZE, categoryId });
  const liveQuery = useLivingRooms(8);

  const ownerIds = [
    ...(videosQuery.data?.items ?? []).map((video) => video.ownerId),
    ...(liveQuery.data ?? []).map((room) => room.ownerId),
  ];
  const profilesQuery = useProfiles(ownerIds);
  const nameOf = (uid: string) => profilesQuery.data?.find((profile) => profile.uid === uid)?.nickname;

  const total = videosQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  function selectCategory(id?: number) {
    const next = new URLSearchParams(searchParams);
    if (id) next.set('categoryId', String(id));
    else next.delete('categoryId');
    // 换分类回到第一页
    next.delete('page');
    setSearchParams(next, { replace: true });
  }

  function goPage(next: number) {
    const params = new URLSearchParams(searchParams);
    if (next <= 1) params.delete('page');
    else params.set('page', String(next));
    setSearchParams(params);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  return (
    <div className="relative">
      {/* 首页顶部氛围光：只在这一屏出现，不跟随其他页面 */}
      <div
        aria-hidden
        className="home-glow pointer-events-none absolute left-1/2 top-[-24px] h-[420px] w-screen -translate-x-1/2"
      />

      <div className="relative space-y-7">
      <section aria-label="分类" className="flex gap-2 overflow-x-auto pb-1">
        <button
          type="button"
          onClick={() => selectCategory(undefined)}
          className={`ui-chip ${categoryId ? '' : 'ui-chip-active'}`}
        >
          全部
        </button>
        {(categoriesQuery.data ?? []).map((category) => (
          <button
            key={category.id}
            type="button"
            onClick={() => selectCategory(category.id)}
            className={`ui-chip ${categoryId === category.id ? 'ui-chip-active' : ''}`}
          >
            {category.name}
          </button>
        ))}
      </section>

      <section aria-labelledby="video-heading" className="space-y-4">
        <div className="flex items-baseline justify-between">
          <h2 id="video-heading" className="text-h1 font-semibold text-ink">
            推荐
          </h2>
          {videosQuery.data && (
            <span className="text-meta text-ink-tertiary">
              共 {videosQuery.data.total} 个视频 · 第 {page} / {totalPages} 页
            </span>
          )}
        </div>

        {videosQuery.isPending && <CardGridSkeleton count={PAGE_SIZE} />}

        {videosQuery.isError && (
          <EmptyState
            title="视频加载失败"
            hint={(videosQuery.error as Error)?.message}
            action={
              <button
                type="button"
                onClick={() => void videosQuery.refetch()}
                className="ui-btn-primary"
              >
                重试
              </button>
            }
          />
        )}

        {videosQuery.isSuccess && videosQuery.data.items.length === 0 && (
          <EmptyState title="还没有视频" hint="换个分类看看，或等后端种子数据就绪" />
        )}

        {videosQuery.isSuccess && videosQuery.data.items.length > 0 && (
          <>
            <div className="grid grid-cols-1 gap-x-5 gap-y-7 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {videosQuery.data.items.map((video) => (
                <VideoCard key={video.id} video={video} ownerName={nameOf(video.ownerId)} />
              ))}
            </div>
            <Pagination page={page} totalPages={totalPages} onChange={goPage} className="pt-1" />
          </>
        )}
      </section>

      {(liveQuery.isPending || (liveQuery.data?.length ?? 0) > 0) && (
        <section aria-labelledby="live-heading" className="space-y-4">
          <div className="flex items-baseline justify-between">
            <h2 id="live-heading" className="text-h1 font-semibold text-ink">
              正在直播
            </h2>
            <Link to="/streams" className="text-meta text-ink-tertiary hover:text-brand">
              查看全部
            </Link>
          </div>

          {liveQuery.isPending ? (
            <CardGridSkeleton count={4} />
          ) : (
            <div className="grid grid-cols-1 gap-x-5 gap-y-7 sm:grid-cols-2 lg:grid-cols-4">
              {(liveQuery.data ?? []).map((room) => (
                <LiveCard key={room.roomId} room={room} ownerName={nameOf(room.ownerId)} />
              ))}
            </div>
          )}
        </section>
      )}
      </div>
    </div>
  );
}
