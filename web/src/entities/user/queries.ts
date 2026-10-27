import { useQuery } from '@tanstack/react-query';
import { qk } from '@/shared/api/query-keys';
import { fetchProfiles } from './api';

/** 批量资料：按 uid 集合缓存 5 分钟，首页/播放页共用同一份。 */
export function useProfiles(uids: string[]) {
  const unique = [...new Set(uids.filter(Boolean))];
  return useQuery({
    queryKey: qk.profiles(unique),
    queryFn: () => fetchProfiles(unique),
    staleTime: 5 * 60_000,
    enabled: unique.length > 0,
  });
}
