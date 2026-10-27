import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
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

/**
 * 分页列表：收藏 / 我的投稿用。
 * 返回的 `items` 由页面自己 flatMap，避免把分页细节泄漏到组件。
 */
export function useInfiniteVideos(params: VideoListParams = {}) {
  return useInfiniteQuery({
    queryKey: qk.infiniteVideos(params),
    queryFn: ({ pageParam }) =>
      fetchVideos({ ...params, page: pageParam, pageSize: params.pageSize ?? 12 }),
    initialPageParam: 1,
    getNextPageParam: (last) => {
      const loaded = (last.page - 1) * last.pageSize + last.items.length;
      return loaded < last.total ? last.page + 1 : undefined;
    },
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
