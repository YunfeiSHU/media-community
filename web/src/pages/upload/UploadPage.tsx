import { useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useCategories } from '@/entities/category/queries';
import { uploadCover } from '@/entities/upload/api';
import { useVideoUpload } from '@/features/video-upload/useVideoUpload';
import { describeError } from '@/shared/api/errors';

const MAX_SIZE = 2 * 1024 * 1024 * 1024; // 2GB，与后端 upload.max_size_mb 对齐
const ALLOWED_EXT = ['mp4', 'mov', 'mkv', 'avi', 'flv', 'webm'];

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

export default function UploadPage() {
  const categoriesQuery = useCategories();
  const { state, persisted, start, resume, cancel, publish, retry, reset } = useVideoUpload();

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [categoryId, setCategoryId] = useState<number | null>(null);
  const [tags, setTags] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [coverUrl, setCoverUrl] = useState('');
  const [coverUploading, setCoverUploading] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const resumeInputRef = useRef<HTMLInputElement>(null);

  const busy =
    state.step === 'creating' ||
    state.step === 'uploading' ||
    state.step === 'transcoding' ||
    state.step === 'publishing';

  const tagList = useMemo(
    () =>
      tags
        .split(/[,，\s]+/)
        .map((tag) => tag.trim())
        .filter(Boolean)
        .slice(0, 10),
    [tags],
  );

  function pickFile(selected: File | null) {
    setFormError(null);
    if (!selected) {
      setFile(null);
      return;
    }
    const ext = selected.name.split('.').pop()?.toLowerCase() ?? '';
    if (!ALLOWED_EXT.includes(ext)) {
      setFormError(`只支持 ${ALLOWED_EXT.join(' / ')}`);
      return;
    }
    if (selected.size > MAX_SIZE) {
      setFormError('文件超过 2GB，前端先拦掉，避免传完才被后端拒绝');
      return;
    }
    setFile(selected);
  }

  async function pickCover(selected: File | null) {
    if (!selected) return;
    if (selected.size > 5 * 1024 * 1024) {
      setFormError('封面不能超过 5MB');
      return;
    }
    setCoverUploading(true);
    setFormError(null);
    try {
      const image = await uploadCover(selected);
      setCoverUrl(image.url);
    } catch (error) {
      setFormError(describeError(error));
    } finally {
      setCoverUploading(false);
    }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setFormError(null);
    if (!file) {
      setFormError('请先选择视频文件');
      return;
    }
    if (!title.trim()) {
      setFormError('标题不能为空');
      return;
    }
    await start(
      {
        title: title.trim(),
        description: description.trim(),
        categoryId,
        tags: tagList,
        coverUrl,
      },
      file,
    );
  }

  // ---------------------------------------------------------------- 进行中
  if (busy || state.step === 'ready' || state.step === 'failed' || state.step === 'done' || state.step === 'cancelled') {
    const percent = state.progress.percent;
    const speedMb = state.progress.speedBps / 1024 / 1024;

    return (
      <div className="mx-auto max-w-2xl space-y-5">
        <h1 className="text-lg font-semibold text-ink">投稿</h1>

        <section className="rounded-md bg-card p-5">
          {state.step === 'creating' && <p className="text-sm text-ink-secondary">正在创建草稿…</p>}

          {(state.step === 'uploading' || state.step === 'creating') && (
            <div className="space-y-3">
              <div className="flex justify-between text-sm text-ink-secondary">
                <span>上传中</span>
                <span>{percent}%</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-page">
                <div className="h-full bg-brand transition-all" style={{ width: `${percent}%` }} />
              </div>
              <p className="text-xs text-ink-tertiary">
                {formatBytes(state.progress.uploadedBytes)} / {formatBytes(state.progress.totalBytes)} ·{' '}
                {state.progress.partsDone}/{state.progress.partsTotal} 片 ·{' '}
                {speedMb > 0 ? `${speedMb.toFixed(2)} MB/s` : '计算速度中…'}
              </p>
              <p className="text-xs text-ink-tertiary">
                并发 3 片、失败自动重试 3 次；离开页面前请确认已传完。
              </p>
            </div>
          )}

          {state.step === 'transcoding' && (
            <div className="space-y-3">
              <div className="flex justify-between text-sm text-ink-secondary">
                <span>转码中（mock 驱动只会生成占位产物）</span>
                <span>{state.transcode?.progress ?? 0}%</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-page">
                <div
                  className="h-full bg-brand transition-all"
                  style={{ width: `${state.transcode?.progress ?? 0}%` }}
                />
              </div>
              <p className="text-xs text-ink-tertiary">
                {state.transcode?.message || '正在切片与生成 HLS，页面每 2 秒刷新一次'}
              </p>
            </div>
          )}

          {state.step === 'ready' && (
            <div className="space-y-3">
              <p className="text-sm text-success">转码完成，可以发布了</p>
              <div className="flex items-center gap-3">
                {coverUrl ? (
                  <img src={coverUrl} alt="封面" className="h-16 w-28 rounded-sm object-cover" />
                ) : (
                  <div className="grid h-16 w-28 place-items-center rounded-sm bg-page text-xs text-ink-tertiary">
                    无封面
                  </div>
                )}
                <p className="text-xs text-ink-tertiary">
                  发布后视频会出现在首页；没有封面时列表用标题首字占位。
                </p>
              </div>
              <button
                type="button"
                onClick={() => void publish()}
                className="h-10 rounded-md bg-brand px-5 text-sm text-white"
              >
                发布
              </button>
            </div>
          )}

          {state.step === 'publishing' && <p className="text-sm text-ink-secondary">正在发布…</p>}

          {state.step === 'done' && (
            <div className="space-y-3">
              <p className="text-sm text-success">发布成功</p>
              <div className="flex gap-3">
                <Link
                  to={`/video/${state.videoId}`}
                  className="rounded-md bg-brand px-4 py-2 text-sm text-white"
                >
                  去看视频
                </Link>
                <Link to="/space" className="rounded-md border border-line px-4 py-2 text-sm text-ink-secondary">
                  我的投稿
                </Link>
              </div>
            </div>
          )}

          {state.step === 'failed' && (
            <div className="space-y-3">
              <p className="text-sm text-danger">{state.error || '上传失败'}</p>
              <div className="flex gap-3">
                <button
                  type="button"
                  onClick={() => void retry()}
                  className="rounded-md bg-brand px-4 py-2 text-sm text-white"
                >
                  重试转码
                </button>
                <button
                  type="button"
                  onClick={reset}
                  className="rounded-md border border-line px-4 py-2 text-sm text-ink-secondary"
                >
                  重新开始
                </button>
              </div>
            </div>
          )}

          {state.step === 'cancelled' && (
            <div className="space-y-3">
              <p className="text-sm text-ink-secondary">已取消上传</p>
              <button
                type="button"
                onClick={reset}
                className="rounded-md bg-brand px-4 py-2 text-sm text-white"
              >
                重新投稿
              </button>
            </div>
          )}

          {(state.step === 'uploading' || state.step === 'transcoding') && (
            <button
              type="button"
              onClick={() => void cancel()}
              className="mt-4 text-xs text-ink-tertiary hover:text-danger"
            >
              取消上传
            </button>
          )}
        </section>

        {state.step === 'ready' && state.transcode ? (
          <p className="text-xs text-ink-tertiary">
            转码档位：{state.transcode.profiles.join(' / ') || '默认'}
          </p>
        ) : null}
      </div>
    );
  }

  // ---------------------------------------------------------------- 表单
  return (
    <div className="mx-auto max-w-2xl space-y-5">
      <h1 className="text-h1 font-semibold text-ink">投稿</h1>

      {persisted && (
        <section className="ui-card p-4">
          <p className="text-body text-ink">
            检测到未完成的上传：<span className="font-mono text-meta">{persisted.fileName}</span>
          </p>
          <p className="mt-1 text-meta text-ink-tertiary">
            浏览器拿不到上次选择的文件，请重新选择**同一个文件**后继续，已传分片会自动跳过。
          </p>
          <div className="mt-3 flex items-center gap-3">
            <input
              ref={resumeInputRef}
              type="file"
              className="hidden"
              accept=".mp4,.mov,.mkv,.avi,.flv,.webm"
              onChange={(event) => {
                const selected = event.target.files?.[0];
                if (selected) void resume(selected);
              }}
            />
            <button
              type="button"
              onClick={() => resumeInputRef.current?.click()}
              className="rounded-md bg-brand px-4 py-2 text-sm text-white"
            >
              选择同一文件继续
            </button>
            <button
              type="button"
              onClick={reset}
              className="rounded-md border border-line px-4 py-2 text-sm text-ink-secondary"
            >
              放弃
            </button>
          </div>
        </section>
      )}

      <form onSubmit={submit} className="ui-card space-y-4 p-5">
        <label className="block">
          <span className="mb-1 block text-meta text-ink-secondary">标题</span>
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            maxLength={128}
            placeholder="给视频起个标题"
            className="ui-input h-10"
          />
        </label>

        <label className="block">
          <span className="mb-1 block text-meta text-ink-secondary">简介</span>
          <textarea
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            maxLength={1024}
            rows={4}
            placeholder="介绍一下这个视频"
            className="w-full rounded-sm border border-line bg-subtle p-3 text-sm text-ink placeholder:text-ink-tertiary focus:border-brand focus:bg-card focus:outline-none"
          />
        </label>

        <div className="grid gap-4 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-meta text-ink-secondary">分区</span>
            <select
              value={categoryId ?? ''}
              onChange={(event) =>
                setCategoryId(event.target.value ? Number(event.target.value) : null)
              }
              className="ui-input h-10"
            >
              <option value="">不选择</option>
              {(categoriesQuery.data ?? []).map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </select>
          </label>

          <label className="block">
            <span className="mb-1 block text-meta text-ink-secondary">标签（逗号分隔，最多 10 个）</span>
            <input
              value={tags}
              onChange={(event) => setTags(event.target.value)}
              placeholder="技术, 演示"
              className="ui-input h-10"
            />
          </label>
        </div>

        <div>
          <span className="mb-1 block text-meta text-ink-secondary">视频文件</span>
          <input
            type="file"
            accept=".mp4,.mov,.mkv,.avi,.flv,.webm"
            onChange={(event) => pickFile(event.target.files?.[0] ?? null)}
            className="block w-full text-sm text-ink-secondary file:mr-3 file:rounded-full file:border-0 file:bg-brand file:px-4 file:py-2 file:text-sm file:text-white"
          />
          {file && (
            <p className="mt-1 text-meta text-ink-tertiary">
              {file.name} · {formatBytes(file.size)}
            </p>
          )}
        </div>

        <div>
          <span className="mb-1 block text-meta text-ink-secondary">封面（可选，16:9 更合适）</span>
          <div className="flex items-center gap-3">
            {coverUrl ? (
              <img src={coverUrl} alt="封面预览" className="h-16 w-28 rounded-sm object-cover" />
            ) : (
              <div className="grid h-16 w-28 place-items-center rounded-sm bg-subtle text-meta text-ink-tertiary">
                未上传
              </div>
            )}
            <input
              type="file"
              accept="image/jpeg,image/png,image/webp"
              onChange={(event) => void pickCover(event.target.files?.[0] ?? null)}
              className="text-sm text-ink-secondary"
            />
          </div>
          {coverUploading && <p className="mt-1 text-meta text-ink-tertiary">封面上传中…</p>}
        </div>

        {formError && <p className="text-meta text-danger">{formError}</p>}

        <button
          type="submit"
          disabled={!file || !title.trim()}
          className="ui-btn-primary h-10 w-full text-sm"
        >
          开始上传
        </button>
      </form>
    </div>
  );
}
