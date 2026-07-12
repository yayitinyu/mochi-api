import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, ApiError } from '../lib/api';
import type { User } from '../lib/types';
import { useAuth } from '../context/AuthContext';
import { Button } from '../components/Button';
import { Field, Input } from '../components/Field';

export function LoginPage() {
  const { setUser } = useAuth();
  const navigate = useNavigate();
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      const path = mode === 'login' ? '/api/auth/login' : '/api/auth/register';
      const user = await api.post<User>(path, { username, password });
      setUser(user);
      navigate('/');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '请求失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid min-h-[100dvh] place-items-center p-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-3 grid h-16 w-16 place-items-center rounded-[1.5rem] bg-sakura-400 text-3xl shadow-lg shadow-sakura-200">
            🌸
          </div>
          <h1 className="text-2xl font-extrabold text-ink">Mochi</h1>
          <p className="text-sm text-ink-soft">柔软又可靠的 API 聚合网关</p>
        </div>

        <div className="rounded-3xl border border-white bg-surface/85 dark:border-white/10 p-6 shadow-xl backdrop-blur">
          <div className="mb-5 flex rounded-full bg-sakura-50 p-1 dark:bg-sakura-500/10">
            {(['login', 'register'] as const).map((m) => (
              <button
                key={m}
                onClick={() => {
                  setMode(m);
                  setError('');
                }}
                className={`flex-1 rounded-full py-2 text-sm font-bold transition ${
                  mode === m ? 'bg-sakura-500 text-white shadow-sm' : 'text-ink-soft'
                }`}
              >
                {m === 'login' ? '登录' : '注册'}
              </button>
            ))}
          </div>

          <form onSubmit={submit} className="flex flex-col gap-4">
            <Field label="用户名">
              <Input
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="请输入用户名"
                autoComplete="username"
                required
              />
            </Field>
            <Field label="密码" hint={mode === 'register' ? '至少 8 位字符' : undefined}>
              <Input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="请输入密码"
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                required
              />
            </Field>

            {error && (
              <div className="rounded-2xl bg-sakura-50 px-4 dark:bg-sakura-500/10 py-2.5 text-sm font-bold text-sakura-600">
                {error}
              </div>
            )}

            <Button type="submit" disabled={busy} className="mt-1 w-full py-2.5">
              {busy ? '请稍候…' : mode === 'login' ? '登录' : '注册并进入'}
            </Button>
          </form>
        </div>

        <p className="mt-4 text-center text-xs text-ink-soft">首位注册的用户将成为管理员</p>
      </div>
    </div>
  );
}
