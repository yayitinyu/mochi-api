import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, ApiError } from '../lib/api';
import type { RegisterMode, SiteStatus, User } from '../lib/types';
import { useAuth } from '../context/AuthContext';
import { Button } from '../components/Button';
import { Field, Input } from '../components/Field';
import { Logo } from '../components/Logo';

export function LoginPage() {
  const { setUser } = useAuth();
  const navigate = useNavigate();
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [registerMode, setRegisterMode] = useState<RegisterMode>('open');
  const [bootstrapPending, setBootstrapPending] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [inviteCode, setInviteCode] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // Unknown status (e.g. network hiccup) falls back to open; the
    // backend still enforces the real mode on submit.
    api
      .get<SiteStatus>('/api/status')
      .then((s) => {
        setRegisterMode(s.register_mode);
        setBootstrapPending(s.bootstrap_pending);
      })
      .catch(() => {
        setRegisterMode('open');
        setBootstrapPending(false);
      });
  }, []);

  const registerClosed = registerMode === 'closed';
  const effectiveMode = registerClosed ? 'login' : mode;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      const path = effectiveMode === 'login' ? '/api/auth/login' : '/api/auth/register';
      const user = await api.post<User>(path, {
        username,
        password,
        invite_code: inviteCode,
      });
      setUser(user);
      navigate('/');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '请求失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-[100dvh] w-full items-start justify-center overflow-x-hidden px-3 py-6 sm:items-center sm:p-6">
      <div className="w-full min-w-0 max-w-sm">
        <div className="mb-5 text-center sm:mb-6">
          <Logo size={64} className="mx-auto mb-3 drop-shadow-lg" />
          <h1 className="text-2xl font-extrabold text-ink">Mochi</h1>
          <p className="text-sm text-ink-soft">柔软又可靠的 API 聚合网关</p>
        </div>

        <div className="min-w-0 rounded-3xl border border-white bg-surface/85 p-4 shadow-xl backdrop-blur sm:p-6 dark:border-white/10">
          {!registerClosed && (
            <div className="mb-5 flex rounded-full bg-sakura-50 p-1 dark:bg-sakura-500/10">
              {(['login', 'register'] as const).map((m) => (
                <button
                  key={m}
                  onClick={() => {
                    setMode(m);
                    setError('');
                  }}
                  className={`flex-1 rounded-full py-2 text-sm font-bold transition ${
                    effectiveMode === m ? 'bg-sakura-500 text-white shadow-sm' : 'text-ink-soft'
                  }`}
                >
                  {m === 'login' ? '登录' : '注册'}
                </button>
              ))}
            </div>
          )}

          <form onSubmit={submit} className="flex flex-col gap-4">
            <Field label="用户名">
              <Input
                className="w-full min-w-0"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="请输入用户名"
                autoComplete="username"
                required
              />
            </Field>
            <Field label="密码" hint={effectiveMode === 'register' ? '至少 8 位字符' : undefined}>
              <Input
                className="w-full min-w-0"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="请输入密码"
                autoComplete={effectiveMode === 'login' ? 'current-password' : 'new-password'}
                required
              />
            </Field>
            {effectiveMode === 'register' && registerMode === 'invite' && (
              <Field label="邀请码" hint="本站开启了邀请码注册，请向管理员索取">
                <Input
                  className="w-full min-w-0"
                  value={inviteCode}
                  onChange={(e) => setInviteCode(e.target.value)}
                  placeholder="请输入邀请码"
                  autoComplete="off"
                  required
                />
              </Field>
            )}

            {error && (
              <div className="rounded-2xl bg-sakura-50 px-4 dark:bg-sakura-500/10 py-2.5 text-sm font-bold text-sakura-600">
                {error}
              </div>
            )}

            <Button type="submit" disabled={busy} className="mt-1 w-full py-2.5">
              {busy ? '请稍候…' : effectiveMode === 'login' ? '登录' : '注册并进入'}
            </Button>
          </form>
        </div>

        {registerClosed ? (
          <p className="mt-4 text-center text-xs text-ink-soft">本站已关闭注册</p>
        ) : bootstrapPending ? (
          <p className="mt-4 text-center text-xs text-ink-soft">首位注册的用户将成为管理员</p>
        ) : null}
      </div>
    </div>
  );
}
