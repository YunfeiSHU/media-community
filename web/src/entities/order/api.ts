import { get, request } from '@/shared/api/http';
import { parseServerDate } from '@/shared/lib/datetime';
import type { Order, OrderDTO } from './types';

export function toOrder(dto: OrderDTO): Order {
  return {
    id: Number(dto.id ?? 0),
    orderId: dto.orderId ?? '',
    status: dto.status ?? 0,
    statusName: dto.statusName ?? '未知',
    totalAmount: dto.totalAmount ?? 0,
    expireAt: parseServerDate(dto.expireAt),
    paidAt: parseServerDate(dto.paidAt),
    completedAt: parseServerDate(dto.completedAt),
    cancelledAt: parseServerDate(dto.cancelledAt),
    createdAt: parseServerDate(dto.createdAt),
    items: (dto.items ?? []).map((item) => ({
      id: Number(item.id ?? 0),
      productId: item.productId ?? '',
      skuId: item.skuId ?? '',
      productName: item.productName ?? '',
      skuName: item.skuName ?? '',
      unitPrice: item.unitPrice ?? 0,
      quantity: item.quantity ?? 0,
      totalAmount: item.totalAmount ?? 0,
    })),
  };
}

/** 下单：只传 skuId 与数量，单价/总额由服务端按快照计算，前端不参与定价。 */
export async function createOrder(items: Array<{ skuId: string; quantity: number }>): Promise<Order> {
  const dto = await request<OrderDTO>({
    method: 'POST',
    url: '/orders',
    data: { items },
  });
  return toOrder(dto);
}

/** 订单列表。order-service 仍是旧分页结构 {page,size,total,list}，这里做归一。 */
export async function fetchOrders(params: {
  page?: number;
  size?: number;
  status?: number;
}): Promise<{ items: Order[]; total: number; page: number; pageSize: number }> {
  const dto = await get<{
    page?: number;
    size?: number;
    total?: number;
    list?: OrderDTO[];
    items?: OrderDTO[];
    pageSize?: number;
  }>('/orders', {
    params: {
      page: params.page ?? 1,
      size: params.size ?? 10,
      status: params.status,
    },
  });

  const raw = dto.items ?? dto.list ?? [];
  return {
    items: raw.map(toOrder),
    total: dto.total ?? raw.length,
    page: dto.page ?? 1,
    pageSize: dto.pageSize ?? dto.size ?? raw.length,
  };
}

export async function fetchOrder(id: string): Promise<Order> {
  return toOrder(await get<OrderDTO>(`/orders/${id}`));
}

/** 模拟支付：仅本地/演示开启，成功后订单变已支付。 */
export async function mockPay(orderId: string): Promise<void> {
  await request<unknown>({ method: 'POST', url: `/orders/${orderId}/mock-pay` });
}

export async function cancelOrder(orderId: string): Promise<Order> {
  return toOrder(await request<OrderDTO>({ method: 'POST', url: `/orders/${orderId}/cancel` }));
}
