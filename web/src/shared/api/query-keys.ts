import type { VideoListParams } from '@/entities/video/types';

/** 查询键集中管理：invalidate 时不用记字符串。 */
export const qk = {
  categories: ['categories'] as const,
  videos: (params: VideoListParams) => ['videos', params] as const,
  infiniteVideos: (params: VideoListParams) => ['videos', 'infinite', params] as const,
  video: (id: string) => ['video', id] as const,
  play: (id: string) => ['video', id, 'play'] as const,
  liveRooms: ['live-rooms'] as const,
  profiles: (uids: string[]) => ['profiles', [...uids].sort().join(',')] as const,
};
