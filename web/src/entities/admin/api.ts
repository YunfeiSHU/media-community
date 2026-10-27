import { get, request } from '@/shared/api/http';

export interface AdminWord {
  id: number;
  word: string;
  strategy: string;
  enabled: boolean;
}

export type WordStrategy = 'REJECT' | 'REPLACE' | 'LOG';

export async function fetchSensitiveWords(): Promise<AdminWord[]> {
  const dto = await get<{ items?: AdminWord[] }>('/danmaku/admin/words');
  return (dto.items ?? []).map((word) => ({
    id: Number(word.id ?? 0),
    word: word.word ?? '',
    strategy: word.strategy ?? 'REJECT',
    enabled: Boolean(word.enabled),
  }));
}

export async function createSensitiveWord(input: {
  word: string;
  strategy: WordStrategy;
  enabled: boolean;
}): Promise<void> {
  await request({
    method: 'POST',
    url: '/danmaku/admin/words',
    data: { word: input.word, strategy: input.strategy, enabled: input.enabled },
  });
}

export async function reloadSensitiveWords(): Promise<void> {
  await request({ method: 'POST', url: '/danmaku/admin/reload-words' });
}

export async function createCategory(name: string, sort: number): Promise<void> {
  await request({ method: 'POST', url: '/categories', data: { name, sort } });
}

export async function forceCloseRoom(roomId: string): Promise<void> {
  await request({ method: 'POST', url: `/live/rooms/${roomId}/force-close` });
}

export interface AdminUser {
  userId: number;
  uid: string;
  nickname: string;
  status: string;
  role: string;
  createdAt: string;
}

export async function lookupUser(userId: string): Promise<AdminUser> {
  return get<AdminUser>(`/admin/users/${userId}`);
}
