import { useSearchParams } from 'react-router-dom';
import { useProfiles } from '@/entities/user/queries';
import { useVideos } from '@/entities/video/queries';
import { CardGridSkeleton, EmptyState } from '@/widgets/empty-state/EmptyState';
import { VideoCard } from '@/widgets/video-card/VideoCard';

/** 搜索页：MVP 只搜视频，输入框与文案都不暗示“全站搜索”。 */
export default function SearchPage() {
  const [params] = useSearchParams();
  const keyword = (params.get('q') ?? '').trim();

  const videosQuery = useVideos({ page: 1, pageSize: 24, keyword });
  const profilesQuery = useProfiles((videosQuery.data?.items ?? []).map((video) => video.ownerId));

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-h1 font-semibold text-ink">
          {keyword ? `“${keyword}” 的搜索结果` : '搜索视频'}
        </h1>
        {keyword && videosQuery.data && (
          <span className="text-meta text-ink-tertiary">
            共 {videosQuery.data.total} 个结果
          </span>
        )}
      </div>

      {!keyword && <EmptyState title="输入关键词开始搜索" hint="目前只支持按视频标题搜索" />}

      {keyword && videosQuery.isPending && <CardGridSkeleton count={10} />}

      {keyword && videosQuery.isError && (
        <EmptyState
          title="搜索失败"
          hint={(videosQuery.error as Error)?.message}
          action={
            <button type="button" onClick={() => void videosQuery.refetch()} className="ui-btn-primary">
              重试
            </button>
          }
        />
      )}

      {keyword && videosQuery.isSuccess && videosQuery.data.items.length === 0 && (
        <EmptyState title="没有找到相关视频" hint="换个关键词试试" />
      )}

      {keyword && videosQuery.isSuccess && videosQuery.data.items.length > 0 && (
        <div className="grid grid-cols-1 gap-x-5 gap-y-7 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {videosQuery.data.items.map((video) => (
            <VideoCard
              key={video.id}
              video={video}
              ownerName={profilesQuery.data?.find((profile) => profile.uid === video.ownerId)?.nickname}
            />
          ))}
        </div>
      )}
    </div>
  );
}
