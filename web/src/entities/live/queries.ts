import { useQuery } from '@tanstack/react-query';
import { qk } from '@/shared/api/query-keys';
import { fetchLivePlay, fetchLiveRoom, fetchLiveRooms, fetchMyLiveRooms, fetchOnline } from './api';

export function useLivingRooms(pageSize = 8) {
  return useQuery({
    queryKey: [...qk.liveRooms, pageSize],
    queryFn: () => fetchLiveRooms({ pageSize }),
    // 直播状态变化快，10s 后即视为过期
    staleTime: 10_000,
  });
}

export function useLiveRoom(id: string) {
  return useQuery({
    queryKey: ['live-room', id],
    queryFn: () => fetchLiveRoom(id),
    staleTime: 10_000,
    enabled: Boolean(id),
  });
}

/** 我的直播间（含未开播/已结束），用于开播管理页。 */
export function useMyLiveRooms() {
  return useQuery({
    queryKey: ['live-mine'],
    queryFn: () => fetchMyLiveRooms(),
    staleTime: 0,
  });
}

/**
 * 播放地址：只在 LIVING 时后端才返回 playUrls，否则 409/70001。
 * 这里把「未开播」当成业务状态而不是错误，避免红屏；409 也不重试。
 */
export function useLivePlay(id: string) {
  return useQuery({
    queryKey: ['live-play', id],
    queryFn: () => fetchLivePlay(id),
    staleTime: 10_000,
    enabled: Boolean(id),
    retry: false,
  });
}

/** 在线人数：轻量接口，5–10s 轮询；WS 只推弹幕与系统事件。 */
export function useOnline(roomId: string) {
  return useQuery({
    queryKey: ['live-online', roomId],
    queryFn: () => fetchOnline(roomId),
    refetchInterval: 8000,
    enabled: Boolean(roomId),
  });
}
