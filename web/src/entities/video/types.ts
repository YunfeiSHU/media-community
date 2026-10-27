import type { Page } from '@/shared/api/errors';

export type VideoStatus = 'DRAFT' | 'TRANSCODING' | 'READY' | 'PUBLISHED' | 'OFFLINE' | 'FAILED';

/** 页面层使用的干净模型：时间已解析、缺省值已补齐。 */
export interface Video {
  id: string;
  title: string;
  description: string;
  ownerId: string;
  categoryId: number | null;
  categoryName: string;
  tags: string[];
  coverUrl: string;
  durationSec: number;
  status: VideoStatus;
  viewCount: number;
  publishedAt: Date | null;
  createdAt: Date | null;
}

/** 后端原始结构：字段全部可选，兼容 Java 的 non_null 省略策略。 */
export interface VideoDTO {
  id?: number | string;
  title?: string;
  description?: string;
  ownerId?: string;
  categoryId?: number | null;
  categoryName?: string;
  tags?: string[] | null;
  coverUrl?: string | null;
  duration?: number;
  status?: string;
  viewCount?: number;
  publishedAt?: string | null;
  createdAt?: string | null;
  updatedAt?: string | null;
}

export interface VideoListParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
  categoryId?: number;
  mine?: boolean;
  /** 只看某个作者：公开主页用 */
  ownerId?: string;
  /** 只返回当前登录用户的收藏（后端按收藏时间倒序） */
  favorite?: boolean;
}

export type VideoPageDTO = Page<VideoDTO> & { list?: VideoDTO[]; size?: number };

export interface PlayInfo {
  videoId: string;
  title: string;
  coverUrl: string;
  durationSec: number;
  format: string;
  playbackUrl: string;
  sources: Array<{ profile: string; url: string; width: number; height: number; bandwidth: number }>;
}
