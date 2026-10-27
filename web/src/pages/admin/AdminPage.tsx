import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCategories } from '@/entities/category/queries';
import {
  createCategory,
  createSensitiveWord,
  fetchSensitiveWords,
  forceCloseRoom,
  lookupUser,
  reloadSensitiveWords,
  type AdminUser,
  type WordStrategy,
} from '@/entities/admin/api';
import { offlineVideo } from '@/entities/video/api';
import { useAuthStore } from '@/features/auth/store';
import { describeError } from '@/shared/api/errors';
import { EmptyState } from '@/widgets/empty-state/EmptyState';

type Tab = 'categories' | 'words' | 'live' | 'users' | 'videos';

/** 管理后台：分类、敏感词、强制关播、用户查询、视频下架。 */
export default function AdminPage() {
  const user = useAuthStore((state) => state.user);
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<Tab>('categories');
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // 分类
  const categoriesQuery = useCategories();
  const [categoryName, setCategoryName] = useState('');
  const [categorySort, setCategorySort] = useState('0');

  // 敏感词
  const wordsQuery = useQuery({
    queryKey: ['admin-words'],
    queryFn: fetchSensitiveWords,
    enabled: user?.role === 'ADMIN',
  });
  const [word, setWord] = useState('');
  const [strategy, setStrategy] = useState<WordStrategy>('REJECT');

  // 直播 / 视频 / 用户
  const [roomId, setRoomId] = useState('');
  const [videoId, setVideoId] = useState('');
  const [userId, setUserId] = useState('');
  const [foundUser, setFoundUser] = useState<AdminUser | null>(null);

  if (user?.role !== 'ADMIN') {
    return <EmptyState title="需要管理员权限" hint="当前账号不是 ADMIN，无法访问管理后台" />;
  }

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await action();
    } catch (actionError) {
      setError(describeError(actionError));
    } finally {
      setBusy(false);
    }
  }

  const tabs: Array<{ key: Tab; label: string }> = [
    { key: 'categories', label: '分类' },
    { key: 'words', label: '敏感词' },
    { key: 'live', label: '强制关播' },
    { key: 'users', label: '用户查询' },
    { key: 'videos', label: '视频下架' },
  ];

  return (
    <div className="space-y-4">
      <h1 className="text-h1 font-semibold text-ink">管理后台</h1>

      <div className="flex flex-wrap gap-1 border-b border-line">
        {tabs.map((item) => (
          <button
            key={item.key}
            type="button"
            onClick={() => setTab(item.key)}
            className={`-mb-px border-b-2 px-3 py-2 text-sm ${
              tab === item.key
                ? 'border-brand text-brand'
                : 'border-transparent text-ink-secondary hover:text-ink'
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>

      {error && <p className="text-meta text-brand">{error}</p>}
      {notice && <p className="text-meta text-ink-tertiary">{notice}</p>}

      {tab === 'categories' && (
        <section className="space-y-3 ui-card p-4">
          <h2 className="text-sm font-medium text-ink">分类列表</h2>
          <ul className="space-y-1 text-sm text-ink-secondary">
            {(categoriesQuery.data ?? []).map((category) => (
              <li key={category.id}>
                #{category.id} {category.name}
              </li>
            ))}
          </ul>
          <div className="flex flex-wrap gap-2">
            <input
              value={categoryName}
              onChange={(event) => setCategoryName(event.target.value)}
              placeholder="新分类名称"
              className="ui-input min-w-0 flex-1"
            />
            <input
              value={categorySort}
              onChange={(event) => setCategorySort(event.target.value)}
              placeholder="排序"
              className="h-9 w-20 rounded-md border border-line bg-page px-3 text-sm text-ink"
            />
            <button
              type="button"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await createCategory(categoryName.trim(), Number(categorySort) || 0);
                  setCategoryName('');
                  setNotice('分类已创建');
                  await queryClient.invalidateQueries({ queryKey: ['categories'] });
                })
              }
              className="h-9 ui-btn-primary disabled:opacity-60"
            >
              创建分类
            </button>
          </div>
        </section>
      )}

      {tab === 'words' && (
        <section className="space-y-3 ui-card p-4">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-medium text-ink">敏感词（{wordsQuery.data?.length ?? 0}）</h2>
            <button
              type="button"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await reloadSensitiveWords();
                  setNotice('词库已重载');
                  await queryClient.invalidateQueries({ queryKey: ['admin-words'] });
                })
              }
              className="ui-btn-ghost h-8 px-3 text-meta text-ink-secondary"
            >
              重载词库
            </button>
          </div>

          <ul className="space-y-1 text-sm text-ink-secondary">
            {(wordsQuery.data ?? []).map((item) => (
              <li key={item.id}>
                {item.word} · {item.strategy} · {item.enabled ? '启用' : '停用'}
              </li>
            ))}
          </ul>

          <div className="flex flex-wrap gap-2">
            <input
              value={word}
              onChange={(event) => setWord(event.target.value)}
              placeholder="敏感词"
              className="ui-input min-w-0 flex-1"
            />
            <select
              value={strategy}
              onChange={(event) => setStrategy(event.target.value as WordStrategy)}
              className="ui-input"
            >
              <option value="REJECT">REJECT（拒绝）</option>
              <option value="REPLACE">REPLACE（替换）</option>
              <option value="LOG">LOG（仅记录）</option>
            </select>
            <button
              type="button"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await createSensitiveWord({ word: word.trim(), strategy, enabled: true });
                  await reloadSensitiveWords();
                  setWord('');
                  setNotice('敏感词已添加');
                  await queryClient.invalidateQueries({ queryKey: ['admin-words'] });
                })
              }
              className="h-9 ui-btn-primary disabled:opacity-60"
            >
              添加
            </button>
          </div>
        </section>
      )}

      {tab === 'live' && (
        <AdminAction
          title="强制关播"
          placeholder="直播间 ID（数字）"
          value={roomId}
          onChange={setRoomId}
          button="强制关播"
          busy={busy}
          onRun={() =>
            void run(async () => {
              await forceCloseRoom(roomId.trim());
              setNotice(`直播间 ${roomId} 已强制关播`);
            })
          }
        />
      )}

      {tab === 'videos' && (
        <AdminAction
          title="视频下架"
          placeholder="视频 ID（数字）"
          value={videoId}
          onChange={setVideoId}
          button="下架视频"
          busy={busy}
          onRun={() =>
            void run(async () => {
              await offlineVideo(videoId.trim());
              setNotice(`视频 ${videoId} 已下架`);
            })
          }
        />
      )}

      {tab === 'users' && (
        <section className="space-y-3 ui-card p-4">
          <h2 className="text-sm font-medium text-ink">按内部 userId 查询</h2>
          <div className="flex gap-2">
            <input
              value={userId}
              onChange={(event) => setUserId(event.target.value)}
              placeholder="userId（数字主键，不是 uid）"
              className="ui-input min-w-0 flex-1"
            />
            <button
              type="button"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  setFoundUser(await lookupUser(userId.trim()));
                })
              }
              className="h-9 ui-btn-primary disabled:opacity-60"
            >
              查询
            </button>
          </div>
          {foundUser && (
            <pre className="overflow-auto rounded-md bg-page p-3 text-meta text-ink-secondary">
              {JSON.stringify(foundUser, null, 2)}
            </pre>
          )}
        </section>
      )}
    </div>
  );
}

interface AdminActionProps {
  title: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
  button: string;
  busy: boolean;
  onRun: () => void;
}

function AdminAction({ title, placeholder, value, onChange, button, busy, onRun }: AdminActionProps) {
  return (
    <section className="space-y-3 ui-card p-4">
      <h2 className="text-sm font-medium text-ink">{title}</h2>
      <div className="flex gap-2">
        <input
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder={placeholder}
          className="ui-input min-w-0 flex-1"
        />
        <button
          type="button"
          disabled={busy}
          onClick={onRun}
          className="h-9 ui-btn-primary disabled:opacity-60"
        >
          {button}
        </button>
      </div>
    </section>
  );
}
