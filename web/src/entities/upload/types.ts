export type VideoStatus =
  | 'DRAFT'
  | 'TRANSCODING'
  | 'READY'
  | 'PUBLISHED'
  | 'OFFLINE'
  | 'FAILED';

export interface UploadPart {
  partNumber: number;
  size: number;
  etag: string;
}

/** 初始化 / 查询进度 / 上传分片三类接口共用的完整快照。 */
export interface UploadSession {
  videoId: number;
  uploadId: string;
  objectKey: string;
  ext: string;
  fileSize: number;
  partSize: number;
  totalParts: number;
  uploadedParts: UploadPart[];
  uploadedBytes: number;
  uploadedCount: number;
  completed: boolean;
}

export interface TranscodeInfo {
  videoId: number;
  taskId: number;
  status: 'PENDING' | 'RUNNING' | 'SUCCESS' | 'FAILED' | string;
  progress: number;
  profiles: string[];
  message: string;
}

export interface UploadedImage {
  url: string;
  width: number;
  height: number;
  sizeBytes: number;
  variants: Record<string, string>;
}

export interface PersistedUpload {
  videoId: number;
  uploadId: string;
  fileName: string;
  fileSize: number;
}
