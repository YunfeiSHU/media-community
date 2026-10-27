import { get } from '@/shared/api/http';
import type { Page } from '@/shared/api/errors';
import { parseServerDate } from '@/shared/lib/datetime';

export type DanmakuPosition = 'SCROLL' | 'TOP' | 'BOTTOM';

export interface DanmakuItem {
  danmakuId: number;
  clientMsgId: string;
  content: string;
  uid: string;
  nickname: string;
  color: string;
  position: DanmakuPosition;
  timeMs: number;
  sentAt: Date | null;
}

interface DanmakuDTO {
  danmakuId?: number;
  clientMsgId?: string;
  content?: string;
  uid?: string;
  nickname?: string;
  color?: string;
  position?: string;
  timeMs?: number;
  sentAt?: string;
}

function toDanmaku(dto: DanmakuDTO): DanmakuItem {
  return {
    danmakuId: dto.danmakuId ?? 0,
    clientMsgId: dto.clientMsgId ?? '',
    content: dto.content ?? '',
    uid: dto.uid ?? '',
    nickname: dto.nickname ?? '',
    color: dto.color ?? '#ffffff',
    position: (dto.position as DanmakuPosition) ?? 'SCROLL',
    timeMs: dto.timeMs ?? 0,
    sentAt: parseServerDate(dto.sentAt),
  };
}

async function fetchHistory(path: string, pageSize: number): Promise<DanmakuItem[]> {
  const dto = await get<Page<DanmakuDTO> & { list?: DanmakuDTO[]; size?: number }>(path, {
    params: { page: 1, pageSize },
  });
  const raw = dto.items ?? dto.list ?? [];
  return raw.map(toDanmaku);
}

/** 直播历史：第 1 页走 Redis 热数据，pageSize 上限 200。 */
export function fetchRoomHistory(roomId: string, pageSize = 50): Promise<DanmakuItem[]> {
  return fetchHistory(`/danmaku/rooms/${roomId}/history`, pageSize);
}

/** 点播历史：按 timeMs 对齐播放进度，pageSize 上限 200。 */
export function fetchVideoHistory(videoId: string, pageSize = 200): Promise<DanmakuItem[]> {
  return fetchHistory(`/danmaku/videos/${videoId}/history`, pageSize);
}
