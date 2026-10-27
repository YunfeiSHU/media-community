import { get, request } from '@/shared/api/http';
import { parseServerDate } from '@/shared/lib/datetime';

export interface Profile {
  uid: string;
  nickname: string;
  avatar: string;
  bio: string;
  gender: string;
  followerCount: number;
  followingCount: number;
  videoCount: number;
  createdAt: Date | null;
}

interface ProfileDTO {
  uid?: string;
  nickname?: string;
  avatar?: string | null;
  bio?: string | null;
  gender?: string | null;
  followerCount?: number;
  followingCount?: number;
  videoCount?: number;
  createdAt?: string | null;
}

function toProfile(dto: ProfileDTO): Profile {
  return {
    uid: dto.uid ?? '',
    nickname: dto.nickname ?? '未知用户',
    avatar: dto.avatar ?? '',
    bio: dto.bio ?? '',
    gender: dto.gender ?? '',
    followerCount: dto.followerCount ?? 0,
    followingCount: dto.followingCount ?? 0,
    videoCount: dto.videoCount ?? 0,
    createdAt: parseServerDate(dto.createdAt),
  };
}

/** 批量查公开资料：一次拿一屏作者，避免每个卡片各发一次请求。 */
export async function fetchProfiles(uids: string[]): Promise<Profile[]> {
  const unique = [...new Set(uids.filter(Boolean))];
  if (unique.length === 0) return [];

  const dto = await get<{ items?: ProfileDTO[] }>('/profiles', {
    // 后端上限 100，超出会被 40001 拒绝；这里按 100 分批
    params: { uids: unique.slice(0, 100).join(',') },
  });
  return (dto.items ?? []).map(toProfile);
}

export interface MeProfile extends Profile {
  role: 'ADMIN' | 'user';
}

export async function fetchMe(): Promise<MeProfile> {
  const dto = await get<ProfileDTO & { role?: string }>('/users/me');
  return {
    ...toProfile(dto),
    role: dto.role === 'ADMIN' ? 'ADMIN' : 'user',
  };
}

export interface FollowState {
  following: boolean;
  followerCount: number;
}

/** 关注 / 取关（幂等）。 */
export function setFollowing(uid: string, following: boolean): Promise<FollowState> {
  return request<FollowState>({
    method: following ? 'POST' : 'DELETE',
    url: `/users/${uid}/follow`,
  });
}

/** 当前用户对目标用户的关注状态；未登录不要调用（会 401）。 */
export function fetchRelationship(uid: string): Promise<FollowState> {
  return get<FollowState>(`/users/${uid}/relationship`);
}

export interface UpdateProfileInput {
  nickname?: string;
  avatar?: string;
  gender?: string;
  bio?: string;
}

/** 修改自己的资料（PUT /users/me），返回更新后的完整资料。 */
export async function updateMe(input: UpdateProfileInput): Promise<MeProfile> {
  await request({
    method: 'PUT',
    url: '/users/me',
    data: input,
  });
  return fetchMe();
}

interface ProfilePageDTO {
  items?: ProfileDTO[];
  total?: number;
}

export interface ProfilePage {
  items: Profile[];
  total: number;
}

export async function fetchFollowers(
  uid: string,
  page = 1,
  pageSize = 20,
): Promise<ProfilePage> {
  const dto = await get<ProfilePageDTO>(`/users/${uid}/followers`, {
    params: { page, pageSize },
  });
  return { items: (dto.items ?? []).map(toProfile), total: dto.total ?? 0 };
}

export async function fetchFollowing(
  uid: string,
  page = 1,
  pageSize = 20,
): Promise<ProfilePage> {
  const dto = await get<ProfilePageDTO>(`/users/${uid}/following`, {
    params: { page, pageSize },
  });
  return { items: (dto.items ?? []).map(toProfile), total: dto.total ?? 0 };
}
