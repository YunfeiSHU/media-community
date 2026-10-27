import { get, request } from '@/shared/api/http';
import type { TranscodeInfo, UploadSession, UploadedImage } from './types';

export interface CreateVideoInput {
  title: string;
  description: string;
  categoryId: number | null;
  tags: string[];
  coverUrl: string;
}

/** 创建视频草稿（拿到 videoId 后才能 init 上传）。 */
export async function createVideoDraft(input: CreateVideoInput): Promise<{ videoId: number; status: string }> {
  return request<{ videoId: number; status: string }>({
    method: 'POST',
    url: '/videos',
    data: {
      title: input.title,
      description: input.description,
      categoryId: input.categoryId,
      tags: input.tags,
      coverUrl: input.coverUrl,
    },
  });
}

export function initUpload(videoId: number, filename: string, fileSize: number): Promise<UploadSession> {
  return request<UploadSession>({
    method: 'POST',
    url: `/videos/${videoId}/upload/init`,
    data: { filename, fileSize },
  });
}

export function fetchUploadStatus(videoId: number): Promise<UploadSession> {
  return get<UploadSession>(`/videos/${videoId}/upload/status`);
}

/**
 * 上传单个分片。
 *
 * 注意两点（见《前端设计》1.10）：
 *  - body 必须是 Blob/ArrayBuffer，浏览器才会带 Content-Length；ReadableStream 会变成 chunked 被后端拒绝；
 *  - 超时单独放宽，默认 15s 对 5MiB 分片不够。
 */
export function uploadPart(videoId: number, partNumber: number, chunk: Blob): Promise<UploadSession> {
  return request<UploadSession>({
    method: 'PUT',
    url: `/videos/${videoId}/upload/parts/${partNumber}`,
    data: chunk,
    headers: { 'Content-Type': 'application/octet-stream' },
    timeout: 120_000,
  });
}

export function completeUpload(videoId: number): Promise<UploadSession> {
  return request<UploadSession>({
    method: 'POST',
    url: `/videos/${videoId}/upload/complete`,
    timeout: 120_000,
  });
}

export function abortUpload(videoId: number): Promise<void> {
  return request<void>({ method: 'DELETE', url: `/videos/${videoId}/upload` });
}

export function fetchTranscode(videoId: number): Promise<TranscodeInfo> {
  return get<TranscodeInfo>(`/videos/${videoId}/transcode`);
}

export function retryTranscode(videoId: number): Promise<TranscodeInfo> {
  return request<TranscodeInfo>({ method: 'POST', url: `/videos/${videoId}/transcode/retry` });
}

export function publishVideo(videoId: number): Promise<{ videoId: number; status: string }> {
  return request<{ videoId: number; status: string }>({
    method: 'POST',
    url: `/videos/${videoId}/publish`,
  });
}

/** 图片上传：multipart，field 名必须是 file；scene 决定后端存储目录。 */
export function uploadImage(file: File, scene = 'image'): Promise<UploadedImage> {
  const form = new FormData();
  form.append('file', file);
  form.append('scene', scene);
  return request<UploadedImage>({
    method: 'POST',
    url: '/media/images',
    data: form,
    timeout: 60_000,
  });
}

/** 封面上传：兼容原有调用，等价于 scene=video_cover。 */
export function uploadCover(file: File): Promise<UploadedImage> {
  return uploadImage(file, 'video_cover');
}
