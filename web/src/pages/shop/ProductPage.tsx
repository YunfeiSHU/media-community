import { useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { createOrder } from '@/entities/order/api';
import { useProductDetail } from '@/entities/product/queries';
import { useAuthStore } from '@/features/auth/store';
import { describeError } from '@/shared/api/errors';
import { formatCents } from '@/shared/lib/money';
import { DefaultCover } from '@/shared/ui/DefaultCover';
import { EmptyState } from '@/widgets/empty-state/EmptyState';

export default function ProductPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const authStatus = useAuthStore((state) => state.status);

  const detailQuery = useProductDetail(id);
  const product = detailQuery.data?.product;
  const skus = detailQuery.data?.skus ?? [];

  const [skuId, setSkuId] = useState<string>('');
  const [quantity, setQuantity] = useState(1);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const selectedSku = useMemo(
    () => skus.find((sku) => sku.skuId === skuId) ?? skus[0],
    [skus, skuId],
  );

  async function buy() {
    if (!selectedSku) return;
    if (authStatus !== 'authed') {
      navigate(`/login?returnTo=${encodeURIComponent(`/shop/${id}`)}`);
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      const order = await createOrder([{ skuId: selectedSku.skuId, quantity }]);
      await queryClient.invalidateQueries({ queryKey: ['orders'] });
      navigate(`/orders/${order.orderId}`);
    } catch (submitError) {
      setError(describeError(submitError));
    } finally {
      setSubmitting(false);
    }
  }

  if (detailQuery.isPending) {
    return <div className="h-72 animate-pulse rounded-lg bg-subtle" />;
  }

  if (detailQuery.isError || !product) {
    return <EmptyState title="商品不存在或已下架" />;
  }

  const total = selectedSku ? selectedSku.price * quantity : 0;

  return (
    <div className="grid gap-8 lg:grid-cols-[minmax(0,440px)_minmax(0,1fr)]">
      <div className="relative aspect-square overflow-hidden rounded-lg bg-subtle">
        <DefaultCover
          seed={product.productId || product.name}
          kind="product"
          hint={product.name}
          className="absolute inset-0 h-full w-full"
        />

        {product.coverUrl && (
          <img
            src={product.coverUrl}
            alt={product.name}
            className="relative h-full w-full object-cover"
            onError={(event) => {
              event.currentTarget.style.display = 'none';
            }}
          />
        )}
      </div>

      <div className="space-y-5">
        <div>
          <h1 className="text-display font-semibold text-ink">{product.name}</h1>
          <p className="mt-1.5 text-meta text-ink-tertiary">
            {product.productId} · {product.statusName} · {product.skuCount} 个规格
          </p>
        </div>

        <p className="text-display font-semibold tabular-nums text-brand">
          {formatCents(selectedSku?.price ?? product.minPrice)}
        </p>

        <section>
          <h2 className="mb-2 text-body text-ink-secondary">选择规格</h2>
          <div className="flex flex-wrap gap-2">
            {skus.map((sku) => {
              const disabled = sku.availableStock <= 0;
              const active = selectedSku?.skuId === sku.skuId;
              return (
                <button
                  key={sku.skuId}
                  type="button"
                  disabled={disabled}
                  onClick={() => setSkuId(sku.skuId)}
                  className={`rounded-full border px-3.5 py-2 text-sm transition-colors ${
                    active
                      ? 'border-brand bg-brand/10 text-brand'
                      : 'border-line bg-card text-ink-secondary hover:bg-subtle'
                  } disabled:cursor-not-allowed disabled:opacity-40`}
                >
                  {sku.name}
                  <span className="ml-2 text-meta tabular-nums">{formatCents(sku.price)}</span>
                  {disabled && <span className="ml-2 text-meta">无货</span>}
                </button>
              );
            })}
            {skus.length === 0 && <p className="text-meta text-ink-tertiary">该商品还没有配置规格</p>}
          </div>
        </section>

        <section className="flex items-center gap-3">
          <h2 className="text-body text-ink-secondary">数量</h2>
          <div className="flex items-center rounded-full border border-line">
            <button
              type="button"
              onClick={() => setQuantity((value) => Math.max(1, value - 1))}
              className="h-9 w-9 text-ink-secondary"
              aria-label="减少数量"
            >
              −
            </button>
            <span className="w-10 text-center text-sm tabular-nums">{quantity}</span>
            <button
              type="button"
              onClick={() =>
                setQuantity((value) =>
                  Math.min(value + 1, Math.max(1, selectedSku?.availableStock ?? 1), 999),
                )
              }
              className="h-9 w-9 text-ink-secondary"
              aria-label="增加数量"
            >
              +
            </button>
          </div>
          <span className="text-meta text-ink-tertiary">库存 {selectedSku?.availableStock ?? 0}</span>
        </section>

        <div className="flex flex-wrap items-center gap-3 border-t border-line pt-5">
          <span className="text-body text-ink-secondary">
            合计 <strong className="text-h1 tabular-nums text-brand">{formatCents(total)}</strong>
          </span>
          <button
            type="button"
            onClick={() => void buy()}
            disabled={submitting || !selectedSku || selectedSku.availableStock <= 0}
            className="ui-btn-primary ml-auto h-11 px-8 text-base"
          >
            {submitting ? '下单中…' : authStatus === 'authed' ? '立即购买' : '登录后购买'}
          </button>
        </div>

        {error && <p className="text-meta text-danger">{error}</p>}

        {product.description && (
          <section className="ui-card p-4">
            <h2 className="mb-2 text-body font-medium text-ink">商品详情</h2>
            <p className="whitespace-pre-wrap text-body text-ink-secondary">{product.description}</p>
          </section>
        )}
      </div>
    </div>
  );
}
