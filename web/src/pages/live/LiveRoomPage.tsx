import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useLivePlay, useLiveRoom, useOnline } from '@/entities/live/queries';
import { sendHeartbeat, startLive, stopLive, type StartLiveResult } from '@/entities/live/api';
import { useProfiles } from '@/entities/user/queries';
import { useDanmaku } from '@/features/danmaku/useDanmaku';
import { useAuthStore } from '@/features/auth/store';
import { ApiError, describeError } from '@/shared/api/errors';
import { getGuestSessionId } from '@/shared/lib/guest';
import { formatCount } from '@/shared/lib/datetime';
import { textOf, LIVE_STATUS_TEXT } from '@/shared/lib/status';
import { DanmakuLayer } from '@/widgets/danmaku-layer/DanmakuLayer';
import { EmptyState } from '@/widgets/empty-state/EmptyState';
import { VideoPlayer } from '@/widgets/player/VideoPlayer';
import { FlvPlayer } from '@/widgets/player/FlvPlayer';

type Protocol = 'hls' | 'flv';

export default function LiveRoomPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();

  const authStatus = useAuthStore((state) => state.status);
  const user = useAuthStore((state) => state.user);
  const queryClient = useQueryClient();
  const roomQuery = useLiveRoom(id);
  const playQuery = useLivePlay(id);
  const onlineQuery = useOnline(id);
  const room = roomQuery.data;
  const isLiving = room?.status === 'LIVING';
  const isOwner = authStatus === 'authed' && Boolean(user?.uid) && user?.uid === room?.ownerId;

  const profilesQuery = useProfiles(room ? [room.ownerId] : []);
  const owner = profilesQuery.data?.[0];

  const danmaku = useDanmaku({ roomId: id }, Boolean(isLiving));

  const [draft, setDraft] = useState('');
  const [cooldown, setCooldown] = useState(0);
  const [protocol, setProtocol] = useState<Protocol>('hls');
  const [stream, setStream] = useState<StartLiveResult | null>(null);
  const [ownerBusy, setOwnerBusy] = useState(false);
  const [ownerMessage, setOwnerMessage] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // 开播后状态先是 STARTING，等 SRS on_publish 回调才变 LIVING：
  // 主播页必须轮询，不能点完就假设已经开播。
  useEffect(() => {
    if (room?.status !== 'STARTING') return;
    const timer = window.setInterval(() => {
      void queryClient.invalidateQueries({ queryKey: ['live-room', id] });
      void queryClient.invalidateQueries({ queryKey: ['live-play', id] });
    }, 3000);
    return () => window.clearInterval(timer);
  }, [room?.status, id, queryClient]);

  // 观众心跳：登录用户靠网关注入身份，游客靠 sessionId；房间未开播就停止
  useEffect(() => {
    if (!isLiving) return;
    let stopped = false;

    async function tick() {
      if (stopped) return;
      try {
        await sendHeartbeat(id, authStatus === 'authed' ? null : getGuestSessionId());
      } catch (error) {
        // 409/70001 表示已下播：静默停止，不弹红
        if (error instanceof ApiError && (error.status === 409 || error.code === 70001)) {
          stopped = true;
        }
      }
    }

    void tick();
    const timer = window.setInterval(() => void tick(), 20_000);

    return () => {
      stopped = true;
      window.clearInterval(timer);
    };
  }, [id, isLiving, authStatus]);

  // 发送限流：后端通过 retryAfterMs 告诉前端等多久
  useEffect(() => {
    if (!danmaku.error?.retryAfterMs) return;
    setCooldown(Math.max(1, Math.ceil(danmaku.error.retryAfterMs / 1000)));
  }, [danmaku.error]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = window.setTimeout(() => setCooldown((value) => value - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [cooldown]);

  useEffect(() => {
    const element = listRef.current;
    if (element) element.scrollTop = element.scrollHeight;
  }, [danmaku.items.length]);

  function submit(event: React.FormEvent) {
    event.preventDefault();
    if (authStatus !== 'authed') {
      navigate(`/login?returnTo=${encodeURIComponent(`/streams/${id}`)}`);
      return;
    }
    if (cooldown > 0) return;
    danmaku.send(draft);
    setDraft('');
  }

  async function handleStart() {
    setOwnerBusy(true);
    setOwnerMessage(null);
    try {
      const result = await startLive(id);
      setStream(result);
      setOwnerMessage(
        result.srsAvailable
          ? '已生成推流地址，等待收到 SRS 回调后才会变成「直播中」'
          : 'SRS 不可用：开播已记录，但实际推流会失败',
      );
      await queryClient.invalidateQueries({ queryKey: ['live-room', id] });
    } catch (error) {
      setOwnerMessage(describeError(error));
    } finally {
      setOwnerBusy(false);
    }
  }

  async function handleStop() {
    setOwnerBusy(true);
    setOwnerMessage(null);
    try {
      const result = await stopLive(id);
      setOwnerMessage(result.streamKicked ? '已关播并通知 SRS 断流' : '已关播（SRS 未连上，重连会因密钥失效被拒）');
      setStream(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['live-room', id] }),
        queryClient.invalidateQueries({ queryKey: ['live-play', id] }),
      ]);
    } catch (error) {
      setOwnerMessage(describeError(error));
    } finally {
      setOwnerBusy(false);
    }
  }

  /**
   * OBS 是「服务器 + 串流密钥」两栏拼接，必须这样拆：
   *   pushUrl = rtmp://host:1936/live/<streamKey>?session=<sessionId>
   *   server  = rtmp://host:1936/live
   *   key     = <streamKey>?session=<sessionId>
   * 若把 streamKey 留在 server 里，SRS 会把 app 认成 live/<streamKey>，
   * on_publish 回调校验 app 失败（70002），表现为 OBS 推流被拒。
   */
  function splitPushUrl(pushUrl: string): { server: string; key: string; full: string } {
    const [base, session] = pushUrl.split('?session=');
    const trimmed = base.replace(/\/+$/, '');
    const slash = trimmed.lastIndexOf('/');
    if (slash < 0) return { server: pushUrl, key: '', full: pushUrl };
    const server = trimmed.slice(0, slash);
    const streamKey = trimmed.slice(slash + 1);
    return {
      server,
      key: session ? `${streamKey}?session=${session}` : streamKey,
      full: pushUrl,
    };
  }

  async function copyText(text: string, label: string) {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setOwnerMessage(`${label}已复制`);
    } catch {
      setOwnerMessage('复制失败，请手动选择文本复制');
    }
  }

  if (roomQuery.isPending) {
    return (
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="aspect-video animate-pulse rounded-lg bg-subtle" />
        <div className="h-72 animate-pulse rounded-lg bg-subtle" />
      </div>
    );
  }

  if (roomQuery.isError || !room) {
    return <EmptyState title="直播间不存在" hint="可能已被主播删除" />;
  }

  const hlsUrl = playQuery.data?.playUrls?.hls ?? '';
  const flvUrl = playQuery.data?.playUrls?.httpFlv ?? '';
  const effectiveProtocol: Protocol = protocol === 'flv' && flvUrl ? 'flv' : 'hls';
  const playError = playQuery.error instanceof ApiError ? playQuery.error : null;
  const online = onlineQuery.data?.onlineCount ?? room.onlineCount;
  const pushParts = stream ? splitPushUrl(stream.pushUrl) : null;

  return (
    <div className="space-y-4">
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="space-y-3">
          <div className="relative overflow-hidden rounded-lg bg-black">
            {effectiveProtocol === 'flv' ? (
              <FlvPlayer src={isLiving ? flvUrl : ''} poster={room.coverUrl || undefined} />
            ) : (
              <VideoPlayer src={isLiving ? hlsUrl : ''} poster={room.coverUrl || undefined} live />
            )}
            {isLiving && <DanmakuLayer items={danmaku.items} />}

            {!isLiving && (
              <div className="absolute inset-0 grid place-items-center bg-black/70 text-sm text-white">
                <div className="text-center">
                  <p className="text-base font-medium">{textOf(LIVE_STATUS_TEXT, room.status)}</p>
                  <p className="mt-1 text-xs text-white/70">主播开播后这里会自动可以观看</p>
                </div>
              </div>
            )}

            {isLiving && playError && (
              <div className="absolute inset-0 grid place-items-center bg-black/70 text-sm text-white">
                {playError.code === 70001 ? '直播刚刚结束' : playError.message}
              </div>
            )}
          </div>

          <div className="space-y-1">
            <h1 className="text-h1 font-semibold text-ink">{room.title}</h1>
            <div className="flex flex-wrap items-center gap-3 text-meta text-ink-tertiary">
              <span
                className={`rounded-xs px-1.5 py-0.5 ${
                  isLiving ? 'bg-brand text-white' : 'bg-subtle text-ink-secondary'
                }`}
              >
                {textOf(LIVE_STATUS_TEXT, room.status)}
              </span>
              <span className="tabular-nums">{formatCount(online)} 人在线</span>
              <span className="tabular-nums">
                累计 {formatCount(onlineQuery.data?.viewerTotal ?? room.viewerTotal)} 看过
              </span>
              <span>
                主播{' '}
                <Link to={`/user/${room.ownerId}`} className="text-ink-secondary hover:text-brand">
                  {owner?.nickname ?? `UID ${room.ownerId}`}
                </Link>
              </span>
            </div>
            {room.notice && <p className="text-meta text-ink-secondary">{room.notice}</p>}

            {isLiving && flvUrl && (
              <div className="flex items-center gap-2 pt-1 text-meta text-ink-tertiary">
                <span>画质</span>
                <button
                  type="button"
                  onClick={() => setProtocol('hls')}
                  className={`rounded-full px-2.5 py-1 transition-colors ${
                    effectiveProtocol === 'hls'
                      ? 'bg-brand text-white'
                      : 'bg-subtle text-ink-secondary hover:text-ink'
                  }`}
                >
                  HLS（兼容）
                </button>
                <button
                  type="button"
                  onClick={() => setProtocol('flv')}
                  className={`rounded-full px-2.5 py-1 transition-colors ${
                    effectiveProtocol === 'flv'
                      ? 'bg-brand text-white'
                      : 'bg-subtle text-ink-secondary hover:text-ink'
                  }`}
                >
                  FLV（低延迟 2–3s）
                </button>
              </div>
            )}
          </div>

          {isOwner && (
            <section className="ui-card space-y-3 p-4">
              <div className="flex items-center justify-between">
                <h2 className="text-body font-medium text-ink">主播控制台</h2>
                <span className="text-meta text-ink-tertiary">
                  当前状态：{textOf(LIVE_STATUS_TEXT, room.status)}
                </span>
              </div>

              {room.status === 'LIVING' || room.status === 'STARTING' ? (
                <button
                  type="button"
                  onClick={() => void handleStop()}
                  disabled={ownerBusy}
                  className="ui-btn-ghost"
                >
                  {ownerBusy ? '处理中…' : '结束直播'}
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => void handleStart()}
                  disabled={ownerBusy}
                  className="ui-btn-primary"
                >
                  {ownerBusy ? '处理中…' : '开始直播'}
                </button>
              )}

              {room.status === 'FAILED' && (
                <p className="rounded-sm bg-subtle p-2 text-meta text-ink-secondary">
                  上次开播在 60s 内没有收到 SRS 的推流回调，已自动标记为失败。
                  重新点「开始直播」拿新的服务器/串流密钥，并确认 OBS 两栏是分开填的。
                </p>
              )}

              {(stream || room.status === 'STARTING') && (
                <div className="space-y-2">
                  {stream && pushParts ? (
                    <>
                      <div className="space-y-1">
                        <p className="text-meta text-ink-tertiary">服务器（OBS「服务器」栏）</p>
                        <div className="flex gap-2">
                          <code className="min-w-0 flex-1 break-all rounded-sm bg-subtle p-2 text-meta text-ink-secondary">
                            {pushParts.server}
                          </code>
                          <button
                            type="button"
                            onClick={() => void copyText(pushParts.server, '服务器地址')}
                            className="ui-btn-ghost h-8 shrink-0 px-3 text-xs"
                          >
                            复制
                          </button>
                        </div>
                      </div>

                      <div className="space-y-1">
                        <p className="text-meta text-ink-tertiary">
                          串流密钥（OBS「串流密钥」栏，含 ?session=）
                        </p>
                        <div className="flex gap-2">
                          <code className="min-w-0 flex-1 break-all rounded-sm bg-subtle p-2 text-meta text-ink-secondary">
                            {pushParts.key}
                          </code>
                          <button
                            type="button"
                            onClick={() => void copyText(pushParts.key, '串流密钥')}
                            className="ui-btn-primary h-8 shrink-0 px-3 text-xs"
                          >
                            复制
                          </button>
                        </div>
                      </div>

                      <details className="text-meta text-ink-tertiary">
                        <summary className="cursor-pointer">查看完整推流地址</summary>
                        <code className="mt-1 block break-all rounded-sm bg-subtle p-2">
                          {pushParts.full}
                        </code>
                      </details>

                      <p className="text-meta text-ink-tertiary">
                        注意：不要把串流密钥粘到「服务器」栏，服务器只填到 <code>/live</code>；
                        否则 SRS 会把 app 识别成 <code>live/&lt;streamKey&gt;</code>，
                        on_publish 校验失败（app 不匹配），表现为 OBS 推流被拒。
                        收到 SRS <code>on_publish</code> 后状态才会变「直播中」。
                      </p>
                    </>
                  ) : (
                    <p className="text-xs text-ink-tertiary">
                      推流地址已生成但页面刷新丢失——每次开播都会换密钥，重新点「开始直播」可再次获取（幂等，不换密钥）。
                    </p>
                  )}
                </div>
              )}

              {ownerMessage && <p className="text-xs text-ink-tertiary">{ownerMessage}</p>}
            </section>
          )}
        </div>

        <aside className="ui-card flex h-[480px] flex-col xl:h-auto">
          <div className="flex items-center justify-between border-b border-line px-3 py-2.5">
            <span className="text-body font-medium text-ink">弹幕</span>
            <span className="text-meta text-ink-tertiary">
              {danmaku.status === 'open'
                ? '已连接'
                : danmaku.status === 'reconnecting'
                  ? `重连中${danmaku.detail ? ` · ${danmaku.detail}` : ''}`
                  : danmaku.status === 'connecting'
                    ? '连接中…'
                    : (danmaku.detail ?? '未连接')}
            </span>
          </div>

          <div ref={listRef} className="flex-1 space-y-1.5 overflow-y-auto px-3 py-2.5 text-body">
            {danmaku.systemMessage && (
              <p className="rounded-sm bg-subtle px-2 py-1 text-meta text-ink-tertiary">
                {danmaku.systemMessage}
              </p>
            )}
            {danmaku.items.length === 0 && (
              <p className="text-meta text-ink-tertiary">还没有人发弹幕，来抢个沙发</p>
            )}
            {danmaku.items.map((item) => (
              <p key={item.clientMsgId || item.danmakuId} className="leading-6">
                <span className="mr-1 text-meta text-brand">{item.nickname || '匿名'}</span>
                <span className="text-ink">{item.content}</span>
              </p>
            ))}
          </div>

          <form onSubmit={submit} className="border-t border-line p-2.5">
            <div className="flex gap-2">
              <input
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                placeholder={isLiving ? '发一条弹幕…' : '未开播，暂时不能发弹幕'}
                disabled={!isLiving || cooldown > 0}
                maxLength={100}
                className="h-9 min-w-0 flex-1 rounded-full bg-subtle px-3.5 text-sm text-ink outline-none placeholder:text-ink-tertiary focus:bg-card disabled:opacity-60"
              />
              <button
                type="submit"
                disabled={!isLiving || cooldown > 0 || !draft.trim()}
                className="ui-btn-primary h-9 shrink-0 px-4"
              >
                {cooldown > 0 ? `${cooldown}s` : '发送'}
              </button>
            </div>
            {danmaku.error && cooldown <= 0 && (
              <p className="mt-1 text-meta text-danger">{danmaku.error.message}</p>
            )}
          </form>
        </aside>
      </div>

      <p className="text-meta text-ink-tertiary">
        弹幕连接关闭码：1000 正常 / 4001 同账号连接过多 / 1003 协议异常。在线人数每 8 秒轮询一次。
      </p>
    </div>
  );
}


