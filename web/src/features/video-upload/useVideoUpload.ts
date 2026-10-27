import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError, describeError } from '@/shared/api/errors';
import {
  abortUpload,
  completeUpload,
  createVideoDraft,
  fetchTranscode,
  fetchUploadStatus,
  initUpload,
  publishVideo,
  retryTranscode,
  uploadPart,
  type CreateVideoInput,
} from '@/entities/upload/api';
import type { PersistedUpload, TranscodeInfo, UploadSession } from '@/entities/upload/types';

const STORAGE_KEY = 'mp.upload.session';
const CONCURRENCY = 3;
const MAX_PART_RETRY = 3;

export type UploadStep =
  | 'idle'
  | 'creating'
  | 'uploading'
  | 'transcoding'
  | 'ready'
  | 'publishing'
  | 'done'
  | 'failed'
  | 'cancelled';

export interface UploadProgress {
  uploadedBytes: number;
  totalBytes: number;
  percent: number;
  speedBps: number;
  partsDone: number;
  partsTotal: number;
}

export interface UploadState {
  step: UploadStep;
  videoId: number | null;
  session: UploadSession | null;
  transcode: TranscodeInfo | null;
  progress: UploadProgress;
  error: string | null;
}

const EMPTY_PROGRESS: UploadProgress = {
  uploadedBytes: 0,
  totalBytes: 0,
  percent: 0,
  speedBps: 0,
  partsDone: 0,
  partsTotal: 0,
};

const initialState: UploadState = {
  step: 'idle',
  videoId: null,
  session: null,
  transcode: null,
  progress: EMPTY_PROGRESS,
  error: null,
};

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

/** 上传会话过期：需要重新 init 再继续。 */
class UploadExpiredError extends Error {}

function readPersisted(): PersistedUpload | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as PersistedUpload) : null;
  } catch {
    return null;
  }
}

export function useVideoUpload() {
  const [state, setState] = useState<UploadState>(initialState);
  const [persisted, setPersisted] = useState<PersistedUpload | null>(() => readPersisted());

  const cancelledRef = useRef(false);
  const timerRef = useRef<number | null>(null);

  // 上传/转码途中离开页面要拦一下，避免用户以为已经传完
  useEffect(() => {
    if (state.step !== 'uploading' && state.step !== 'transcoding') return;

    function onBeforeUnload(event: BeforeUnloadEvent) {
      event.preventDefault();
      event.returnValue = '';
    }
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [state.step]);

  useEffect(() => {
    return () => {
      cancelledRef.current = true;
      if (timerRef.current) window.clearTimeout(timerRef.current);
    };
  }, []);

  const reset = useCallback(() => {
    cancelledRef.current = false;
    setState(initialState);
  }, []);

  const pollTranscode = useCallback(async (videoId: number) => {
    // 2s 一次，最多 5 分钟
    for (let attempt = 0; attempt < 150; attempt += 1) {
      if (cancelledRef.current) throw new Error('cancelled');

      const info = await fetchTranscode(videoId);
      setState((prev) => ({ ...prev, step: 'transcoding', transcode: info }));

      if (info.status === 'SUCCESS') {
        setState((prev) => ({ ...prev, step: 'ready', transcode: info }));
        return;
      }
      if (info.status === 'FAILED') {
        setState((prev) => ({
          ...prev,
          step: 'failed',
          transcode: info,
          error: info.message || '转码失败',
        }));
        return;
      }
      await sleep(2000);
    }
    throw new Error('转码超时，请稍后在「我的投稿」查看');
  }, []);

  const uploadAllParts = useCallback(
    async (videoId: number, file: File, session: UploadSession) => {
      const { partSize, totalParts } = session;
      const done = new Set(session.uploadedParts.map((part) => part.partNumber));

      let uploadedBytes = session.uploadedParts.reduce((sum, part) => sum + part.size, 0);
      let doneParts = done.size;
      const startedAt = Date.now();

      const pending: number[] = [];
      for (let partNumber = 1; partNumber <= totalParts; partNumber += 1) {
        if (!done.has(partNumber)) pending.push(partNumber);
      }

      const publishProgress = () => {
        const elapsed = Math.max(1, (Date.now() - startedAt) / 1000);
        const speed = (uploadedBytes - (session.uploadedBytes || 0)) / elapsed;
        setState((prev) => ({
          ...prev,
          step: 'uploading',
          progress: {
            uploadedBytes,
            totalBytes: file.size,
            percent: Math.min(100, Math.round((uploadedBytes / file.size) * 100)),
            speedBps: Number.isFinite(speed) ? Math.max(0, speed) : 0,
            partsDone: doneParts,
            partsTotal: totalParts,
          },
        }));
      };
      publishProgress();

      let cursor = 0;

      async function worker() {
        while (true) {
          if (cancelledRef.current) throw new Error('cancelled');
          const index = cursor;
          cursor += 1;
          if (index >= pending.length) return;

          const partNumber = pending[index];
          const start = (partNumber - 1) * partSize;
          const end = Math.min(start + partSize, file.size);
          const chunk = file.slice(start, end);

          let attempt = 0;
          while (true) {
            try {
              await uploadPart(videoId, partNumber, chunk);
              break;
            } catch (error) {
              if ((error as ApiError)?.code === 40404) throw new UploadExpiredError();
              attempt += 1;
              if (attempt >= MAX_PART_RETRY) throw error;
              await sleep(2 ** (attempt - 1) * 1000);
            }
          }

          uploadedBytes += chunk.size;
          doneParts += 1;
          publishProgress();
        }
      }

      await Promise.all(Array.from({ length: Math.min(CONCURRENCY, pending.length) }, () => worker()));
    },
    [],
  );

  const runFromInit = useCallback(
    async (videoId: number, file: File, session?: UploadSession) => {
      let current = session ?? (await initUpload(videoId, file.name, file.size));
      setState((prev) => ({
        ...prev,
        step: 'uploading',
        videoId,
        session: current,
        progress: {
          uploadedBytes: current.uploadedBytes,
          totalBytes: file.size,
          percent: Math.min(100, Math.round((current.uploadedBytes / file.size) * 100)),
          speedBps: 0,
          partsDone: current.uploadedCount,
          partsTotal: current.totalParts,
        },
      }));

      try {
        await uploadAllParts(videoId, file, current);
      } catch (error) {
        if (error instanceof UploadExpiredError) {
          // 会话失效：重新 init 后从头传（已传分片会在新的会话里重新计数）
          current = await initUpload(videoId, file.name, file.size);
          await uploadAllParts(videoId, file, current);
        } else {
          throw error;
        }
      }

      current = await completeUpload(videoId);
      setState((prev) => ({ ...prev, session: current, step: 'transcoding' }));
      localStorage.removeItem(STORAGE_KEY);
      setPersisted(null);
      await pollTranscode(videoId);
    },
    [pollTranscode, uploadAllParts],
  );

  /** 新建投稿：创建草稿 → init → 上传 → complete → 轮询转码 */
  const start = useCallback(
    async (input: CreateVideoInput, file: File) => {
      cancelledRef.current = false;
      setState({ ...initialState, step: 'creating' });

      try {
        const draft = await createVideoDraft(input);
        const record: PersistedUpload = {
          videoId: draft.videoId,
          uploadId: '',
          fileName: file.name,
          fileSize: file.size,
        };
        localStorage.setItem(STORAGE_KEY, JSON.stringify(record));
        setPersisted(record);

        await runFromInit(draft.videoId, file);
      } catch (error) {
        if ((error as Error)?.message === 'cancelled') {
          setState((prev) => ({ ...prev, step: 'cancelled' }));
          return;
        }
        setState((prev) => ({ ...prev, step: 'failed', error: describeError(error) }));
      }
    },
    [runFromInit],
  );

  /** 刷新/断网后恢复：重新选同一个文件，用服务端 ListParts 作为唯一权威继续传 */
  const resume = useCallback(
    async (file: File) => {
      const record = readPersisted();
      if (!record) return;

      cancelledRef.current = false;
      setState({ ...initialState, step: 'uploading', videoId: record.videoId });

      try {
        if (record.fileSize !== file.size) {
          throw new Error('文件大小与上次不一致，请选择同一个文件');
        }
        const session = await fetchUploadStatus(record.videoId);
        if (session.completed) {
          setState((prev) => ({ ...prev, step: 'transcoding', session }));
          await pollTranscode(record.videoId);
          return;
        }
        if (file.size !== session.fileSize) {
          throw new Error('文件大小与上传会话不一致，请选择原来的文件');
        }
        await runFromInit(record.videoId, file, session);
      } catch (error) {
        setState((prev) => ({ ...prev, step: 'failed', error: describeError(error) }));
      }
    },
    [pollTranscode, runFromInit],
  );

  const cancel = useCallback(async () => {
    cancelledRef.current = true;
    const videoId = state.videoId;
    localStorage.removeItem(STORAGE_KEY);
    setPersisted(null);
    setState((prev) => ({ ...prev, step: 'cancelled' }));
    if (videoId) {
      try {
        await abortUpload(videoId);
      } catch {
        // 取消失败不阻塞 UI：服务端会话会自然过期
      }
    }
  }, [state.videoId]);

  const publish = useCallback(async () => {
    if (!state.videoId) return;
    setState((prev) => ({ ...prev, step: 'publishing' }));
    try {
      await publishVideo(state.videoId);
      setState((prev) => ({ ...prev, step: 'done' }));
    } catch (error) {
      setState((prev) => ({ ...prev, step: 'ready', error: describeError(error) }));
    }
  }, [state.videoId]);

  const retry = useCallback(async () => {
    if (!state.videoId) return;
    try {
      await retryTranscode(state.videoId);
      await pollTranscode(state.videoId);
    } catch (error) {
      setState((prev) => ({ ...prev, step: 'failed', error: describeError(error) }));
    }
  }, [pollTranscode, state.videoId]);

  return {
    state,
    persisted,
    start,
    resume,
    cancel,
    publish,
    retry,
    reset,
  };
}
