import { Outlet } from 'react-router-dom';
import { SiteHeader } from '@/widgets/site-header/SiteHeader';

export function SiteLayout() {
  return (
    <div className="flex min-h-full flex-col bg-page">
      <SiteHeader />
      <main className="mx-auto w-full max-w-content flex-1 px-4 py-6 sm:px-6">
        <Outlet />
      </main>
      <footer className="border-t border-line py-6 text-center text-meta text-ink-tertiary">
        演示项目 · 数据来自本地后端
      </footer>
    </div>
  );
}
