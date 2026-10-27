import { get, request } from '@/shared/api/http';
import type { Product, ProductDTO, ProductPageDTO, ProductDetail, Sku } from './types';

function toProduct(dto: ProductDTO): Product {
  return {
    id: Number(dto.id ?? 0),
    productId: dto.productId ?? '',
    uid: dto.uid ?? '',
    name: dto.name ?? '未命名商品',
    description: dto.description ?? '',
    coverUrl: dto.coverUrl ?? '',
    productType: dto.productType ?? 1,
    status: dto.status ?? 0,
    statusName: dto.statusName ?? '未知',
    minPrice: dto.minPrice ?? null,
    skuCount: dto.skuCount ?? 0,
  };
}

function toSku(dto: {
  id?: number;
  skuId?: string;
  name?: string;
  price?: number;
  status?: number;
  availableStock?: number;
}): Sku {
  return {
    id: Number(dto.id ?? 0),
    skuId: dto.skuId ?? '',
    name: dto.name ?? '默认规格',
    price: dto.price ?? 0,
    status: dto.status ?? 0,
    availableStock: dto.availableStock ?? 0,
  };
}

export interface ProductListResult {
  items: Product[];
  total: number;
  page: number;
  pageSize: number;
}

export async function fetchProducts(params: {
  page?: number;
  pageSize?: number;
  keyword?: string;
}): Promise<ProductListResult> {
  const dto = await get<ProductPageDTO>('/products', {
    params: {
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 12,
      keyword: params.keyword || undefined,
    },
  });
  const raw = dto.items ?? dto.list ?? [];
  return {
    items: raw.map(toProduct),
    total: dto.total ?? raw.length,
    page: dto.page ?? 1,
    pageSize: dto.pageSize ?? dto.size ?? raw.length,
  };
}

/**
 * 商品详情（含 SKU 列表）。
 *
 * 后端详情接口 `id` 同时接受内部主键与业务编号，前端统一用 `productId`（P...）。
 */
export async function fetchProductDetail(id: string): Promise<ProductDetail> {
  const dto = await get<{
    product?: ProductDTO;
    skus?: Array<{
      id?: number;
      skuId?: string;
      name?: string;
      price?: number;
      status?: number;
      availableStock?: number;
    }>;
  }>(`/products/${id}`);

  return {
    product: toProduct(dto.product ?? {}),
    skus: (dto.skus ?? []).map(toSku).filter((sku) => sku.skuId),
  };
}

/** 我的商品（含草稿/下架）。 */
export async function fetchMyProducts(pageSize = 50): Promise<Product[]> {
  const dto = await get<ProductPageDTO>('/products/mine', {
    params: { page: 1, pageSize },
  });
  const raw = dto.items ?? dto.list ?? [];
  return raw.map(toProduct);
}

export interface CreateProductInput {
  name: string;
  description?: string;
  coverUrl?: string;
  productType?: number;
}

export async function createProduct(input: CreateProductInput): Promise<ProductDetail> {
  const dto = await request<{
    product?: ProductDTO;
    skus?: Array<Parameters<typeof toSku>[0]>;
  }>({
    method: 'POST',
    url: '/products',
    data: {
      name: input.name,
      description: input.description ?? '',
      coverUrl: input.coverUrl ?? '',
      productType: input.productType ?? 1,
    },
  });
  return {
    product: toProduct(dto.product ?? {}),
    skus: (dto.skus ?? []).map(toSku).filter((sku) => sku.skuId),
  };
}

export interface CreateSkuInput {
  name: string;
  /** 单位：分 */
  price: number;
  initialStock: number;
}

export async function createSku(productId: string, input: CreateSkuInput): Promise<Sku> {
  const dto = await request<Parameters<typeof toSku>[0]>({
    method: 'POST',
    url: `/products/${productId}/skus`,
    data: input,
  });
  return toSku(dto);
}

export async function updateProduct(
  productId: string,
  input: { name?: string; description?: string; coverUrl?: string; productType?: number },
): Promise<void> {
  await request({ method: 'PUT', url: `/products/${productId}`, data: input });
}

export async function deleteProduct(productId: string): Promise<void> {
  await request<void>({ method: 'DELETE', url: `/products/${productId}` });
}

export async function onShelfProduct(productId: string): Promise<void> {
  await request({ method: 'POST', url: `/products/${productId}/on-shelf` });
}

export async function offShelfProduct(productId: string): Promise<void> {
  await request({ method: 'POST', url: `/products/${productId}/off-shelf` });
}

/** 改价：skuId 支持业务编号 S... */
export async function updateSkuPrice(skuId: string, price: number): Promise<void> {
  await request({ method: 'PUT', url: `/skus/${skuId}/price`, data: { price } });
}

/** 增加库存。 */
export async function increaseSkuStock(skuId: string, quantity: number): Promise<void> {
  await request({ method: 'POST', url: `/skus/${skuId}/stock/increase`, data: { quantity } });
}

/** 启用/禁用 SKU（1 启用，0 禁用）。 */
export async function updateSkuStatus(skuId: string, status: 0 | 1): Promise<void> {
  await request({ method: 'PUT', url: `/skus/${skuId}`, data: { status } });
}
