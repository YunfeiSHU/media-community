import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useCategories } from '@/entities/category/queries';
import { deleteVideo, offlineVideo, updateVideo, type UpdateVideoInput } from '@/entities/video/api';
import { useVideos } from '@/entities/video/queries';
import type { Video } from '@/entities/video/types';
import { uploadCover } from '@/entities/upload/api';
import { describeError } from '@/shared/api/errors';
import { formatCount, formatDate } from '@/shared/lib/datetime';
import { textOf, VIDEO_STATUS_TEXT } from '@/shared/lib/status';
import { CardGridSkeleton, EmptyState } from '@/widgets/empty-state/EmptyState';

/** 我的投稿管理：编辑基本信息、下架、删除。 */
export default function VideoManagePage() {
  const queryClient = useQueryClient();
  const categoriesQuery = useCategories();
  const videosQuery = useVideos({ mine: true, pageSize: 50 });
  const videos = videosQuery.data?.items ?? [];

  const [editing, setEditing] = useState<Video | null>(null);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [tags, setTags] = useState('');
  const [coverUrl, setCoverUrl] = useState('');
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  function beginEdit(video: Video) {
    setEditing(video);
    setTitle(video.title);
    setDescription(video.description);
    setCategoryId(video.categoryId ? String(video.categoryId) : '');
    setTags(video.tags.join(', '));
    setCoverUrl(video.coverUrl);
    setError(null);
    setNotice(null);
  }

  async function save() {
    if (!editing) return;
    if (!title.trim()) {
      setError('标题不能为空');
      return;
    }
    setBusy('save');
    setError(null);
    try {
      const input: UpdateVideoInput = {
        title: title.trim(),
        description: description.trim(),
        categoryId: categoryId ? Number(categoryId) : null,
        tags: tags
          .split(/[,，\s]+/)
          .map((tag) => tag.trim())
          .filter(Boolean),
        coverUrl,
      };
      await updateVideo(editing.id, input);
      setEditing(null);
      setNotice('已保存');
      await queryClient.invalidateQueries({ queryKey: ['videos'] });
      await queryClient.invalidateQueries({ queryKey: ['video', editing.id] });
    } catch (saveError) {
      setError(describeError(saveError));
    } finally {
      setBusy(null);
    }
  }

  async function handleOffline(video: Video) {
    setBusy(`offline:${video.id}`);
    setError(null);
    try {
      await offlineVideo(video.id);
      setNotice('已下架');
      await queryClient.invalidateQueries({ queryKey: ['videos'] });
    } catch (offlineError) {
      setError(describeError(offlineError));
    } finally {
      setBusy(null);
    }
  }

  async function handleDelete(video: Video) {
    if (!window.confirm(`确定删除「${video.title}」？该操作不可恢复。`)) return;
    setBusy(`delete:${video.id}`);
    setError(null);
    try {
      await deleteVideo(video.id);
      setNotice('已删除');
      await queryClient.invalidateQueries({ queryKey: ['videos'] });
    } catch (deleteError) {
      setError(describeError(deleteError));
    } finally {
      setBusy(null);
    }
  }

  async function handleCover(file: File | null) {
    if (!file) return;
    setBusy('cover');
    setError(null);
    try {
      const image = await uploadCover(file);
      setCoverUrl(image.url);
    } catch (uploadError) {
      setError(describeError(uploadError));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-h1 font-semibold text-ink">视频管理</h1>
        <div className="flex gap-2 text-meta">
          <Link to="/studio/products" className="ui-btn-ghost h-8 px-3 text-meta text-ink-secondary hover:text-brand">
            商品管理
          </Link>
          <Link to="/upload" className="rounded-md bg-brand px-3 py-1.5 text-white">
            去投稿
          </Link>
        </div>
      </div>

      {error && <p className="text-meta text-brand">{error}</p>}
      {notice && <p className="text-meta text-ink-tertiary">{notice}</p>}

      {videosQuery.isPending && <CardGridSkeleton count={4} />}
      {videosQuery.isError && <EmptyState title="投稿加载失败" hint="请确认视频服务已启动" />}
      {videosQuery.isSuccess && videos.length === 0 && (
        <EmptyState title="还没有投稿" hint="去投稿页上传第一个视频" />
      )}

      {videos.length > 0 && (
        <ul className="ui-card divide-y divide-line overflow-hidden">
          {videos.map((video) => (
            <li key={video.id} className="space-y-2 px-4 py-3">
              <div className="flex flex-wrap items-center gap-3">
                <span className="rounded-sm bg-page px-1.5 py-0.5 text-meta text-ink-secondary">
                  {textOf(VIDEO_STATUS_TEXT, video.status)}
                </span>
                <Link
                  to={`/video/${video.id}`}
                  className="min-w-0 flex-1 truncate text-sm text-ink hover:text-brand"
                >
                  {video.title}
                </Link>
                <span className="text-meta text-ink-tertiary">
                  {formatCount(video.viewCount)} 播放 · {formatDate(video.createdAt)}
                </span>
                <div className="flex gap-2 text-meta">
                  <button
                    type="button"
                    onClick={() => beginEdit(video)}
                    className="ui-btn-ghost h-8 px-3 text-meta text-ink-secondary hover:text-brand"
                  >
                    编辑
                  </button>
                  {video.status === 'PUBLISHED' && (
                    <button
                      type="button"
                      onClick={() => void handleOffline(video)}
                      disabled={busy === `offline:${video.id}`}
                      className="ui-btn-ghost h-8 px-3 text-meta text-ink-secondary disabled:opacity-60"
                    >
                      下架
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => void handleDelete(video)}
                    disabled={busy === `delete:${video.id}`}
                    className="rounded-full border border-danger/30 bg-card px-3 py-1 text-meta text-danger disabled:opacity-60"
                  >
                    删除
                  </button>
                </div>
              </div>

              {editing?.id === video.id && (
                <div className="space-y-2 rounded-md bg-page p-3">
                  <input
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                    maxLength={100}
                    placeholder="标题"
                    className="h-9 w-full rounded-md border border-line bg-card px-3 text-sm text-ink"
                  />
                  <textarea
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                    rows={3}
                    placeholder="简介"
                    className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm text-ink"
                  />
                  <div className="flex flex-wrap gap-2">
                    <select
                      value={categoryId}
                      onChange={(event) => setCategoryId(event.target.value)}
                      className="h-9 rounded-md border border-line bg-card px-3 text-sm text-ink"
                    >
                      <option value="">未分类</option>
                      {(categoriesQuery.data ?? []).map((category) => (
                        <option key={category.id} value={category.id}>
                          {category.name}
                        </option>
                      ))}
                    </select>
                    <input
                      value={tags}
                      onChange={(event) => setTags(event.target.value)}
                      placeholder="标签，用逗号分隔"
                      className="h-9 min-w-0 flex-1 rounded-md border border-line bg-card px-3 text-sm text-ink"
                    />
                    <label className="grid h-9 cursor-pointer place-items-center rounded-full border border-line bg-card px-3 text-meta text-ink-secondary">
                      {busy === 'cover' ? '上传中…' : coverUrl ? '更换封面' : '上传封面'}
                      <input
                        type="file"
                        accept="image/*"
                        className="hidden"
                        onChange={(event) => void handleCover(event.target.files?.[0] ?? null)}
                      />
                    </label>
                  </div>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => void save()}
                      disabled={busy === 'save'}
                      className="h-8 rounded-md bg-brand px-3 text-meta text-white disabled:opacity-60"
                    >
                      {busy === 'save' ? '保存中…' : '保存'}
                    </button>
                    <button
                      type="button"
                      onClick={() => setEditing(null)}
                      className="h-8 rounded-full border border-line bg-card px-3 text-meta text-ink-secondary"
                    >
                      取消
                    </button>
                  </div>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
