import { useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '@/features/auth/store';
import { describeError } from '@/shared/api/errors';

export default function LoginPage() {
  const [type, setType] = useState<'PHONE' | 'EMAIL'>('PHONE');
  const [target, setTarget] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const returnTo = params.get('returnTo') || '/';

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(type, target.trim(), password);
      navigate(returnTo, { replace: true });
    } catch (submitError) {
      setError(describeError(submitError));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto max-w-sm rounded-lg border border-line bg-card p-6">
      <h1 className="mb-1 text-h1 font-semibold text-ink">登录</h1>
      <p className="mb-5 text-meta text-ink-tertiary">登录后才能投稿、发弹幕、点赞收藏和下单</p>

      <div className="mb-4 flex rounded-md bg-page p-1 text-sm">
        {(['PHONE', 'EMAIL'] as const).map((item) => (
          <button
            key={item}
            type="button"
            onClick={() => setType(item)}
            className={`flex-1 rounded-sm py-1.5 ${
              type === item ? 'bg-card text-brand shadow-card' : 'text-ink-secondary'
            }`}
          >
            {item === 'PHONE' ? '手机号' : '邮箱'}
          </button>
        ))}
      </div>

      <form onSubmit={submit} className="space-y-3">
        <label className="block">
          <span className="mb-1 block text-meta text-ink-secondary">
            {type === 'PHONE' ? '手机号' : '邮箱'}
          </span>
          <input
            value={target}
            onChange={(event) => setTarget(event.target.value)}
            autoComplete="username"
            placeholder={type === 'PHONE' ? '1xxxxxxxxxx' : 'you@example.com'}
            className="ui-input h-10"
          />
        </label>

        <label className="block">
          <span className="mb-1 block text-meta text-ink-secondary">密码</span>
          <input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="current-password"
            placeholder="8–20 位"
            className="ui-input h-10"
          />
        </label>

        {error && (
          <p role="alert" className="rounded-sm bg-danger/10 px-3 py-2 text-meta text-danger">
            {error}
          </p>
        )}

        <button
          type="submit"
          disabled={submitting || !target.trim() || !password}
          className="ui-btn-primary h-10 w-full"
        >
          {submitting ? '登录中…' : '登录'}
        </button>
      </form>

      <p className="mt-4 text-center text-meta text-ink-tertiary">
        还没有账号？
        <Link to="/register" className="ml-1 text-brand">
          去注册
        </Link>
      </p>
    </div>
  );
}
