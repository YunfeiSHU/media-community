import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { mockPay } from '@/entities/order/api';
import { useOrders } from '@/entities/order/queries';
import { describeError } from '@/shared/api/errors';
import { formatDate } from '@/shared/lib/datetime';
import { textOfNumber, ORDER_STATUS_TEXT } from '@/shared/lib/status';
import { formatCents } from '@/shared/lib/money';
import { EmptyState } from '@/widgets/empty-state/EmptyState';

const FILTERS: Array<{ label: string; value?: number }> = [
  { label: '全部' },
  { label: '待支付', value: 0 },
  { label: '已支付', value: 1 },
  { label: '已完成', value: 2 },
  { label: '已取消', value: 3 },
];

export default function OrdersPage() {
  const [status, setStatus] = useState<number | undefined>(undefined);
  const [payingId, setPayingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const queryClient = useQueryClient();
  const ordersQuery = useOrders({ page: 1, size: 20, status });

  async function pay(orderId: string) {
    setPayingId(orderId);
    setError(null);
    try {
      await mockPay(orderId);
      await queryClient.invalidateQueries({ queryKey: ['orders'] });
    } catch (payError) {
      setError(describeError(payError));
    } finally {
      setPayingId(null);
    }
  }

  return (
    <div className="space-y-4">
      <h1 className="text-h1 font-semibold text-ink">我的订单</h1>

      <div className="flex flex-wrap gap-2">
        {FILTERS.map((filter) => (
          <button
            key={filter.label}
            type="button"
            onClick={() => setStatus(filter.value)}
            className={`rounded-full px-3 py-1.5 text-sm ${
              status === filter.value ? 'bg-brand text-white' : 'bg-card text-ink-secondary'
            }`}
          >
            {filter.label}
          </button>
        ))}
      </div>

      {ordersQuery.isPending && (
        <div className="space-y-3">
          {[0, 1, 2].map((index) => (
            <div
              key={index}
              className="h-24 animate-[skeleton-pulse_1.2s_ease-in-out_infinite] ui-card"
            />
          ))}
        </div>
      )}

      {ordersQuery.isError && <EmptyState title="订单加载失败" hint={(ordersQuery.error as Error)?.message} />}

      {ordersQuery.isSuccess && ordersQuery.data.items.length === 0 && (
        <EmptyState title="还没有订单" hint="去商城挑点东西吧" action={<Link to="/shop" className="rounded-md bg-brand px-3 py-1.5 text-sm text-white">去商城</Link>} />
      )}

      {ordersQuery.isSuccess && ordersQuery.data.items.length > 0 && (
        <div className="space-y-3">
          {ordersQuery.data.items.map((order) => (
            <div key={order.orderId} className="ui-card p-4">
              <div className="flex flex-wrap items-center gap-3 text-meta text-ink-tertiary">
                <span className="font-mono">{order.orderId}</span>
                <span>{formatDate(order.createdAt, true)}</span>
                <span className="rounded-sm bg-page px-2 py-0.5 text-ink-secondary">
                  {order.statusName || textOfNumber(ORDER_STATUS_TEXT, order.status)}
                </span>
              </div>

              <ul className="mt-3 space-y-1 text-sm text-ink">
                {order.items.map((item) => (
                  <li key={`${order.orderId}-${item.skuId}`} className="flex justify-between gap-4">
                    <span className="min-w-0 truncate">
                      {item.productName}
                      {item.skuName ? ` · ${item.skuName}` : ''} × {item.quantity}
                    </span>
                    <span className="shrink-0 text-ink-secondary">{formatCents(item.totalAmount)}</span>
                  </li>
                ))}
              </ul>

              <div className="mt-3 flex items-center gap-3 border-t border-line pt-3">
                <span className="text-sm text-ink-secondary">
                  合计 <strong className="text-brand">{formatCents(order.totalAmount)}</strong>
                </span>
                <Link to={`/orders/${order.orderId}`} className="ml-auto text-meta text-ink-tertiary hover:text-brand">
                  查看详情
                </Link>
                {order.status === 0 && (
                  <button
                    type="button"
                    onClick={() => void pay(order.orderId)}
                    disabled={payingId === order.orderId}
                    className="h-8 rounded-md bg-brand px-3 text-meta text-white disabled:opacity-60"
                  >
                    {payingId === order.orderId ? '支付中…' : '去支付'}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {error && <p className="text-meta text-danger">{error}</p>}
    </div>
  );
}
