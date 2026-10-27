import { Icon } from '@/shared/ui/Icon';

interface Props {
  title?: string;
  hint?: string;
  action?: React.ReactNode;
}

/** 空状态：列表为空、搜索无结果、区块加载失败统一用它。 */
export function EmptyState({ title = '这里什么都没有', hint, action }: Props) {
  return (
    <div className="grid place-items-center rounded-md border border-line bg-card px-6 py-14 text-center">
      <span className="mb-3 grid h-12 w-12 place-items-center rounded-full bg-subtle text-ink-tertiary">
        <Icon name="message" size={24} />
      </span>
      <p className="text-body font-medium text-ink">{title}</p>
      {hint && <p className="mt-1 max-w-md text-meta text-ink-tertiary">{hint}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

/** 骨架屏：保持与卡片一致的 16:9，避免 CLS。 */
export function CardSkeleton() {
  return (
    <div>
      <div className="aspect-video w-full animate-pulse rounded-md bg-subtle" />
      <div className="mt-2.5 space-y-2">
        <div className="h-4 w-4/5 animate-pulse rounded-xs bg-subtle" />
        <div className="h-3 w-1/2 animate-pulse rounded-xs bg-subtle" />
      </div>
    </div>
  );
}

export function CardGridSkeleton({ count = 8 }: { count?: number }) {
  return (
    <div className="grid grid-cols-1 gap-x-5 gap-y-7 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {Array.from({ length: count }, (_, index) => (
        <CardSkeleton key={index} />
      ))}
    </div>
  );
}
