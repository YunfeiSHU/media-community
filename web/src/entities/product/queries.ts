import { useQuery } from '@tanstack/react-query';
import { fetchProductDetail, fetchProducts } from './api';

export function useProducts(params: { page?: number; pageSize?: number; keyword?: string }) {
  return useQuery({
    queryKey: ['products', params],
    queryFn: () => fetchProducts(params),
    staleTime: 60_000,
  });
}

export function useProductDetail(id: string) {
  return useQuery({
    queryKey: ['product', id],
    queryFn: () => fetchProductDetail(id),
    staleTime: 5 * 60_000,
    enabled: Boolean(id),
  });
}
