interface Props {
  page: number;
  totalPages: number;
  onChange: (page: number) => void;
  className?: string;
}

type PageItem = number | 'ellipsis';

function buildPages(current: number, total: number): PageItem[] {
  const visible = new Set([1, total, current - 1, current, current + 1]);
  const numbers = [...visible].filter((page) => page >= 1 && page <= total).sort((a, b) => a - b);

  const result: PageItem[] = [];
  numbers.forEach((page, index) => {
    if (index > 0 && page - numbers[index - 1] > 1) result.push('ellipsis');
    result.push(page);
  });
  return result;
}

/** 统一分页条：上一页 / 页码 / 下一页；页码过多时用省略号收起中间页。 */
export function Pagination({ page, totalPages, onChange, className = '' }: Props) {
  if (totalPages <= 1) return null;

  return (
    <nav
      className={`flex flex-wrap items-center justify-center gap-1.5 ${className}`}
      aria-label="分页"
    >
      <button
        type="button"
        onClick={() => onChange(page - 1)}
        disabled={page <= 1}
        className="ui-btn-ghost h-8 px-3 text-xs"
      >
        上一页
      </button>

      {buildPages(page, totalPages).map((item, index) =>
        item === 'ellipsis' ? (
          <span key={`ellipsis-${index}`} className="px-1 text-meta text-ink-tertiary">
            …
          </span>
        ) : (
          <button
            key={item}
            type="button"
            onClick={() => onChange(item)}
            aria-current={item === page ? 'page' : undefined}
            className={`h-8 min-w-8 rounded-full px-2 text-xs tabular-nums transition-colors ${
              item === page
                ? 'bg-brand font-medium text-white'
                : 'text-ink-secondary hover:bg-subtle'
            }`}
          >
            {item}
          </button>
        ),
      )}

      <button
        type="button"
        onClick={() => onChange(page + 1)}
        disabled={page >= totalPages}
        className="ui-btn-ghost h-8 px-3 text-xs"
      >
        下一页
      </button>
    </nav>
  );
}
