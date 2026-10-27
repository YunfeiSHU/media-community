import { useQuery } from '@tanstack/react-query';
import { qk } from '@/shared/api/query-keys';
import { fetchPlayInfo, fetchVideo, fetchVideos } from './api';
import type { VideoListParams } from './types';

export function useVideos(params: VideoListParams = {}) {
  return useQuery({
    queryKey: qk.videos(params),
    queryFn: () => fetchVideos(params),
    staleTime: 60_000,
  });
}

export function useVideo(id: string) {
  return useQuery({
    queryKey: qk.video(id),
    queryFn: () => fetchVideo(id),
    staleTime: 5 * 60_000,
    enabled: Boolean(id),
  });
}

export function usePlayInfo(id: string) {
  return useQuery({
    queryKey: qk.play(id),
    queryFn: () => fetchPlayInfo(id),
    // 播放地址只缓存 30s：发布状态/地址可能变，且 /play 会累加播放量
    staleTime: 30_000,
    enabled: Boolean(id),
    refetchOnWindowFocus: false,
  });
}
