import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchRoomHistory, fetchVideoHistory, type DanmakuItem } from '@/entities/danmaku/api';
import { useAuthStore } from '@/features/auth/store';
import { DanmakuSocket, type DanmakuFrame, type SocketStatus } from '@/shared/ws/DanmakuSocket';

const MAX_ITEMS = 200;

export interface DanmakuError {
  code: number;
  message: string;
  retryAfterMs?: number;
}

/** 弹幕目标：直播房间与点播视频二选一。 */
export interface DanmakuTarget {
  roomId?: string;
  videoId?: string;
}

export interface SendOptions {
  timeMs?: number;
  color?: string;
  position?: 'SCROLL' | 'TOP' | 'BOTTOM';
}

function frameToItem(frame: DanmakuFrame): DanmakuItem | null {
  const data = frame.data as Record<string, unknown>;
  const content = typeof data.content === 'string' ? data.content : '';
  if (!content) return null;
  return {
    danmakuId: Number(data.danmakuId ?? 0),
    clientMsgId: String(data.clientMsgId ?? ''),
    content,
    uid: String(data.uid ?? ''),
    nickname: String(data.nickname ?? ''),
    color: typeof data.color === 'string' ? data.color : '#ffffff',
    position: (data.position as DanmakuItem['position']) ?? 'SCROLL',
    timeMs: Number(data.timeMs ?? 0),
    sentAt: null,
  };
}

/**
 * 弹幕连接：先拉历史，再连 WS；断线重连由 DanmakuSocket 负责。
 *
 * 直播和点播共用同一套协议，区别只有：
 * - 历史接口：/rooms/:id/history vs /videos/:id/history
 * - 发送时点播要带 timeMs（对齐播放进度）
 */
export function useDanmaku(target: DanmakuTarget, enabled = true) {
  const user = useAuthStore((state) => state.user);
  const [items, setItems] = useState<DanmakuItem[]>([]);
  const [status, setStatus] = useState<SocketStatus>('closed');
  const [detail, setDetail] = useState<string | null>(null);
  const [error, setError] = useState<DanmakuError | null>(null);
  const [systemMessage, setSystemMessage] = useState<string | null>(null);

  const socketRef = useRef<DanmakuSocket | null>(null);
  const roomId = target.roomId ?? '';
  const videoId = target.videoId ?? '';
  const targetKey = roomId || videoId;

  useEffect(() => {
    if (!targetKey || !enabled) return;

    let disposed = false;
    setItems([]);
    setError(null);

    // 历史先到，避免进房/进播放页后空白几秒
    const historyPromise = roomId
      ? fetchRoomHistory(roomId, 50)
      : fetchVideoHistory(videoId, 200);
    void historyPromise
      .then((history) => {
        if (!disposed) setItems(history.slice(-MAX_ITEMS));
      })
      .catch(() => {
        // 历史失败不影响实时弹幕
      });

    const socket = new DanmakuSocket(
      {
        roomId: roomId || undefined,
        videoId: videoId || undefined,
        uid: user?.uid,
        nickname: user?.nickname,
      },
      (frame) => {
        if (frame.type === 'danmaku') {
          const item = frameToItem(frame);
          if (!item) return;
          setItems((prev) => [...prev, item].slice(-MAX_ITEMS));
          return;
        }
        if (frame.type === 'system') {
          const message = String((frame.data as Record<string, unknown>).message ?? '');
          if (message) setSystemMessage(message);
          return;
        }
        if (frame.type === 'error') {
          const data = frame.data as Record<string, unknown>;
          setError({
            code: Number(data.code ?? 0),
            message: String(data.message ?? '发送失败'),
            retryAfterMs: data.retryAfterMs ? Number(data.retryAfterMs) : undefined,
          });
        }
      },
      (next, reason) => {
        setStatus(next);
        setDetail(reason ?? null);
      },
    );

    socketRef.current = socket;
    socket.connect();

    return () => {
      disposed = true;
      socket.close();
      socketRef.current = null;
    };
  }, [roomId, videoId, targetKey, enabled, user?.uid, user?.nickname]);

  const send = useCallback((content: string, options: SendOptions = {}) => {
    const trimmed = content.trim();
    if (!trimmed) return;
    setError(null);
    try {
      socketRef.current?.send(
        trimmed,
        options.color ?? '#FFFFFF',
        options.position ?? 'SCROLL',
        Math.max(0, Math.round(options.timeMs ?? 0)),
      );
    } catch (sendError) {
      setError({ code: 0, message: (sendError as Error).message });
    }
  }, []);

  return { items, status, detail, error, systemMessage, send, clearError: () => setError(null) };
}
