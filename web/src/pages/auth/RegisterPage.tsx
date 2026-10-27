import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { register, sendVerification } from '@/features/auth/api';
import { useAuthStore } from '@/features/auth/store';
import { describeError } from '@/shared/api/errors';

export default function RegisterPage() {
  const [type, setType] = useState<'PHONE' | 'EMAIL'>('PHONE');
  const [target, setTarget] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [devCode, setDevCode] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = window.setTimeout(() => setCooldown((value) => value - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [cooldown]);

  async function sendCode() {
    setError(null);
    try {
      const info = await sendVerification(type, target.trim());
      // 学习模式下后端会把验证码回传，直接填上省一步
      if (info.code) {
        setCode(info.code);
        setDevCode(info.code);
      }
      setCooldown(60);
    } catch (sendError) {
      setError(describeError(sendError));
    }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);

    if (password !== confirm) {
      setError('两次输入的密码不一致');
      return;
    }

    setSubmitting(true);
    try {
      await register(type, target.trim(), code.trim(), password);
      // 注册接口返回 void，用同一账号密码自动登录
      await login(type, target.trim(), password);
      navigate('/', { replace: true });
    } catch (submitError) {
      setError(describeError(submitError));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto max-w-sm rounded-lg border border-line bg-card p-6">
      <h1 className="mb-1 text-h1 font-semibold text-ink">注册</h1>
      <p className="mb-5 text-meta text-ink-tertiary">注册成功后会自动登录</p>

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
        <input
          value={target}
          onChange={(event) => setTarget(event.target.value)}
          placeholder={type === 'PHONE' ? '手机号' : '邮箱'}
          autoComplete="username"
          className="ui-input h-10"
        />

        <div className="flex gap-2">
          <input
            value={code}
            onChange={(event) => setCode(event.target.value)}
            placeholder="6 位验证码"
            inputMode="numeric"
            className="h-10 min-w-0 flex-1 rounded-md border border-line bg-page px-3 text-sm outline-none focus:border-brand"
          />
          <button
            type="button"
            onClick={sendCode}
            disabled={cooldown > 0 || !target.trim()}
            className="h-10 shrink-0 rounded-md border border-line px-3 text-meta text-ink-secondary disabled:opacity-50"
          >
            {cooldown > 0 ? `${cooldown}s 后重发` : '发送验证码'}
          </button>
        </div>

        {devCode && (
          <p className="text-meta text-ink-tertiary">
            开发模式验证码：<span className="font-mono text-brand">{devCode}</span>
          </p>
        )}

        <input
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          placeholder="密码（8–20 位）"
          autoComplete="new-password"
          className="ui-input h-10"
        />
        <input
          type="password"
          value={confirm}
          onChange={(event) => setConfirm(event.target.value)}
          placeholder="再输一次密码"
          autoComplete="new-password"
          className="ui-input h-10"
        />

        {error && (
          <p role="alert" className="rounded-sm bg-danger/10 px-3 py-2 text-meta text-danger">
            {error}
          </p>
        )}

        <button
          type="submit"
          disabled={submitting || !target.trim() || !code.trim() || password.length < 8}
          className="ui-btn-primary h-10 w-full"
        >
          {submitting ? '注册中…' : '注册并登录'}
        </button>
      </form>

      <p className="mt-4 text-center text-meta text-ink-tertiary">
        已有账号？
        <Link to="/login" className="ml-1 text-brand">
          去登录
        </Link>
      </p>
    </div>
  );
}
