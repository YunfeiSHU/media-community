import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '@/features/auth/store';
import { fetchInteraction, setFavorited, setLiked } from '@/entities/video/api';
import { usePlayInfo, useVideo } from '@/entities/video/queries';
import { useProfiles } from '@/entities/user/queries';
import { setFollowing, fetchRelationship, type FollowState } from '@/entities/user/api';
import { describeError } from '@/shared/api/errors';
import { formatCount, formatDate, relativeTime } from '@/shared/lib/datetime';
import { textOf, VIDEO_STATUS_TEXT } from '@/shared/lib/status';
import { useDanmaku } from '@/features/danmaku/useDanmaku';
import { useScheduledDanmaku } from '@/features/danmaku/useScheduledDanmaku';
import { DanmakuLayer } from '@/widgets/danmaku-layer/DanmakuLayer';
import { EmptyState } from '@/widgets/empty-state/EmptyState';
import { VideoPlayer } from '@/widgets/player/VideoPlayer';
import { Icon } from '@/shared/ui/Icon';
import { Avatar } from '@/shared/ui/Avatar';

export default function VideoPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const authStatus = useAuthStore((state) => state.status);
  const videoQuery = useVideo(id);
  const playQuery = usePlayInfo(id);
  const authorQuery = useProfiles(videoQuery.data ? [videoQuery.data.ownerId] : []);

  const interactionQuery = useQuery({
    queryKey: ['video-interaction', id],
    queryFn: () => fetchInteraction(id),
    staleTime: 30_000,
    enabled: Boolean(id),
  });

  // 关注状态只在登录后查：未登录调这个接口会 401
  const relationshipQuery = useQuery({
    queryKey: ['follow-state', videoQuery.data?.ownerId],
    queryFn: () => fetchRelationship(videoQuery.data?.ownerId ?? ''),
    staleTime: 30_000,
    enabled: authStatus === 'authed' && Boolean(videoQuery.data?.ownerId),
  });

  const [pending, setPending] = useState<'like' | 'favorite' | null>(null);
  const [followPending, setFollowPending] = useState(false);
  const [followState, setFollowState] = useState<FollowState | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  // ---------------- 点播弹幕 ----------------
  const [currentTime, setCurrentTime] = useState(0);
  const [danmakuOn, setDanmakuOn] = useState(true);
  const [danmakuDraft, setDanmakuDraft] = useState('');
  const danmaku = useDanmaku({ videoId: id }, Boolean(id));
  const scheduledDanmaku = useScheduledDanmaku(danmaku.items, currentTime, danmakuOn);

  const video = videoQuery.data;
  const author = authorQuery.data?.[0];
  const interaction = interactionQuery.data;

  /** 未登录时跳登录并带回跳地址；返回 false 表示调用方应中止。 */
  function requireLogin(): boolean {
    if (authStatus === 'authed') return true;
    navigate(`/login?returnTo=${encodeURIComponent(`/video/${id}`)}`);
    return false;
  }

  async function toggleLike() {
    if (!requireLogin() || !interaction) return;
    setPending('like');
    setNotice(null);
    try {
      const next = await setLiked(id, !interaction.liked);
      queryClient.setQueryData(['video-interaction', id], next);
    } catch (error) {
      setNotice(describeError(error));
    } finally {
      setPending(null);
    }
  }

  async function toggleFavorite() {
    if (!requireLogin() || !interaction) return;
    setPending('favorite');
    setNotice(null);
    try {
      const next = await setFavorited(id, !interaction.favorited);
      queryClient.setQueryData(['video-interaction', id], next);
      // 收藏/取消后让「我的收藏」重新拉取，避免下一页仍是旧列表
      await queryClient.invalidateQueries({ queryKey: ['videos'] });
      if (next.favorited) setNotice('已收藏，可在「我的空间 · 收藏」里找到');
    } catch (error) {
      setNotice(describeError(error));
    } finally {
      setPending(null);
    }
  }

  async function toggleFollow() {
    if (!requireLogin() || !video) return;
    setFollowPending(true);
    setNotice(null);
    const following = followState?.following ?? false;
    try {
      setFollowState(await setFollowing(video.ownerId, !following));
    } catch (error) {
      setNotice(describeError(error));
    } finally {
      setFollowPending(false);
    }
  }

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setNotice('链接已复制');
    } catch {
      setNotice('复制失败，请手动复制地址栏');
    }
  }

  function sendDanmaku() {
    if (!requireLogin()) return;
    const content = danmakuDraft.trim();
    if (!content) return;
    if (danmaku.status !== 'open') {
      setNotice('弹幕连接尚未就绪，请稍后重试');
      return;
    }
    danmaku.send(content, { timeMs: Math.round(currentTime * 1000) });
    setDanmakuDraft('');
  }

  const danmakuStatusText =
    danmaku.status === 'open'
      ? '弹幕已连接'
      : danmaku.status === 'connecting'
        ? '弹幕连接中…'
        : danmaku.status === 'reconnecting'
          ? `弹幕重连中${danmaku.detail ? `（${danmaku.detail}）` : ''}`
          : '弹幕未连接';

  if (videoQuery.isPending) {
    return (
      <div className="grid gap-6 lg:grid-cols-[minmax(0,7fr)_minmax(0,3fr)]">
        <div className="aspect-video w-full animate-[skeleton-pulse_1.2s_ease-in-out_infinite] rounded-md bg-card" />
        <div className="h-48 animate-[skeleton-pulse_1.2s_ease-in-out_infinite] rounded-md bg-card" />
      </div>
    );
  }

  if (videoQuery.isError || !video) {
    return (
      <EmptyState
        title="视频不存在或已下架"
        hint="未发布的视频对游客返回 404，这是预期行为"
      />
    );
  }

  const following = followState?.following ?? relationshipQuery.data?.following ?? false;
  const followerCount =
    followState?.followerCount ??
    relationshipQuery.data?.followerCount ??
    author?.followerCount ??
    0;

  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
      <div className="space-y-4">
        <div className="relative overflow-hidden rounded-lg bg-black">
          <VideoPlayer
            src={playQuery.data?.playbackUrl ?? ''}
            poster={video.coverUrl || undefined}
            onTimeUpdate={setCurrentTime}
          />
          {danmakuOn && <DanmakuLayer items={scheduledDanmaku} />}
        </div>

        {/* 点播弹幕工具条：游客可看，登录后才能发 */}
        <div className="ui-card flex flex-wrap items-center gap-2 p-2">
          <button
            type="button"
            onClick={() => setDanmakuOn((value) => !value)}
            aria-pressed={danmakuOn}
            className={`ui-chip h-8 shrink-0 ${
              danmakuOn ? 'bg-brand text-white hover:bg-brand-hover' : ''
            }`}
          >
            弹幕 {danmakuOn ? '开' : '关'}
          </button>
          <input
            value={danmakuDraft}
            onChange={(event) => setDanmakuDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') sendDanmaku();
            }}
            maxLength={50}
            placeholder={authStatus === 'authed' ? '发一条弹幕…' : '登录后发弹幕'}
            disabled={authStatus !== 'authed'}
            className="h-8 min-w-0 flex-1 rounded-full bg-subtle px-3 text-sm text-ink outline-none placeholder:text-ink-tertiary focus:bg-card disabled:opacity-60"
          />
          <button
            type="button"
            onClick={sendDanmaku}
            disabled={authStatus !== 'authed' || !danmakuDraft.trim()}
            className="ui-btn-primary h-8 shrink-0 px-4 text-xs"
          >
            发送
          </button>
          <span className="text-meta text-ink-tertiary">{danmakuStatusText}</span>
        </div>

        {danmaku.error && <p className="text-meta text-danger">{danmaku.error.message}</p>}

        <div className="space-y-1.5">
          <h1 className="text-h1 font-semibold text-ink">{video.title}</h1>
          <div className="flex flex-wrap items-center gap-3 text-meta text-ink-tertiary">
            <span className="tabular-nums">{formatCount(video.viewCount)} 播放</span>
            {video.publishedAt && <span>{relativeTime(video.publishedAt)}</span>}
            <span>{textOf(VIDEO_STATUS_TEXT, video.status)}</span>
          </div>
        </div>

        {/* 操作区：不放灰色假按钮，未登录点了会跳登录 */}
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => void toggleLike()}
            disabled={pending === 'like'}
            aria-pressed={interaction?.liked ?? false}
            className={`inline-flex h-9 items-center gap-1.5 rounded-full border px-4 text-sm transition-colors disabled:opacity-60 ${
              interaction?.liked
                ? 'border-brand bg-brand text-white'
                : 'border-line bg-card text-ink-secondary hover:bg-subtle'
            }`}
          >
            <Icon name="like" size={16} />
            点赞
            <span className="text-meta opacity-80 tabular-nums">
              {formatCount(interaction?.likeCount ?? 0)}
            </span>
          </button>

          <button
            type="button"
            onClick={() => void toggleFavorite()}
            disabled={pending === 'favorite'}
            aria-pressed={interaction?.favorited ?? false}
            className={`inline-flex h-9 items-center gap-1.5 rounded-full border px-4 text-sm transition-colors disabled:opacity-60 ${
              interaction?.favorited
                ? 'border-brand bg-brand text-white'
                : 'border-line bg-card text-ink-secondary hover:bg-subtle'
            }`}
          >
            <Icon name="star" size={16} />
            收藏
            <span className="text-meta opacity-80 tabular-nums">
              {formatCount(interaction?.favoriteCount ?? 0)}
            </span>
          </button>

          <button type="button" onClick={() => void copyLink()} className="ui-btn-ghost">
            <Icon name="share" size={16} />
            分享
          </button>
        </div>

        {notice && <p className="text-meta text-ink-tertiary">{notice}</p>}

        <section className="ui-card p-4">
          <h2 className="mb-2 text-body font-medium text-ink">简介</h2>
          <p className="whitespace-pre-wrap text-body text-ink-secondary">
            {video.description || '这个 UP 主很懒，什么都没写。'}
          </p>
          {video.tags.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {video.tags.map((tag) => (
                <span key={tag} className="rounded-full bg-subtle px-2.5 py-1 text-micro text-ink-secondary">
                  #{tag}
                </span>
              ))}
            </div>
          )}
        </section>
      </div>

      <aside className="space-y-4">
        <section className="ui-card p-4">
          <h2 className="mb-3 text-body font-medium text-ink">UP 主</h2>
          <div className="flex items-center gap-3">
            <Avatar
              src={author?.avatar}
              alt={author?.nickname ?? 'UP 主'}
              className="h-11 w-11 shrink-0"
            />
            <div className="min-w-0 flex-1">
              <Link
                to={`/user/${video.ownerId}`}
                className="block truncate text-body font-medium text-ink hover:text-brand"
              >
                {author?.nickname ?? `UID ${video.ownerId}`}
              </Link>
              <p className="text-meta text-ink-tertiary">
                {formatCount(followerCount)} 粉丝 · {author?.videoCount ?? 0} 个投稿
              </p>
            </div>
            <button
              type="button"
              onClick={() => void toggleFollow()}
              disabled={followPending}
              className={`h-8 shrink-0 rounded-full px-3.5 text-xs font-medium transition-colors disabled:opacity-60 ${
                following
                  ? 'border border-line bg-card text-ink-secondary hover:bg-subtle'
                  : 'bg-brand text-white hover:bg-brand-hover'
              }`}
            >
              {following ? '已关注' : '关注'}
            </button>
          </div>
          {author?.bio && (
            <p className="mt-3 line-clamp-2 text-meta text-ink-secondary">{author.bio}</p>
          )}
        </section>

        <section className="ui-card p-4 text-meta text-ink-tertiary">
          <h2 className="mb-2 text-body font-medium text-ink">发布信息</h2>
          <p>创建于 {formatDate(video.createdAt)}</p>
          <p>分区：{video.categoryName || '未分类'}</p>
        </section>
      </aside>
    </div>
  );
}
