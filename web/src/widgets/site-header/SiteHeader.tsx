import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useState } from 'react';
import { useAuthStore } from '@/features/auth/store';
import { useThemeStore } from '@/shared/store/theme';
import { Icon, type IconName } from '@/shared/ui/Icon';
import { Avatar } from '@/shared/ui/Avatar';

interface NavItem {
  to: string;
  label: string;
  icon: IconName;
  exact?: boolean;
}

/** 顶部导航：60px 吸顶，白底 + 1px 边框；当前项用浅灰胶囊标识。 */
export function SiteHeader() {
  const navigate = useNavigate();
  const location = useLocation();
  const [keyword, setKeyword] = useState('');

  const status = useAuthStore((state) => state.status);
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const theme = useThemeStore((state) => state.theme);
  const toggleTheme = useThemeStore((state) => state.toggle);

  const returnTo = encodeURIComponent(location.pathname + location.search);

  const items: NavItem[] = [
    { to: '/', label: '首页', icon: 'home', exact: true },
    { to: '/streams', label: '直播', icon: 'live' },
    { to: '/shop', label: '商城', icon: 'shop' },
    ...(status === 'authed'
      ? [
          { to: '/orders', label: '订单', icon: 'order' as IconName },
          { to: '/streams/manage', label: '开播', icon: 'live' as IconName },
          ...(user?.role === 'ADMIN'
            ? [{ to: '/admin', label: '管理', icon: 'settings' as IconName }]
            : []),
        ]
      : []),
  ];

  function isActive(item: NavItem): boolean {
    return item.exact ? location.pathname === item.to : location.pathname.startsWith(item.to);
  }

  function submit(event: React.FormEvent) {
    event.preventDefault();
    const q = keyword.trim();
    if (q) navigate(`/search?q=${encodeURIComponent(q)}`);
  }

  async function handleLogout() {
    await logout();
    navigate('/', { replace: true });
  }

  return (
    <header className="sticky top-0 z-30 border-b border-line bg-card/95 backdrop-blur">
      <div className="mx-auto flex h-header max-w-content items-center gap-3 px-4 sm:px-6">
        <Link to="/" className="shrink-0 text-base font-semibold text-ink lg:hidden">
          视频社区
        </Link>

        <nav className="hidden items-center gap-1 lg:flex">
          {items.map((item) => (
            <Link
              key={item.to}
              to={item.to}
              className={`inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-sm transition-colors ${
                isActive(item)
                  ? 'bg-subtle font-medium text-ink'
                  : 'text-ink-secondary hover:bg-subtle hover:text-ink'
              }`}
            >
              <Icon name={item.icon} size={16} />
              {item.label}
            </Link>
          ))}
        </nav>

        <form onSubmit={submit} className="relative ml-auto w-full max-w-[420px]" role="search">
          <label className="sr-only" htmlFor="site-search">
            搜索视频
          </label>
          <Icon
            name="search"
            size={16}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-tertiary"
          />
          <input
            id="site-search"
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="搜索视频"
            className="h-9 w-full rounded-full border border-transparent bg-subtle pl-9 pr-4 text-sm text-ink outline-none transition-colors placeholder:text-ink-tertiary focus:border-brand focus:bg-card"
          />
        </form>

        <div className="flex shrink-0 items-center gap-1">
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'}
            className="grid h-9 w-9 place-items-center rounded-full text-ink-secondary transition-colors hover:bg-subtle hover:text-ink"
          >
            <Icon name={theme === 'dark' ? 'sun' : 'moon'} size={18} />
          </button>

          {status === 'authed' ? (
            <>
              <Link to="/upload" className="ui-btn-primary hidden h-9 sm:inline-flex">
                <Icon name="plus" size={16} />
                投稿
              </Link>
              <Link
                to="/space"
                className="ml-1 flex items-center gap-2"
                aria-label={user?.nickname ?? '我的空间'}
              >
                <Avatar
                  src={user?.avatar}
                  alt={user?.nickname ?? '我的头像'}
                  className="h-8 w-8"
                />
              </Link>
              <button
                type="button"
                onClick={() => void handleLogout()}
                aria-label="退出登录"
                title="退出登录"
                className="grid h-9 w-9 place-items-center rounded-full text-ink-tertiary transition-colors hover:bg-subtle hover:text-ink"
              >
                <Icon name="logout" size={17} />
              </button>
            </>
          ) : (
            <Link
              to={`/login?returnTo=${returnTo}`}
              className="ml-1 inline-flex h-9 items-center rounded-full bg-brand px-4 text-sm font-medium text-white transition-colors hover:bg-brand-hover"
            >
              登录
            </Link>
          )}
        </div>
      </div>

      {/* 移动端第二行：横向滚动的胶囊导航 */}
      <nav className="flex gap-1 overflow-x-auto border-t border-divider px-3 py-1.5 lg:hidden">
        {items.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            className={`inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full px-3 text-sm transition-colors ${
              isActive(item)
                ? 'bg-subtle font-medium text-ink'
                : 'text-ink-secondary hover:bg-subtle hover:text-ink'
            }`}
          >
            <Icon name={item.icon} size={15} />
            {item.label}
          </Link>
        ))}
      </nav>
    </header>
  );
}
