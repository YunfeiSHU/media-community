import { get, request } from '@/shared/api/http';
import type { PlayInfo, Video, VideoDTO, VideoListParams, VideoPageDTO } from './types';
import { normalizePage, toVideo } from './adapters';

export async function fetchVideos(params: VideoListParams = {}): Promise<{
  items: Video[];
  total: number;
  page: number;
  pageSize: number;
}> {
  const dto = await get<VideoPageDTO>('/videos', {
    params: {
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 20,
      keyword: params.keyword || undefined,
      categoryId: params.categoryId || undefined,
      mine: params.mine ? true : undefined,
      ownerId: params.ownerId || undefined,
      favorite: params.favorite ? true : undefined,
    },
  });
  return normalizePage(dto);
}

export async function fetchVideo(id: string): Promise<Video> {
  const dto = await get<VideoDTO>(`/videos/${id}`);
  return toVideo(dto);
}

export interface UpdateVideoInput {
  title?: string;
  description?: string;
  categoryId?: number | null;
  tags?: string[];
  coverUrl?: string;
}

/** 修改视频（仅作者/管理员）。 */
export async function updateVideo(id: string, input: UpdateVideoInput): Promise<Video> {
  const dto = await request<VideoDTO>({ method: 'PUT', url: `/videos/${id}`, data: input });
  return toVideo(dto);
}

/** 删除视频（仅作者/管理员）。 */
export function deleteVideo(id: string): Promise<void> {
  return request<void>({ method: 'DELETE', url: `/videos/${id}` });
}

/** 下架已发布视频。 */
export function offlineVideo(id: string): Promise<{ videoId: string; status: string }> {
  return request<{ videoId: string; status: string }>({
    method: 'POST',
    url: `/videos/${id}/offline`,
  });
}

export async function fetchPlayInfo(id: string): Promise<PlayInfo> {
  const dto = await get<{
    videoId?: number;
    title?: string;
    coverUrl?: string;
    duration?: number;
    format?: string;
    playbackUrl?: string;
    sources?: PlayInfo['sources'];
  }>(`/videos/${id}/play`);

  return {
    videoId: String(dto.videoId ?? id),
    title: dto.title ?? '',
    coverUrl: dto.coverUrl ?? '',
    durationSec: dto.duration ?? 0,
    format: dto.format ?? 'hls',
    playbackUrl: dto.playbackUrl ?? '',
    sources: dto.sources ?? [],
  };
}

/** 单条视频的互动状态（游客也能拿，liked/favorited 恒为 false）。 */
export interface InteractionState {
  videoId: number;
  liked: boolean;
  favorited: boolean;
  likeCount: number;
  favoriteCount: number;
}

export function fetchInteraction(id: string): Promise<InteractionState> {
  return get<InteractionState>(`/videos/${id}/interactions`);
}

/** 点赞 / 取消点赞；后端幂等，重复调用不会重复计数。 */
export function setLiked(id: string, liked: boolean): Promise<InteractionState> {
  return request<InteractionState>({
    method: liked ? 'POST' : 'DELETE',
    url: `/videos/${id}/like`,
  });
}

/** 收藏 / 取消收藏。 */
export function setFavorited(id: string, favorited: boolean): Promise<InteractionState> {
  return request<InteractionState>({
    method: favorited ? 'POST' : 'DELETE',
    url: `/videos/${id}/favorite`,
  });
}
