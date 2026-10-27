import { parseServerDate } from '@/shared/lib/datetime';
import type { Video, VideoDTO, VideoPageDTO, VideoStatus } from './types';

const STATUSES: VideoStatus[] = ['DRAFT', 'TRANSCODING', 'READY', 'PUBLISHED', 'OFFLINE', 'FAILED'];

export function toVideo(dto: VideoDTO): Video {
  const status = STATUSES.includes(dto.status as VideoStatus)
    ? (dto.status as VideoStatus)
    : 'DRAFT';

  return {
    id: String(dto.id ?? ''),
    title: dto.title ?? '未命名视频',
    description: dto.description ?? '',
    ownerId: dto.ownerId ?? '',
    categoryId: dto.categoryId ?? null,
    categoryName: dto.categoryName ?? '',
    tags: dto.tags ?? [],
    coverUrl: dto.coverUrl ?? '',
    durationSec: dto.duration ?? 0,
    status,
    viewCount: dto.viewCount ?? 0,
    publishedAt: parseServerDate(dto.publishedAt),
    createdAt: parseServerDate(dto.createdAt),
  };
}

/**
 * 分页归一：Java 是 {page,size,total,list}，Go 是 {total,page,pageSize,items}。
 * 统一成 {items,total,page,pageSize}，页面层不再关心来源。
 */
export function normalizePage(dto: VideoPageDTO): {
  items: Video[];
  total: number;
  page: number;
  pageSize: number;
} {
  const rawItems = dto.items ?? dto.list ?? [];
  return {
    items: rawItems.map(toVideo),
    total: dto.total ?? rawItems.length,
    page: dto.page ?? 1,
    pageSize: dto.pageSize ?? dto.size ?? rawItems.length,
  };
}
