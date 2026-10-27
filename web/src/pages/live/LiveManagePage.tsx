import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCategories } from '@/entities/category/queries';
import { createLiveRoom, type CreateLiveRoomResult } from '@/entities/live/api';
import { useMyLiveRooms } from '@/entities/live/queries';
import { describeError } from '@/shared/api/errors';
import { textOf, LIVE_STATUS_TEXT } from '@/shared/lib/status';
import { EmptyState } from '@/widgets/empty-state/EmptyState';

/**
 * 开播管理页：创建直播间 + 我的直播间列表。
 *
 * 开播动作本身在房间页（主播控制台）完成：创建 → 进入房间 → 开始直播 → 拿推流地址。
 */
export default function LiveManagePage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const categoriesQuery = useCategories();
  const roomsQuery = useMyLiveRooms();

  const [title, setTitle] = useState('');
  const [notice, setNotice] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [error, setError] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: () =>
      createLiveRoom({
        title: title.trim(),
        notice: notice.trim(),
        categoryId: categoryId ? Number(categoryId) : undefined,
      }),
    onSuccess: (result: CreateLiveRoomResult) => {
      setTitle('');
      setNotice('');
      setCategoryId('');
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ['live-mine'] });
      navigate(`/streams/${result.roomId}`);
    },
    onError: (mutationError) => setError(describeError(mutationError)),
  });

  const rooms = roomsQuery.data ?? [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-h1 font-semibold text-ink">开播管理</h1>
        <p className="mt-1 text-meta text-ink-tertiary">
          先创建直播间，再进入房间点「开始直播」拿推流地址；用 OBS 推流后状态才会变成「直播中」。
        </p>
      </div>

      <section className="ui-card p-5">
        <h2 className="mb-3 text-h2 font-medium text-ink">创建直播间</h2>
        <div className="space-y-3">
          <label className="block text-meta text-ink-tertiary">
            标题（必填）
            <input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              maxLength={60}
              placeholder="例如：今晚聊聊重构"
              className="ui-input mt-1.5"
            />
          </label>

          <label className="block text-meta text-ink-tertiary">
            公告（可选）
            <input
              value={notice}
              onChange={(event) => setNotice(event.target.value)}
              maxLength={200}
              placeholder="直播内容、时间安排…"
              className="ui-input mt-1.5"
            />
          </label>

          <label className="block text-meta text-ink-tertiary">
            分区（可选）
            <select
              value={categoryId}
              onChange={(event) => setCategoryId(event.target.value)}
              className="ui-input mt-1.5"
            >
              <option value="">不选分区</option>
              {(categoriesQuery.data ?? []).map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </select>
          </label>

          {error && <p className="text-meta text-danger">{error}</p>}

          <button
            type="button"
            onClick={() => {
              if (!title.trim()) {
                setError('标题不能为空');
                return;
              }
              createMutation.mutate();
            }}
            disabled={createMutation.isPending}
            className="ui-btn-primary"
          >
            {createMutation.isPending ? '创建中…' : '创建并进入直播间'}
          </button>
        </div>
      </section>

      <section className="space-y-3">
        <div className="flex items-baseline justify-between">
          <h2 className="text-h2 font-medium text-ink">我的直播间</h2>
          <span className="text-meta text-ink-tertiary">{rooms.length} 个</span>
        </div>

        {roomsQuery.isPending && <div className="h-24 animate-pulse rounded-lg bg-subtle" />}

        {roomsQuery.isError && (
          <EmptyState
            title="直播间列表加载失败"
            hint="请确认直播服务已启动"
            action={
              <button type="button" onClick={() => void roomsQuery.refetch()} className="ui-btn-primary">
                重试
              </button>
            }
          />
        )}

        {roomsQuery.isSuccess && rooms.length === 0 && (
          <EmptyState title="还没有直播间" hint="在上方创建第一个，然后进房间开播" />
        )}

        {rooms.length > 0 && (
          <ul className="ui-card divide-y divide-line overflow-hidden">
            {rooms.map((room) => (
              <li key={room.roomId} className="flex flex-wrap items-center gap-3 px-4 py-3">
                <span
                  className={`rounded-xs px-1.5 py-0.5 text-micro ${
                    room.status === 'LIVING'
                      ? 'bg-brand text-white'
                      : 'bg-subtle text-ink-secondary'
                  }`}
                >
                  {textOf(LIVE_STATUS_TEXT, room.status)}
                </span>
                <span className="min-w-0 flex-1 truncate text-body text-ink">{room.title}</span>
                <span className="text-meta text-ink-tertiary">房间号 {room.roomId}</span>
                <Link
                  to={`/streams/${room.roomId}`}
                  className="ui-btn-ghost h-8 px-3 text-xs"
                >
                  {room.status === 'LIVING' ? '进入直播间' : '去开播'}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
