import { useMemo } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useProfiles } from '@/entities/user/queries';
import { useVideos } from '@/entities/video/queries';
import { useAuthStore } from '@/features/auth/store';
import { CardGridSkeleton, EmptyState } from '@/widgets/empty-state/EmptyState';
import { VideoCard } from '@/widgets/video-card/VideoCard';
import { Avatar } from '@/shared/ui/Avatar';
import { Pagination } from '@/shared/ui/Pagination';

type Tab = 'mine' | 'favorite';

const PAGE_SIZE = 12;

/**
 * 个人空间：我的投稿 / 我的收藏。
 *
 * 两个 tab 共用同一条视频列表接口，区别只是 `?mine=true` 与 `?favorite=true`；
 * 收藏由后端按收藏时间倒序返回，前端不再排序。
 * 12 条一页，页码写入 URL（?page=2），方便刷新/分享保持当前页。
 */
export default function SpacePage() {
  const [params, setParams] = useSearchParams();
  const tab: Tab = params.get('tab') === 'favorite' ? 'favorite' : 'mine';
  const page = Math.max(1, Number(params.get('page') ?? '1') || 1);
  const user = useAuthStore((state) => state.user);

  const videosQuery = useVideos({
    page,
    pageSize: PAGE_SIZE,
    mine: tab === 'mine',
    favorite: tab === 'favorite',
  });

  const videos = videosQuery.data?.items ?? [];
  const total = videosQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const ownerIds = useMemo(() => videos.map((video) => video.ownerId), [videos]);
  const profilesQuery = useProfiles(ownerIds);
  const nameOf = (uid: string) =>
    profilesQuery.data?.find((profile) => profile.uid === uid)?.nickname;

  function goPage(next: number) {
    const nextParams = new URLSearchParams(params);
    if (next <= 1) nextParams.delete('page');
    else nextParams.set('page', String(next));
    setParams(nextParams);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  function switchTab(next: Tab) {
    const nextParams = new URLSearchParams();
    if (next === 'favorite') nextParams.set('tab', 'favorite');
    setParams(nextParams, { replace: true });
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
        {total > 0 && (
          <span className="pb-2 text-meta text-ink-tertiary">
            共 {total} 个 · 第 {page} / {totalPages} 页
          </span>
        )}
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

          <Pagination page={page} totalPages={totalPages} onChange={goPage} className="pt-2" />
        </>
      )}
    </div>
  );
}
