import type { Page } from '@/shared/api/errors';

export interface Product {
  /** 内部主键（数字），仅用于路由/调试 */
  id: number;
  /** 对外业务编号 P... */
  productId: string;
  uid: string;
  name: string;
  description: string;
  coverUrl: string;
  productType: number;
  status: number;
  statusName: string;
  minPrice: number | null;
  skuCount: number;
}

export interface Sku {
  id: number;
  skuId: string;
  name: string;
  price: number;
  status: number;
  availableStock: number;
}

export interface ProductDetail {
  product: Product;
  skus: Sku[];
}

export interface ProductDTO {
  id?: number;
  productId?: string;
  uid?: string;
  name?: string;
  description?: string;
  coverUrl?: string | null;
  productType?: number;
  status?: number;
  statusName?: string;
  minPrice?: number | null;
  skuCount?: number | null;
}

export interface SkuDTO {
  id?: number;
  skuId?: string;
  name?: string;
  price?: number;
  status?: number;
  availableStock?: number;
}

export type ProductPageDTO = Page<ProductDTO> & { list?: ProductDTO[]; size?: number };
