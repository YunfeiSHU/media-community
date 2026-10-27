import { useQuery } from '@tanstack/react-query';
import { qk } from '@/shared/api/query-keys';
import { fetchCategories } from './api';

export function useCategories() {
  return useQuery({
    queryKey: qk.categories,
    queryFn: fetchCategories,
    staleTime: 10 * 60_000,
  });
}
