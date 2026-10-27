export type LiveStatus = 'IDLE' | 'STARTING' | 'LIVING' | 'ENDED' | 'FAILED';

export interface PlayUrls {
  hls: string;
  httpFlv: string;
  webrtc: string;
  rtmp: string;
}

export interface LiveRoom {
  roomId: string;
  title: string;
  notice: string;
  coverUrl: string;
  ownerId: string;
  categoryId: number | null;
  status: LiveStatus;
  onlineCount: number;
  viewerTotal: number;
  peakOnline: number;
  startedAt: Date | null;
  playUrls: PlayUrls | null;
}

export interface LiveRoomDTO {
  roomId?: number | string;
  title?: string;
  notice?: string;
  coverUrl?: string | null;
  ownerId?: string;
  categoryId?: number | null;
  status?: string;
  onlineCount?: number;
  viewerTotal?: number;
  peakOnline?: number;
  startedAt?: string | null;
}
