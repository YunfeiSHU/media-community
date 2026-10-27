import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { fetchFollowers, fetchFollowing, fetchRelationship, setFollowing, type FollowState } from '@/entities/user/api';
import { useProfiles } from '@/entities/user/queries';
import { useVideos } from '@/entities/video/queries';
import { useAuthStore } from '@/features/auth/store';
import { describeError } from '@/shared/api/errors';
import { formatCount } from '@/shared/lib/datetime';
import { EmptyState, CardGridSkeleton } from '@/widgets/empty-state/EmptyState';
import { VideoCard } from '@/widgets/video-card/VideoCard';
import { Avatar } from '@/shared/ui/Avatar';

type Tab = 'works' | 'followers' | 'following';

/** 公开用户主页：资料 + 作品 + 粉丝/关注。 */
export default function UserProfilePage() {
  const { uid = '' } = useParams();
  const authStatus = useAuthStore((state) => state.status);
  const me = useAuthStore((state) => state.user);
  const isSelf = Boolean(me?.uid && me.uid === uid);

  const [tab, setTab] = useState<Tab>('works');
  const [followState, setFollowState] = useState<FollowState | null>(null);
  const [followPending, setFollowPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const profileQuery = useProfiles(uid ? [uid] : []);
  const profile = profileQuery.data?.[0];
  const videosQuery = useVideos({ ownerId: uid, pageSize: 24 });
  const videos = videosQuery.data?.items ?? [];

  const relationshipQuery = useQuery({
    queryKey: ['follow-state', uid],
    queryFn: () => fetchRelationship(uid),
    enabled: authStatus === 'authed' && Boolean(uid) && !isSelf,
    staleTime: 30_000,
  });

  const followersQuery = useQuery({
    queryKey: ['followers', uid],
    queryFn: () => fetchFollowers(uid),
    enabled: tab === 'followers' && Boolean(uid),
  });

  const followingQuery = useQuery({
    queryKey: ['following', uid],
    queryFn: () => fetchFollowing(uid),
    enabled: tab === 'following' && Boolean(uid),
  });

  const following = followState?.following ?? relationshipQuery.data?.following ?? false;
  const followerCount =
    followState?.followerCount ?? relationshipQuery.data?.followerCount ?? profile?.followerCount ?? 0;

  async function toggleFollow() {
    if (authStatus !== 'authed') return;
    setFollowPending(true);
    setError(null);
    try {
      setFollowState(await setFollowing(uid, !following));
    } catch (followError) {
      setError(describeError(followError));
    } finally {
      setFollowPending(false);
    }
  }

  if (profileQuery.isPending) {
    return <CardGridSkeleton count={4} />;
  }

  if (!profile) {
    return <EmptyState title="用户不存在" hint="该用户可能已注销或 uid 有误" />;
  }

  const tabs: Array<{ key: Tab; label: string }> = [
    { key: 'works', label: `作品 ${videos.length}` },
    { key: 'followers', label: `粉丝 ${formatCount(followerCount)}` },
    { key: 'following', label: `关注 ${formatCount(profile.followingCount)}` },
  ];

  return (
    <div className="space-y-5">
      <section className="ui-card p-5">
        <div className="flex flex-wrap items-center gap-4">
          <Avatar
            src={profile.avatar}
            alt={profile.nickname}
            className="h-16 w-16 shrink-0"
          />
          <div className="min-w-0 flex-1">
            <h1 className="truncate text-h1 font-semibold text-ink">{profile.nickname}</h1>
            <p className="mt-1 text-meta text-ink-tertiary">
              {formatCount(followerCount)} 粉丝 · {formatCount(profile.followingCount)} 关注 ·{' '}
              {videos.length} 个投稿
            </p>
            {profile.bio && <p className="mt-2 text-body text-ink-secondary">{profile.bio}</p>}
          </div>
          {isSelf ? (
            <Link to="/settings" className="ui-btn-ghost">
              编辑资料
            </Link>
          ) : (
            <button
              type="button"
              onClick={() => void toggleFollow()}
              disabled={followPending || authStatus !== 'authed'}
              className={`h-9 rounded-full px-4 text-sm font-medium transition-colors disabled:opacity-60 ${
                following
                  ? 'border border-line bg-card text-ink-secondary hover:bg-subtle'
                  : 'bg-brand text-white hover:bg-brand-hover'
              }`}
            >
              {authStatus !== 'authed' ? '登录后关注' : following ? '已关注' : '关注'}
            </button>
          )}
        </div>
        {error && <p className="mt-2 text-meta text-danger">{error}</p>}
      </section>

      <div className="flex gap-1 border-b border-line">
        {tabs.map((item) => (
          <button
            key={item.key}
            type="button"
            onClick={() => setTab(item.key)}
            className={`-mb-px border-b-2 px-3 py-2.5 text-body transition-colors ${
              tab === item.key
                ? 'border-brand font-medium text-brand'
                : 'border-transparent text-ink-secondary hover:text-ink'
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>

      {tab === 'works' && (
        <>
          {videosQuery.isPending && <CardGridSkeleton count={8} />}
          {videosQuery.isError && <EmptyState title="作品加载失败" />}
          {videosQuery.isSuccess && videos.length === 0 && (
            <EmptyState title="还没有作品" hint="发布视频后会出现在这里" />
          )}
          {videos.length > 0 && (
            <div className="grid grid-cols-1 gap-x-5 gap-y-7 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {videos.map((video) => (
                <VideoCard key={video.id} video={video} ownerName={profile.nickname} />
              ))}
            </div>
          )}
        </>
      )}

      {tab !== 'works' && (
        <UserList
          pending={tab === 'followers' ? followersQuery.isPending : followingQuery.isPending}
          error={tab === 'followers' ? followersQuery.isError : followingQuery.isError}
          items={(tab === 'followers' ? followersQuery.data?.items : followingQuery.data?.items) ?? []}
          emptyHint={tab === 'followers' ? '还没有粉丝' : '还没有关注任何人'}
        />
      )}
    </div>
  );
}

interface UserListProps {
  pending: boolean;
  error: boolean;
  items: Array<{ uid: string; nickname: string; avatar: string; bio: string }>;
  emptyHint: string;
}

function UserList({ pending, error, items, emptyHint }: UserListProps) {
  if (pending) return <CardGridSkeleton count={4} />;
  if (error) return <EmptyState title="列表加载失败" />;
  if (items.length === 0) return <EmptyState title={emptyHint} />;

  return (
    <ul className="ui-card divide-y divide-line overflow-hidden">
      {items.map((item) => (
        <li key={item.uid}>
          <Link to={`/user/${item.uid}`} className="flex items-center gap-3 px-4 py-3 hover:bg-subtle">
            <Avatar src={item.avatar} alt={item.nickname} className="h-10 w-10 shrink-0" />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-body text-ink">{item.nickname}</span>
              {item.bio && (
                <span className="block truncate text-meta text-ink-tertiary">{item.bio}</span>
              )}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}
