import { Link } from 'react-router-dom';
import { EmptyState } from '@/widgets/empty-state/EmptyState';

export function NotFoundPage() {
  return (
    <EmptyState
      title="页面不存在"
      hint="链接可能已经失效"
      action={
        <Link to="/" className="rounded-md bg-brand px-3 py-1.5 text-sm text-white">
          回到首页
        </Link>
      }
    />
  );
}

interface PlaceholderProps {
  title: string;
  description: string;
  stage: string;
}

/**
 * 尚未实现的页面占位。
 *
 * 刻意不放假按钮：导航里有入口，但页面明确写“哪个阶段接”，
 * 比点进去一片空白或一排点不动的按钮更诚实。
 */
export function PlaceholderPage({ title, description, stage }: PlaceholderProps) {
  return (
    <EmptyState
      title={title}
      hint={`${description}（计划在 ${stage} 接入）`}
      action={
        <Link to="/" className="rounded-md border border-line px-3 py-1.5 text-sm text-ink-secondary">
          回到首页
        </Link>
      }
    />
  );
}
