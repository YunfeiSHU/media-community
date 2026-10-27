import { getAccessToken } from '@/shared/api/session';

export interface DanmakuFrame {
  type: 'welcome' | 'danmaku' | 'pong' | 'system' | 'error';
  data: Record<string, unknown>;
}

export type SocketStatus = 'connecting' | 'open' | 'closed' | 'reconnecting';

interface Options {
  roomId?: string;
  videoId?: string;
  uid?: string;
  nickname?: string;
}

const MAX_RECONNECT = 8;
const PING_INTERVAL = 25_000;

/**
 * 弹幕 WebSocket 客户端。
 *
 * 为什么不用 react-query 包一层：它是一个有状态的长连接，重连、心跳、关闭码
 * 都需要按时间顺序处理，塞进 hooks 反而更难保证一致性。
 */
export class DanmakuSocket {
  private ws: WebSocket | null = null;

  private reconnectAttempt = 0;

  private pingTimer: number | null = null;

  private reconnectTimer: number | null = null;

  private manuallyClosed = false;

  private readonly seen = new Set<string>();

  constructor(
    private readonly options: Options,
    private readonly onFrame: (frame: DanmakuFrame) => void,
    private readonly onStatus: (status: SocketStatus, detail?: string) => void,
  ) {}

  get isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  connect(): void {
    this.manuallyClosed = false;
    this.open();
  }

  close(): void {
    this.manuallyClosed = true;
    this.clearTimers();
    this.ws?.close(1000, 'client close');
    this.ws = null;
  }

  /** 发送弹幕；未连接时抛出，由 UI 提示“连接中，请稍后”。 */
  send(content: string, color = '#FFFFFF', position = 'SCROLL', timeMs = 0): string {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('弹幕连接未就绪，请稍后再试');
    }
    const clientMsgId = crypto.randomUUID();
    this.ws.send(
      JSON.stringify({
        type: 'danmaku',
        data: { content, color, position, timeMs, clientMsgId },
      }),
    );
    return clientMsgId;
  }

  private open(): void {
    const params = new URLSearchParams();
    if (this.options.roomId) params.set('roomId', this.options.roomId);
    if (this.options.videoId) params.set('videoId', this.options.videoId);
    if (this.options.uid) params.set('uid', this.options.uid);
    if (this.options.nickname) params.set('nickname', this.options.nickname);

    // 浏览器无法给 WebSocket 设请求头，只能把令牌放 query；网关会在转发前摘掉
    const token = getAccessToken();
    if (token) params.set('access_token', token);

    const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${scheme}://${window.location.host}/ws/danmaku?${params.toString()}`;

    this.onStatus(this.reconnectAttempt > 0 ? 'reconnecting' : 'connecting');
    const socket = new WebSocket(url);
    this.ws = socket;

    socket.onopen = () => {
      this.reconnectAttempt = 0;
      this.onStatus('open');
      this.startPing();
    };

    socket.onmessage = (event) => {
      let frame: DanmakuFrame;
      try {
        frame = JSON.parse(String(event.data)) as DanmakuFrame;
      } catch {
        return;
      }

      if (frame.type === 'danmaku') {
        const key = String(frame.data.clientMsgId ?? '') || String(frame.data.danmakuId ?? '');
        // 自己的消息会先本地回显，再用 clientMsgId 去重，避免出现两条
        if (key && this.seen.has(key)) return;
        if (key) this.seen.add(key);
      }

      this.onFrame(frame);
    };

    socket.onclose = (event) => {
      this.clearTimers();
      this.ws = null;
      if (this.manuallyClosed) {
        this.onStatus('closed');
        return;
      }

      if (event.code === 1000) {
        this.onStatus('closed');
        return;
      }
      if (event.code === 4001) {
        this.onStatus('closed', '同一账号连接过多，请在其它标签页关闭弹幕');
        return;
      }
      if (event.code === 1003) {
        this.onStatus('closed', '弹幕协议异常，请刷新页面');
        return;
      }

      this.scheduleReconnect();
    };

    socket.onerror = () => {
      // 具体原因在 onclose 里，不重复提示
    };
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempt >= MAX_RECONNECT) {
      this.onStatus('closed', '弹幕连接已断开，刷新页面重试');
      return;
    }
    const delay = Math.min(30_000, 1000 * 2 ** this.reconnectAttempt);
    this.reconnectAttempt += 1;
    this.onStatus('reconnecting', `${Math.round(delay / 1000)}s 后重连`);
    this.reconnectTimer = window.setTimeout(() => this.open(), delay);
  }

  private startPing(): void {
    this.clearTimers();
    this.pingTimer = window.setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping', data: {} }));
      }
    }, PING_INTERVAL);
  }

  private clearTimers(): void {
    if (this.pingTimer) window.clearInterval(this.pingTimer);
    if (this.reconnectTimer) window.clearTimeout(this.reconnectTimer);
    this.pingTimer = null;
    this.reconnectTimer = null;
  }
}
