import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { cancelOrder, mockPay } from '@/entities/order/api';
import { useOrder } from '@/entities/order/queries';
import { describeError } from '@/shared/api/errors';
import { formatDate } from '@/shared/lib/datetime';
import { formatCents } from '@/shared/lib/money';
import { EmptyState } from '@/widgets/empty-state/EmptyState';

export default function OrderDetailPage() {
  const { id = '' } = useParams();
  const queryClient = useQueryClient();
  const orderQuery = useOrder(id);

  const [pending, setPending] = useState<'pay' | 'cancel' | null>(null);
  const [error, setError] = useState<string | null>(null);

  const order = orderQuery.data;

  async function refresh(action: () => Promise<unknown>, kind: 'pay' | 'cancel') {
    setPending(kind);
    setError(null);
    try {
      await action();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['order', id] }),
        queryClient.invalidateQueries({ queryKey: ['orders'] }),
      ]);
    } catch (actionError) {
      setError(describeError(actionError));
    } finally {
      setPending(null);
    }
  }

  if (orderQuery.isPending) {
    return <div className="h-64 animate-[skeleton-pulse_1.2s_ease-in-out_infinite] ui-card" />;
  }

  if (orderQuery.isError || !order) {
    return <EmptyState title="订单不存在" action={<Link to="/orders" className="rounded-md bg-brand px-3 py-1.5 text-sm text-white">返回订单列表</Link>} />;
  }

  return (
    <div className="mx-auto max-w-3xl space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-h1 font-semibold text-ink">订单详情</h1>
        <Link to="/orders" className="text-meta text-ink-tertiary hover:text-brand">
          返回列表
        </Link>
      </div>

      <section className="ui-card p-4">
        <div className="flex flex-wrap items-center gap-3 text-meta text-ink-tertiary">
          <span className="font-mono">{order.orderId}</span>
          <span>{formatDate(order.createdAt, true)}</span>
          <span className="rounded-sm bg-page px-2 py-0.5 text-ink-secondary">{order.statusName}</span>
        </div>

        <ul className="mt-4 divide-y divide-line">
          {order.items.map((item) => (
            <li key={`${item.skuId}-${item.id}`} className="flex items-center gap-4 py-3">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm text-ink">{item.productName}</p>
                <p className="text-meta text-ink-tertiary">
                  {item.skuName || '默认规格'} · {formatCents(item.unitPrice)} × {item.quantity}
                </p>
              </div>
              <span className="text-sm text-ink-secondary">{formatCents(item.totalAmount)}</span>
            </li>
          ))}
        </ul>

        <div className="flex items-center justify-between border-t border-line pt-4">
          <span className="text-sm text-ink-secondary">合计</span>
          <strong className="text-lg text-brand">{formatCents(order.totalAmount)}</strong>
        </div>
      </section>

      <section className="ui-card p-4 text-meta leading-6 text-ink-tertiary">
        <p>下单时间：{formatDate(order.createdAt, true) || '—'}</p>
        <p>支付时间：{formatDate(order.paidAt, true) || '—'}</p>
        <p>超时时间：{formatDate(order.expireAt, true) || '—'}</p>
      </section>

      <div className="flex items-center gap-3">
        {order.status === 0 && (
          <>
            <button
              type="button"
              onClick={() => void refresh(() => mockPay(order.orderId), 'pay')}
              disabled={pending !== null}
              className="h-10 rounded-md bg-brand px-5 text-sm text-white disabled:opacity-60"
            >
              {pending === 'pay' ? '支付中…' : '模拟支付'}
            </button>
            <button
              type="button"
              onClick={() => void refresh(() => cancelOrder(order.orderId), 'cancel')}
              disabled={pending !== null}
              className="h-10 rounded-md border border-line px-5 text-sm text-ink-secondary disabled:opacity-60"
            >
              {pending === 'cancel' ? '取消中…' : '取消订单'}
            </button>
          </>
        )}
        {order.status === 1 && (
          <p className="text-sm text-success">已支付，等待发货 / 权益发放</p>
        )}
        {(order.status === 2 || order.status === 3) && (
          <p className="text-sm text-ink-tertiary">{order.statusName}</p>
        )}
      </div>

      {error && <p className="text-meta text-danger">{error}</p>}

      <p className="text-meta text-ink-tertiary">
        「模拟支付」只在本地/演示环境开启（后端 <code>MOCK_PAY_ENABLED</code>）。
      </p>
    </div>
  );
}
