import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createProduct,
  createSku,
  deleteProduct,
  fetchMyProducts,
  fetchProductDetail,
  increaseSkuStock,
  offShelfProduct,
  onShelfProduct,
  updateSkuPrice,
} from '@/entities/product/api';
import type { ProductDetail } from '@/entities/product/types';
import { describeError } from '@/shared/api/errors';
import { CardGridSkeleton, EmptyState } from '@/widgets/empty-state/EmptyState';

function yuan(cents: number | null): string {
  return cents === null ? '—' : (cents / 100).toFixed(2);
}

/** 我的商品管理：创建商品 + 默认 SKU、上下架、改价、补库存、删除。 */
export default function ProductManagePage() {
  const queryClient = useQueryClient();
  const productsQuery = useQuery({
    queryKey: ['my-products'],
    queryFn: () => fetchMyProducts(),
    staleTime: 0,
  });
  const products = productsQuery.data ?? [];

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [priceYuan, setPriceYuan] = useState('99');
  const [stock, setStock] = useState('10');
  const [creating, setCreating] = useState(false);

  const [expanded, setExpanded] = useState<string | null>(null);
  const [detail, setDetail] = useState<ProductDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [priceDraft, setPriceDraft] = useState<Record<string, string>>({});
  const [stockDraft, setStockDraft] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  function refreshList() {
    void queryClient.invalidateQueries({ queryKey: ['my-products'] });
    void queryClient.invalidateQueries({ queryKey: ['products'] });
  }

  async function handleCreate() {
    if (!name.trim()) {
      setError('商品名不能为空');
      return;
    }
    const price = Math.round(Number(priceYuan) * 100);
    const initialStock = Number(stock);
    if (!Number.isFinite(price) || price < 0) {
      setError('价格不合法');
      return;
    }
    if (!Number.isInteger(initialStock) || initialStock < 0) {
      setError('库存必须是非负整数');
      return;
    }

    setCreating(true);
    setError(null);
    setNotice(null);
    try {
      const created = await createProduct({ name: name.trim(), description: description.trim() });
      if (!created.product.productId) throw new Error('创建商品失败：缺少 productId');
      await createSku(created.product.productId, {
        name: '默认规格',
        price,
        initialStock,
      });
      setName('');
      setDescription('');
      setNotice('商品已创建（草稿），上架后才会出现在商城');
      refreshList();
    } catch (createError) {
      setError(describeError(createError));
    } finally {
      setCreating(false);
    }
  }

  async function toggleDetail(productId: string) {
    if (expanded === productId) {
      setExpanded(null);
      setDetail(null);
      return;
    }
    setExpanded(productId);
    setDetail(null);
    setDetailLoading(true);
    setError(null);
    try {
      const data = await fetchProductDetail(productId);
      setDetail(data);
      setPriceDraft(
        Object.fromEntries(data.skus.map((sku) => [sku.skuId, (sku.price / 100).toFixed(2)])),
      );
      setStockDraft(Object.fromEntries(data.skus.map((sku) => [sku.skuId, '0'])));
    } catch (detailError) {
      setError(describeError(detailError));
    } finally {
      setDetailLoading(false);
    }
  }

  async function handleShelf(productId: string, on: boolean) {
    setBusy(`shelf:${productId}`);
    setError(null);
    try {
      if (on) await onShelfProduct(productId);
      else await offShelfProduct(productId);
      setNotice(on ? '已上架' : '已下架');
      refreshList();
    } catch (shelfError) {
      setError(describeError(shelfError));
    } finally {
      setBusy(null);
    }
  }

  async function handleDelete(productId: string, productName: string) {
    if (!window.confirm(`确定删除「${productName}」？`)) return;
    setBusy(`delete:${productId}`);
    setError(null);
    try {
      await deleteProduct(productId);
      setNotice('已删除');
      setExpanded(null);
      setDetail(null);
      refreshList();
    } catch (deleteError) {
      setError(describeError(deleteError));
    } finally {
      setBusy(null);
    }
  }

  async function saveSkuPrice(skuId: string) {
    const value = Math.round(Number(priceDraft[skuId]) * 100);
    if (!Number.isFinite(value) || value < 0) {
      setError('价格不合法');
      return;
    }
    setBusy(`price:${skuId}`);
    setError(null);
    try {
      await updateSkuPrice(skuId, value);
      setNotice('价格已更新');
      refreshList();
    } catch (priceError) {
      setError(describeError(priceError));
    } finally {
      setBusy(null);
    }
  }

  async function addStock(skuId: string) {
    const value = Number(stockDraft[skuId]);
    if (!Number.isInteger(value) || value <= 0) {
      setError('补货数量必须是正整数');
      return;
    }
    setBusy(`stock:${skuId}`);
    setError(null);
    try {
      await increaseSkuStock(skuId, value);
      setNotice('库存已增加');
      setStockDraft((prev) => ({ ...prev, [skuId]: '0' }));
      const refreshed = detail ? await fetchProductDetail(detail.product.productId) : null;
      setDetail(refreshed);
      refreshList();
    } catch (stockError) {
      setError(describeError(stockError));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-h1 font-semibold text-ink">商品管理</h1>
        <div className="flex gap-2 text-meta">
          <Link
            to="/studio/videos"
            className="ui-btn-ghost h-8 px-3 text-meta text-ink-secondary hover:text-brand"
          >
            视频管理
          </Link>
          <Link to="/shop" className="ui-btn-ghost h-8 px-3 text-meta text-ink-secondary hover:text-brand">
            去商城
          </Link>
        </div>
      </div>

      {error && <p className="text-meta text-brand">{error}</p>}
      {notice && <p className="text-meta text-ink-tertiary">{notice}</p>}

      <section className="space-y-3 ui-card p-4">
        <h2 className="text-sm font-medium text-ink">新建商品</h2>
        <div className="grid gap-2 sm:grid-cols-2">
          <input
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="商品名"
            maxLength={128}
            className="ui-input"
          />
          <input
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="描述（可选）"
            maxLength={500}
            className="ui-input"
          />
          <label className="flex items-center gap-2 text-meta text-ink-tertiary">
            价格（元）
            <input
              value={priceYuan}
              onChange={(event) => setPriceYuan(event.target.value)}
              inputMode="decimal"
              className="ui-input min-w-0 flex-1"
            />
          </label>
          <label className="flex items-center gap-2 text-meta text-ink-tertiary">
            初始库存
            <input
              value={stock}
              onChange={(event) => setStock(event.target.value)}
              inputMode="numeric"
              className="ui-input min-w-0 flex-1"
            />
          </label>
        </div>
        <button
          type="button"
          onClick={() => void handleCreate()}
          disabled={creating}
          className="h-9 ui-btn-primary disabled:opacity-60"
        >
          {creating ? '创建中…' : '创建商品（含默认 SKU）'}
        </button>
      </section>

      {productsQuery.isPending && <CardGridSkeleton count={4} />}
      {productsQuery.isError && <EmptyState title="商品加载失败" hint="请确认商品服务已启动" />}
      {productsQuery.isSuccess && products.length === 0 && (
        <EmptyState title="还没有商品" hint="在上方创建第一个商品" />
      )}

      {products.length > 0 && (
        <ul className="ui-card divide-y divide-line overflow-hidden">
          {products.map((product) => (
            <li key={product.productId} className="space-y-2 px-4 py-3">
              <div className="flex flex-wrap items-center gap-3">
                <span className="rounded-sm bg-page px-1.5 py-0.5 text-meta text-ink-secondary">
                  {product.statusName}
                </span>
                <span className="min-w-0 flex-1 truncate text-sm text-ink">{product.name}</span>
                <span className="text-meta text-ink-tertiary">
                  ¥{yuan(product.minPrice)} · {product.skuCount} 个 SKU
                </span>
                <div className="flex gap-2 text-meta">
                  <button
                    type="button"
                    onClick={() => void toggleDetail(product.productId)}
                    className="ui-btn-ghost h-8 px-3 text-meta text-ink-secondary hover:text-brand"
                  >
                    {expanded === product.productId ? '收起' : '管理 SKU'}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleShelf(product.productId, product.status !== 1)}
                    disabled={busy === `shelf:${product.productId}`}
                    className="ui-btn-ghost h-8 px-3 text-meta text-ink-secondary disabled:opacity-60"
                  >
                    {product.status === 1 ? '下架' : '上架'}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleDelete(product.productId, product.name)}
                    disabled={busy === `delete:${product.productId}`}
                    className="rounded-full border border-danger/30 bg-card px-3 py-1 text-meta text-danger disabled:opacity-60"
                  >
                    删除
                  </button>
                </div>
              </div>

              {expanded === product.productId && (
                <div className="rounded-sm bg-subtle p-3">
                  {detailLoading && <p className="text-meta text-ink-tertiary">SKU 加载中…</p>}
                  {detail && detail.skus.length === 0 && (
                    <p className="text-meta text-ink-tertiary">该商品还没有 SKU</p>
                  )}
                  {detail && detail.skus.length > 0 && (
                    <ul className="space-y-2">
                      {detail.skus.map((sku) => (
                        <li key={sku.skuId} className="flex flex-wrap items-center gap-2 text-meta">
                          <span className="min-w-24 text-ink-secondary">{sku.name}</span>
                          <span className="text-ink-tertiary">库存 {sku.availableStock}</span>
                          <label className="flex items-center gap-1 text-ink-tertiary">
                            ¥
                            <input
                              value={priceDraft[sku.skuId] ?? ''}
                              onChange={(event) =>
                                setPriceDraft((prev) => ({ ...prev, [sku.skuId]: event.target.value }))
                              }
                              className="h-8 w-20 rounded-md border border-line bg-card px-2 text-ink"
                            />
                          </label>
                          <button
                            type="button"
                            onClick={() => void saveSkuPrice(sku.skuId)}
                            disabled={busy === `price:${sku.skuId}`}
                            className="rounded-full border border-line bg-card px-2.5 py-1 text-meta text-ink-secondary disabled:opacity-60"
                          >
                            改价
                          </button>
                          <label className="flex items-center gap-1 text-ink-tertiary">
                            补货
                            <input
                              value={stockDraft[sku.skuId] ?? '0'}
                              onChange={(event) =>
                                setStockDraft((prev) => ({ ...prev, [sku.skuId]: event.target.value }))
                              }
                              className="h-8 w-16 rounded-md border border-line bg-card px-2 text-ink"
                            />
                          </label>
                          <button
                            type="button"
                            onClick={() => void addStock(sku.skuId)}
                            disabled={busy === `stock:${sku.skuId}`}
                            className="rounded-full border border-line bg-card px-2.5 py-1 text-meta text-ink-secondary disabled:opacity-60"
                          >
                            加库存
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
