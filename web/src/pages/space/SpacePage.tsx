import { useEffect, useMemo, useRef } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useProfiles } from '@/entities/user/queries';
import { useInfiniteVideos } from '@/entities/video/queries';
import { useAuthStore } from '@/features/auth/store';
import { CardGridSkeleton, EmptyState } from '@/widgets/empty-state/EmptyState';
import { VideoCard } from '@/widgets/video-card/VideoCard';
import { Avatar } from '@/shared/ui/Avatar';

type Tab = 'mine' | 'favorite';

/**
 * 个人空间：我的投稿 / 我的收藏。
 *
 * 两个 tab 共用同一条视频列表接口，区别只是 `?mine=true` 与 `?favorite=true`；
 * 收藏由后端按收藏时间倒序返回，前端不再排序。
 * 列表按 12 条一页做无限滚动，底部保留「加载更多」按钮作为兜底。
 */
export default function SpacePage() {
  const [params, setParams] = useSearchParams();
  const tab: Tab = params.get('tab') === 'favorite' ? 'favorite' : 'mine';
  const user = useAuthStore((state) => state.user);

  const videosQuery = useInfiniteVideos({
    pageSize: 12,
    mine: tab === 'mine',
    favorite: tab === 'favorite',
  });

  const videos = useMemo(
    () => videosQuery.data?.pages.flatMap((page) => page.items) ?? [],
    [videosQuery.data],
  );
  const total = videosQuery.data?.pages[0]?.total ?? 0;

  const ownerIds = useMemo(() => videos.map((video) => video.ownerId), [videos]);
  const profilesQuery = useProfiles(ownerIds);
  const nameOf = (uid: string) =>
    profilesQuery.data?.find((profile) => profile.uid === uid)?.nickname;

  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = videosQuery;

  useEffect(() => {
    const node = loadMoreRef.current;
    if (!node || !hasNextPage || isFetchingNextPage) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) void fetchNextPage();
      },
      { rootMargin: '400px 0px' },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  function switchTab(next: Tab) {
    setParams(next === 'mine' ? {} : { tab: next }, { replace: true });
  }

  return (
    <div className="space-y-5">
      <section className="ui-card flex flex-wrap items-center gap-4 p-5">
        <Avatar
          src={user?.avatar}
          alt={user?.nickname ?? '我的头像'}
          className="h-14 w-14 shrink-0"
        />
        <div className="min-w-0 flex-1">
          <h1 className="text-h1 font-semibold text-ink">{user?.nickname ?? '我的空间'}</h1>
          <p className="mt-0.5 text-meta text-ink-tertiary">
            {user ? `${user.followerCount} 粉丝 · ${user.followingCount} 关注` : '资料加载中'}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {user && (
            <Link to={`/user/${user.uid}`} className="ui-btn-ghost h-8 px-3 text-xs">
              我的主页
            </Link>
          )}
          <Link to="/settings" className="ui-btn-ghost h-8 px-3 text-xs">
            编辑资料
          </Link>
          <Link to="/studio/videos" className="ui-btn-ghost h-8 px-3 text-xs">
            视频管理
          </Link>
          <Link to="/studio/products" className="ui-btn-ghost h-8 px-3 text-xs">
            商品管理
          </Link>
        </div>
      </section>

      <div className="flex items-end justify-between gap-3 border-b border-line">
        <div className="flex gap-2">
          {(
            [
              ['mine', '我的投稿'],
              ['favorite', '我的收藏'],
            ] as const
          ).map(([key, label]) => (
            <button
              key={key}
              type="button"
              onClick={() => switchTab(key)}
              className={`-mb-px border-b-2 px-3 py-2.5 text-body transition-colors ${
                tab === key
                  ? 'border-brand font-medium text-brand'
                  : 'border-transparent text-ink-secondary hover:text-ink'
              }`}
            >
              {label}
            </button>
          ))}
        </div>
        {total > 0 && <span className="pb-2 text-meta text-ink-tertiary">共 {total} 个</span>}
      </div>

      {videosQuery.isPending && <CardGridSkeleton count={8} />}

      {videosQuery.isError && (
        <EmptyState
          title="加载失败"
          hint={(videosQuery.error as Error)?.message}
          action={
            <button type="button" onClick={() => void videosQuery.refetch()} className="ui-btn-primary">
              重试
            </button>
          }
        />
      )}

      {videosQuery.isSuccess && videos.length === 0 && (
        <EmptyState
          title={tab === 'mine' ? '还没有投稿' : '还没有收藏'}
          hint={tab === 'mine' ? '去投稿页上传第一个视频吧' : '在播放页点「收藏」就会出现在这里'}
          action={
            <Link to={tab === 'mine' ? '/upload' : '/'} className="ui-btn-primary">
              {tab === 'mine' ? '去投稿' : '去逛逛'}
            </Link>
          }
        />
      )}

      {videosQuery.isSuccess && videos.length > 0 && (
        <>
          <div className="grid grid-cols-1 gap-x-5 gap-y-7 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {videos.map((video) => (
              <VideoCard key={video.id} video={video} ownerName={nameOf(video.ownerId)} />
            ))}
          </div>

          <div ref={loadMoreRef} className="flex flex-col items-center gap-2 py-2">
            {isFetchingNextPage && <span className="text-meta text-ink-tertiary">加载更多…</span>}
            {hasNextPage && !isFetchingNextPage && (
              <button
                type="button"
                onClick={() => void fetchNextPage()}
                className="ui-btn-ghost h-9 px-4 text-xs"
              >
                加载更多
              </button>
            )}
            {!hasNextPage && <span className="text-meta text-ink-tertiary">没有更多了</span>}
          </div>
        </>
      )}
    </div>
  );
}
