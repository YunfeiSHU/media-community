import type { AxiosAdapter, AxiosInstance } from 'axios';

/**
 * 极简 mock：只在 `VITE_USE_MOCK=1` 时替换 axios 的 adapter。
 *
 * 为什么不用 MSW：MSW 要拉 service worker 与额外依赖，而这一阶段只需要
 * “没有后端也能看到首页/播放页”，用 adapter 更轻，联调时天然不会误用。
 * 数据形状与 `docs/接口文档.md` 对齐，接口一改这里要同步（见《开发问题记录》F-001）。
 */
export function installMockAdapter(client: AxiosInstance): void {
  const adapter: AxiosAdapter = async (config) => {
    const url = config.url ?? '';
    const method = (config.method ?? 'get').toUpperCase();
    const body = route(
      url,
      config.params as Record<string, unknown> | undefined,
      method,
      config.data as Record<string, unknown> | undefined,
    );
    return {
      data: body,
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    };
  };
  client.defaults.adapter = adapter;
}

const categories = [
  { id: 1, name: '动画', sort: 1 },
  { id: 2, name: '游戏', sort: 2 },
  { id: 3, name: '知识', sort: 3 },
  { id: 4, name: '生活', sort: 4 },
  { id: 5, name: '音乐', sort: 5 },
  { id: 6, name: '影视', sort: 6 },
];

const owners = [
  { uid: '1001', nickname: '演示实验室', avatar: '' },
  { uid: '1002', nickname: '夜航船', avatar: '' },
  { uid: '1003', nickname: '像素工坊', avatar: '' },
  { uid: '1004', nickname: '晚风电台', avatar: '' },
];

const titles = [
  '从零实现一个可用的弹幕渲染器',
  '三分钟讲清 HLS 与 FLV 的区别',
  '我把家里的旧手机做成了直播摄像头',
  '手写一个分片上传：断点续传到底难在哪',
  '为什么你的首屏加载慢：一次真实的性能排查',
  '用 Canvas 画一条会呼吸的曲线',
  '直播延迟 8 秒，能不能再低一点',
  '从 0 到 1 搭一个视频站的后端骨架',
  '镜头语言入门：三个机位讲一个故事',
  '把推荐系统做成一个无聊但正确的东西',
  '一次线上 504 的完整复盘',
  '独立开发者的第 100 天',
];

const videos = titles.map((title, index) => ({
  id: index + 1,
  title,
  description: `${title}。这是演示数据，用于在没有后端时预览页面结构。`,
  ownerId: owners[index % owners.length].uid,
  categoryId: categories[index % categories.length].id,
  categoryName: categories[index % categories.length].name,
  tags: ['演示', '技术'],
  coverUrl: '',
  duration: 180 + index * 37,
  status: 'PUBLISHED',
  viewCount: 1200 + index * 317,
  publishedAt: `2026-10-${String((index % 20) + 1).padStart(2, '0')}T10:30:00+08:00`,
  createdAt: `2026-10-${String((index % 20) + 1).padStart(2, '0')}T10:00:00+08:00`,
  updatedAt: `2026-10-${String((index % 20) + 1).padStart(2, '0')}T10:00:00+08:00`,
}));

const liveRooms = [1, 2, 3, 4].map((id) => ({
  roomId: String(5000 + id),
  title: ['深夜写代码，顺便聊聊重构', '一起看一部老电影', '手绘练习：从线稿到上色', '周末电台：随便聊聊'][id - 1],
  notice: '演示直播间',
  coverUrl: '',
  ownerId: owners[id % owners.length].uid,
  categoryId: categories[id % categories.length].id,
  status: 'LIVING',
  onlineCount: 120 * id,
  viewerTotal: 1000 * id,
  peakOnline: 300 * id,
  startedAt: '2026-10-26T20:00:00+08:00',
  playUrls: null,
}));

function page<T>(items: T[], params?: Record<string, unknown>) {
  const pageNo = Number(params?.page ?? 1);
  const pageSize = Number(params?.pageSize ?? 20);
  const start = (pageNo - 1) * pageSize;
  return {
    code: 0,
    message: 'success',
    data: { items: items.slice(start, start + pageSize), total: items.length, page: pageNo, pageSize },
  };
}

function ok(data: unknown) {
  return { code: 0, message: 'success', data };
}

// ---------------------------------------------------------------- 商品 / 订单 mock 数据
const products = [
  ['P20261001001', '演示联名马克杯', 8900, 3, 1, '上架'],
  ['P20261001002', '像素工坊帆布包', 12900, 2, 1, '上架'],
  ['P20261001003', '弹幕样式贴纸套装', 2900, 1, 1, '上架'],
  ['P20261001004', '深夜写代码卫衣', 19900, 4, 1, '上架'],
  ['P20261001005', '直播补光灯（迷你）', 15900, 2, 1, '上架'],
  ['P20261001006', '《从 0 到 1 做视频站》电子书', 4900, 1, 1, '上架'],
  ['P20261001007', '手绘分镜笔记本', 3900, 2, 1, '上架'],
  ['P20261001008', '桌面麦克风支架', 7900, 1, 0, '下架'],
] as const;

const mockProducts = products.map(([productId, name, minPrice, skuCount, status, statusName], index) => ({
  id: index + 1,
  productId,
  uid: '1001',
  name,
  description: `${name}。这是 mock 数据，用于在没有后端时预览商城页面。`,
  coverUrl: '',
  productType: 1,
  status,
  statusName,
  minPrice,
  skuCount,
}));

interface MockOrderItem {
  id: number;
  productId: string;
  skuId: string;
  productName: string;
  skuName: string;
  unitPrice: number;
  quantity: number;
  totalAmount: number;
}

interface MockOrder {
  id: number;
  orderId: string;
  status: number;
  statusName: string;
  totalAmount: number;
  expireAt: string;
  paidAt: string | null;
  completedAt: string | null;
  cancelledAt: string | null;
  createdAt: string;
  items: MockOrderItem[];
}

const orders: MockOrder[] = [];

function skuOf(productId: string) {
  return {
    skuId: `S${productId.slice(1)}01`,
    name: '默认规格',
    price: mockProducts.find((item) => item.productId === productId)?.minPrice ?? 0,
    status: 1,
    availableStock: 10,
  };
}

function route(
  url: string,
  params?: Record<string, unknown>,
  method = 'GET',
  data?: Record<string, unknown>,
) {
  // ---------------- 认证（mock 模式下任意账号密码都能登录）
  if (url === '/auth/login' || url === '/auth/token/refresh') {
    return ok({
      uid: '1001',
      accessToken: 'mock-access-token',
      refreshToken: '1001.mock-session.secret',
      accessTokenExpiresIn: 1800,
      refreshTokenExpiresIn: 604800,
    });
  }
  if (url === '/auth/logout' || url.startsWith('/auth/register/')) {
    return ok(null);
  }
  if (url === '/auth/verification/send') {
    return ok({ purpose: 'register', type: 'phone', target: 'demo', code: '123456' });
  }
  if (url === '/users/me') {
    return ok({
      uid: '1001',
      nickname: '演示用户',
      avatar: '',
      bio: '这是 mock 模式的演示账号',
      gender: 'unknown',
      role: 'ADMIN',
      followerCount: 12,
      followingCount: 3,
      videoCount: 8,
      createdAt: '2026-09-01T10:00:00+08:00',
    });
  }

  if (url.startsWith('/videos/') && url.endsWith('/play')) {
    const id = Number(url.split('/')[2]);
    const video = videos.find((item) => item.id === id) ?? videos[0];
    return {
      code: 0,
      message: 'success',
      data: {
        videoId: video.id,
        title: video.title,
        coverUrl: '',
        duration: video.duration,
        format: 'hls',
        // 公开测试流，仅用于验证播放器能跑起来
        playbackUrl: 'https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8',
        sources: [],
      },
    };
  }

  if (url.startsWith('/videos/')) {
    const id = Number(url.split('/')[2]);
    const video = videos.find((item) => item.id === id);
    return video
      ? { code: 0, message: 'success', data: { ...video, playbackUrl: '', sizeBytes: 0 } }
      : { code: 40401, message: '视频不存在', data: null };
  }

  if (url === '/videos') {
    const keyword = String(params?.keyword ?? '').trim();
    const filtered = keyword ? videos.filter((item) => item.title.includes(keyword)) : videos;
    return page(filtered, params);
  }

  if (url === '/live/rooms') {
    return page(liveRooms, { ...params, pageSize: params?.pageSize ?? 8 });
  }

  if (url === '/categories') {
    return { code: 0, message: 'success', data: { items: categories } };
  }

  if (url === '/profiles') {
    const uids = String(params?.uids ?? '').split(',').filter(Boolean);
    const items = owners
      .filter((owner) => uids.includes(owner.uid))
      .map((owner) => ({
        ...owner,
        bio: '演示资料',
        gender: 'unknown',
        followerCount: 12,
        followingCount: 3,
        videoCount: 4,
        createdAt: '2026-09-01T10:00:00+08:00',
      }));
    return { code: 0, message: 'success', data: { items } };
  }

  // ---------------- 商品
  if (url === '/products') {
    const keyword = String(params?.keyword ?? '').trim();
    const filtered = keyword
      ? mockProducts.filter((item) => item.name.includes(keyword))
      : mockProducts;
    const pageNo = Number(params?.page ?? 1);
    const pageSize = Number(params?.pageSize ?? 12);
    const start = (pageNo - 1) * pageSize;
    return ok({
      items: filtered.slice(start, start + pageSize),
      total: filtered.length,
      page: pageNo,
      pageSize,
    });
  }

  if (url.startsWith('/products/')) {
    const productId = decodeURIComponent(url.split('/')[2]);
    const product = mockProducts.find((item) => item.productId === productId);
    if (!product) return { code: 40401, message: '商品不存在', data: null };
    return ok({
      product,
      skus: [skuOf(product.productId)],
    });
  }

  // ---------------- 订单
  if (url === '/orders' && method === 'GET') {
    const status = params?.status;
    const list = status == null ? orders : orders.filter((item) => item.status === Number(status));
    return ok({ page: 1, size: 10, total: list.length, list });
  }

  if (url === '/orders' && method === 'POST') {
    const items = (data?.items as Array<{ skuId: string; quantity: number }> | undefined) ?? [];
    const orderItems: MockOrderItem[] = items.map((item, index) => {
      const product = mockProducts.find((candidate) => skuOf(candidate.productId).skuId === item.skuId);
      const price = product?.minPrice ?? 0;
      return {
        id: index + 1,
        productId: product?.productId ?? '',
        skuId: item.skuId,
        productName: product?.name ?? '演示商品',
        skuName: '默认规格',
        unitPrice: price,
        quantity: item.quantity,
        totalAmount: price * item.quantity,
      };
    });
    const now = new Date();
    const order: MockOrder = {
      id: orders.length + 1,
      orderId: `O${now.getTime()}`,
      status: 0,
      statusName: '待支付',
      totalAmount: orderItems.reduce((sum, item) => sum + item.totalAmount, 0),
      expireAt: new Date(now.getTime() + 30 * 60_000).toISOString(),
      paidAt: null,
      completedAt: null,
      cancelledAt: null,
      createdAt: now.toISOString(),
      items: orderItems,
    };
    orders.unshift(order);
    return ok(order);
  }

  if (url.startsWith('/orders/')) {
    const orderId = url.split('/')[2];
    const order = orders.find((item) => item.orderId === orderId);
    if (!order) return { code: 40401, message: '订单不存在', data: null };

    if (url.endsWith('/mock-pay')) {
      order.status = 1;
      order.statusName = '已支付';
      order.paidAt = new Date().toISOString();
      return ok(null);
    }
    if (url.endsWith('/cancel')) {
      order.status = 3;
      order.statusName = '已取消';
      order.cancelledAt = new Date().toISOString();
      return ok(order);
    }
    return ok(order);
  }

  return { code: 40401, message: `未 mock 的接口: ${url}`, data: null };
}
