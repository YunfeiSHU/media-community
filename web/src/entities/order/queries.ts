import { useQuery } from '@tanstack/react-query';
import { fetchOrder, fetchOrders } from './api';

export function useOrders(params: { page?: number; size?: number; status?: number }) {
  return useQuery({
    queryKey: ['orders', params],
    queryFn: () => fetchOrders(params),
    staleTime: 30_000,
  });
}

export function useOrder(id: string) {
  return useQuery({
    queryKey: ['order', id],
    queryFn: () => fetchOrder(id),
    staleTime: 10_000,
    enabled: Boolean(id),
  });
}
