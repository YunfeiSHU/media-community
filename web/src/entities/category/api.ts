import { get } from '@/shared/api/http';

export interface Category {
  id: number;
  name: string;
  sort: number;
}

export async function fetchCategories(): Promise<Category[]> {
  const dto = await get<{ items?: Category[] }>('/categories');
  return (dto.items ?? []).sort((a, b) => a.sort - b.sort);
}
