import { get, request } from '@/shared/api/http';
import { parseServerDate } from '@/shared/lib/datetime';
import type { LiveRoom, LiveRoomDTO, LiveStatus, PlayUrls } from './types';

const STATUSES: LiveStatus[] = ['IDLE', 'STARTING', 'LIVING', 'ENDED', 'FAILED'];

interface RoomDetailDTO extends LiveRoomDTO {
  playUrls?: PlayUrls | null;
  sessionId?: string;
}

export function toLiveRoom(dto: RoomDetailDTO): LiveRoom {
  return {
    roomId: String(dto.roomId ?? ''),
    title: dto.title ?? '未命名直播间',
    notice: dto.notice ?? '',
    coverUrl: dto.coverUrl ?? '',
    ownerId: dto.ownerId ?? '',
    categoryId: dto.categoryId ?? null,
    status: STATUSES.includes(dto.status as LiveStatus) ? (dto.status as LiveStatus) : 'IDLE',
    onlineCount: dto.onlineCount ?? 0,
    viewerTotal: dto.viewerTotal ?? 0,
    peakOnline: dto.peakOnline ?? 0,
    startedAt: parseServerDate(dto.startedAt),
    playUrls: dto.playUrls ?? null,
  };
}

export async function fetchLiveRooms(params: { pageSize?: number } = {}): Promise<LiveRoom[]> {
  const dto = await get<{ items?: RoomDetailDTO[]; total?: number }>('/live/rooms', {
    params: { livingOnly: true, page: 1, pageSize: params.pageSize ?? 8 },
  });
  return (dto.items ?? []).map(toLiveRoom);
}

export async function fetchLiveRoom(id: string): Promise<LiveRoom> {
  return toLiveRoom(await get<RoomDetailDTO>(`/live/rooms/${id}`));
}

export interface CreateLiveRoomInput {
  title: string;
  notice?: string;
  coverUrl?: string;
  categoryId?: number;
}

export interface CreateLiveRoomResult {
  roomId: string;
  title: string;
  status: LiveStatus;
  streamKey: string;
  pushUrl: string;
}

/** 创建直播间：owner 取网关注入的当前用户；新房间是 IDLE，需要进房间页开播。 */
export async function createLiveRoom(input: CreateLiveRoomInput): Promise<CreateLiveRoomResult> {
  const dto = await request<{
    roomId?: number;
    title?: string;
    status?: string;
    streamKey?: string;
    pushUrl?: string;
  }>({
    method: 'POST',
    url: '/live/rooms',
    data: {
      title: input.title,
      notice: input.notice ?? '',
      coverUrl: input.coverUrl ?? '',
      categoryId: input.categoryId ?? null,
    },
  });

  return {
    roomId: String(dto.roomId ?? ''),
    title: dto.title ?? input.title,
    status: STATUSES.includes(dto.status as LiveStatus) ? (dto.status as LiveStatus) : 'IDLE',
    streamKey: dto.streamKey ?? '',
    pushUrl: dto.pushUrl ?? '',
  };
}

/** 我的直播间：`mine=true` 由后端按 X-User-Id 过滤，包含未开播/已结束的房间。 */
export async function fetchMyLiveRooms(pageSize = 50): Promise<LiveRoom[]> {
  const dto = await get<{ items?: RoomDetailDTO[]; total?: number }>('/live/rooms', {
    params: { mine: true, page: 1, pageSize },
  });
  return (dto.items ?? []).map(toLiveRoom);
}

export interface LivePlayInfo {
  roomId: string;
  title: string;
  status: string;
  onlineCount: number;
  playUrls: PlayUrls | null;
}

/** 播放地址：未开播时后端返回 409/70001，调用方按业务错误处理，不要当异常弹红。 */
export async function fetchLivePlay(id: string): Promise<LivePlayInfo> {
  const dto = await get<{
    roomId?: number;
    title?: string;
    status?: string;
    onlineCount?: number;
    playUrls?: PlayUrls | null;
  }>(`/live/rooms/${id}/play`);

  return {
    roomId: String(dto.roomId ?? id),
    title: dto.title ?? '',
    status: dto.status ?? '',
    onlineCount: dto.onlineCount ?? 0,
    playUrls: dto.playUrls ?? null,
  };
}

export interface HeartbeatResult {
  roomId: number;
  onlineCount: number;
  memberKey: string;
  firstEnter: boolean;
  ttlSec: number;
}

/**
 * 观众心跳：登录用户靠网关身份，游客靠 sessionId。
 *
 * 未开播房间会返回 409/70001（后端不给人气灌水），调用方应静默停止心跳。
 */
export function sendHeartbeat(roomId: string, sessionId: string | null): Promise<HeartbeatResult> {
  return request<HeartbeatResult>({
    method: 'POST',
    url: `/live/rooms/${roomId}/heartbeat`,
    data: sessionId ? { sessionId } : {},
  });
}

export interface OnlineInfo {
  roomId: string;
  onlineCount: number;
  peakOnline: number;
  viewerTotal: number;
}

export async function fetchOnline(roomId: string): Promise<OnlineInfo> {
  const dto = await get<{
    roomId?: number;
    onlineCount?: number;
    peakOnline?: number;
    viewerTotal?: number;
  }>(`/live/rooms/${roomId}/online`);

  return {
    roomId: String(dto.roomId ?? roomId),
    onlineCount: dto.onlineCount ?? 0,
    peakOnline: dto.peakOnline ?? 0,
    viewerTotal: dto.viewerTotal ?? 0,
  };
}

export interface StartLiveResult {
  roomId: string;
  status: string;
  sessionId: string;
  streamKey: string;
  pushUrl: string;
  srsAvailable: boolean;
  hint: string;
}

/** 开播：状态先变 STARTING，收到 SRS on_publish 后才是 LIVING。 */
export async function startLive(roomId: string): Promise<StartLiveResult> {
  const dto = await request<{
    roomId?: number;
    status?: string;
    sessionId?: string;
    streamKey?: string;
    pushUrl?: string;
    srsAvailable?: boolean;
    hint?: string;
  }>({ method: 'POST', url: `/live/rooms/${roomId}/start` });

  return {
    roomId: String(dto.roomId ?? roomId),
    status: dto.status ?? 'STARTING',
    sessionId: dto.sessionId ?? '',
    streamKey: dto.streamKey ?? '',
    pushUrl: dto.pushUrl ?? '',
    srsAvailable: dto.srsAvailable ?? false,
    hint: dto.hint ?? '',
  };
}

/** 关播：streamKicked=false 表示没连上 SRS，但关播本身已成功。 */
export async function stopLive(roomId: string): Promise<{ status: string; streamKicked: boolean }> {
  const dto = await request<{ status?: string; streamKicked?: boolean }>({
    method: 'POST',
    url: `/live/rooms/${roomId}/stop`,
  });
  return { status: dto.status ?? 'ENDED', streamKicked: dto.streamKicked ?? false };
}
