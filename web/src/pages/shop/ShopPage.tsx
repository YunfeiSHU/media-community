import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useProducts } from '@/entities/product/queries';
import { formatPriceFrom } from '@/shared/lib/money';
import { DefaultCover } from '@/shared/ui/DefaultCover';
import { CardGridSkeleton, EmptyState } from '@/widgets/empty-state/EmptyState';

export default function ShopPage() {
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const productsQuery = useProducts({ page, pageSize: 15, keyword });

  const total = productsQuery.data?.total ?? 0;
  const pageSize = productsQuery.data?.pageSize ?? 15;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-h1 font-semibold text-ink">商城</h1>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setPage(1);
          }}
          className="flex items-center gap-2"
        >
          <input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="搜索商品"
            className="ui-input w-52"
          />
          <button type="submit" className="ui-btn-primary">
            搜索
          </button>
        </form>
      </div>

      {productsQuery.isPending && <CardGridSkeleton count={10} />}

      {productsQuery.isError && (
        <EmptyState
          title="商品加载失败"
          hint={(productsQuery.error as Error)?.message}
          action={
            <button type="button" onClick={() => void productsQuery.refetch()} className="ui-btn-primary">
              重试
            </button>
          }
        />
      )}

      {productsQuery.isSuccess && productsQuery.data.items.length === 0 && (
        <EmptyState title="还没有上架商品" hint="商品需要先在后台创建并上架" />
      )}

      {productsQuery.isSuccess && productsQuery.data.items.length > 0 && (
        <>
          <div className="grid grid-cols-2 gap-x-5 gap-y-7 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
            {productsQuery.data.items.map((product) => (
              <Link
                key={product.productId}
                to={`/shop/${product.productId}`}
                className="group block rounded-md border border-line bg-card p-2 transition-shadow hover:shadow-hover"
              >
                <div className="relative aspect-square overflow-hidden rounded-sm bg-subtle">
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
                      loading="lazy"
                      className="relative h-full w-full object-cover transition-transform duration-200 group-hover:scale-[1.03]"
                      onError={(event) => {
                        event.currentTarget.style.display = 'none';
                      }}
                    />
                  )}
                </div>
                <div className="mt-2.5 space-y-1 px-1 pb-1">
                  <h3 className="line-clamp-2 text-body text-ink group-hover:text-brand">
                    {product.name}
                  </h3>
                  <p className="text-h2 font-semibold tabular-nums text-brand">
                    {formatPriceFrom(product.minPrice)}
                  </p>
                  <p className="text-meta text-ink-tertiary">
                    {product.skuCount > 0 ? `${product.skuCount} 个规格` : '暂无规格'} · {product.statusName}
                  </p>
                </div>
              </Link>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-3">
              <button
                type="button"
                disabled={page <= 1}
                onClick={() => setPage((value) => value - 1)}
                className="ui-btn-ghost h-8 px-3 text-xs"
              >
                上一页
              </button>
              <span className="text-meta tabular-nums text-ink-tertiary">
                {page} / {totalPages}
              </span>
              <button
                type="button"
                disabled={page >= totalPages}
                onClick={() => setPage((value) => value + 1)}
                className="ui-btn-ghost h-8 px-3 text-xs"
              >
                下一页
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
