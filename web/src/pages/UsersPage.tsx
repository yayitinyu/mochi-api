import { useEffect, useState } from 'react';
import {
  ArrowDownIcon,
  ArrowUpIcon,
  PasswordIcon,
  ProhibitIcon,
  PlayIcon,
  TrashIcon,
  UsersIcon,
} from '@phosphor-icons/react';
import { api, ApiError } from '../lib/api';
import {
  ROLE_ADMIN,
  ROLE_USER,
  STATUS_DISABLED,
  STATUS_ENABLED,
  type User,
  type UserStat,
} from '../lib/types';
import { useAuth } from '../context/AuthContext';
import { Button } from '../components/Button';
import { Card } from '../components/Card';
import { Field, Input } from '../components/Field';
import { Modal } from '../components/Modal';
import { StatusBadge } from '../components/Badge';
import { useToast } from '../components/Toast';
import { formatCost, formatNumber } from '../lib/format';

const STAT_RANGES = [7, 30, 90] as const;

export function UsersPage() {
  const toast = useToast();
  const { user: me } = useAuth();
  const [users, setUsers] = useState<User[] | null>(null);
  const [stats, setStats] = useState<UserStat[] | null>(null);
  const [statDays, setStatDays] = useState<number>(30);
  const [resetting, setResetting] = useState<User | null>(null);
  const [newPassword, setNewPassword] = useState('');
  const [busy, setBusy] = useState(false);

  async function load() {
    setUsers((await api.get<User[]>('/api/users')) ?? []);
  }
  useEffect(() => {
    void load().catch(() => toast('error', '加载失败'));
  }, []);

  useEffect(() => {
    setStats(null);
    api
      .get<UserStat[]>(`/api/stats/users?days=${statDays}`)
      .then((s) => setStats(s ?? []))
      .catch(() => toast('error', '消耗统计加载失败'));
  }, [statDays]);

  async function update(u: User, patch: Record<string, unknown>, successMsg: string) {
    try {
      await api.put(`/api/users/${u.id}`, patch);
      toast('success', successMsg);
      await load();
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '操作失败');
    }
  }

  async function toggleStatus(u: User) {
    const disabling = u.status === STATUS_ENABLED;
    if (disabling && !window.confirm(`确定禁用用户「${u.username}」吗？其所有 API 密钥将立即失效。`)) {
      return;
    }
    await update(
      u,
      { status: disabling ? STATUS_DISABLED : STATUS_ENABLED },
      disabling ? '已禁用' : '已启用',
    );
  }

  async function toggleRole(u: User) {
    const promoting = u.role < ROLE_ADMIN;
    if (!window.confirm(promoting ? `确定将「${u.username}」提升为管理员吗？` : `确定将「${u.username}」降级为普通用户吗？`)) {
      return;
    }
    await update(u, { role: promoting ? ROLE_ADMIN : ROLE_USER }, promoting ? '已提升为管理员' : '已降级为普通用户');
  }

  async function remove(u: User) {
    if (!window.confirm(`确定删除用户「${u.username}」吗？其所有 API 密钥将一并删除，调用日志保留。`)) {
      return;
    }
    try {
      await api.del(`/api/users/${u.id}`);
      toast('success', '已删除');
      await load();
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '删除失败');
    }
  }

  async function resetPassword(e: React.FormEvent) {
    e.preventDefault();
    if (!resetting) return;
    setBusy(true);
    try {
      await api.put(`/api/users/${resetting.id}`, { password: newPassword });
      toast('success', `已重置「${resetting.username}」的密码`);
      setResetting(null);
      setNewPassword('');
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '重置失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex max-w-5xl flex-col gap-6">
      <Card className="p-0">
        {users === null ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs font-bold text-ink-soft">
                  <th className="px-6 py-4 whitespace-nowrap">用户</th>
                  <th className="px-4 py-4 whitespace-nowrap">角色</th>
                  <th className="px-4 py-4 whitespace-nowrap">状态</th>
                  <th className="px-4 py-4 whitespace-nowrap">注册时间</th>
                  <th className="px-6 py-4 text-right whitespace-nowrap">操作</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => {
                  const self = u.id === me?.id;
                  const admin = u.role >= ROLE_ADMIN;
                  const enabled = u.status === STATUS_ENABLED;
                  return (
                    <tr key={u.id} className="border-t border-sakura-50 dark:border-white/5">
                      <td className="px-6 py-3.5">
                        <div className="font-bold text-ink">
                          {u.username}
                          {self && <span className="ml-1.5 text-xs font-bold text-sakura-500">(我)</span>}
                        </div>
                        <div className="text-xs text-ink-soft">#{u.id}</div>
                      </td>
                      <td className="px-4 py-3.5">
                        <span
                          className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold ${
                            admin ? 'bg-sakura-100 text-sakura-600 dark:bg-sakura-500/20 dark:text-sakura-200' : 'bg-sky/15 text-sky'
                          }`}
                        >
                          {admin ? '管理员' : '普通用户'}
                        </span>
                      </td>
                      <td className="px-4 py-3.5">
                        <StatusBadge enabled={enabled} />
                      </td>
                      <td className="px-4 py-3.5 text-ink-soft">
                        {new Date(u.created_at * 1000).toLocaleDateString()}
                      </td>
                      <td className="px-6 py-3.5 text-right">
                        <div className="flex justify-end gap-2">
                          <Button
                            variant="soft"
                            className="px-3 py-1.5 text-xs"
                            onClick={() => {
                              setResetting(u);
                              setNewPassword('');
                            }}
                            aria-label={`重置 ${u.username} 的密码`}
                          >
                            <PasswordIcon size={14} weight="bold" />
                            重置密码
                          </Button>
                          {!self && (
                            <>
                              <Button
                                variant="soft"
                                className="px-3 py-1.5 text-xs"
                                onClick={() => toggleRole(u)}
                                aria-label={admin ? `降级 ${u.username}` : `提升 ${u.username}`}
                              >
                                {admin ? <ArrowDownIcon size={14} weight="bold" /> : <ArrowUpIcon size={14} weight="bold" />}
                                {admin ? '降级' : '升管理员'}
                              </Button>
                              <Button
                                variant="soft"
                                className="px-3 py-1.5 text-xs"
                                onClick={() => toggleStatus(u)}
                                aria-label={enabled ? `禁用 ${u.username}` : `启用 ${u.username}`}
                              >
                                {enabled ? <ProhibitIcon size={14} weight="bold" /> : <PlayIcon size={14} weight="bold" />}
                                {enabled ? '禁用' : '启用'}
                              </Button>
                              <Button
                                variant="danger"
                                className="px-3 py-1.5 text-xs"
                                onClick={() => remove(u)}
                                aria-label={`删除 ${u.username}`}
                              >
                                <TrashIcon size={14} weight="bold" />
                              </Button>
                            </>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <Card className="p-0">
        <div className="flex flex-wrap items-center justify-between gap-2 px-6 pt-5 pb-2">
          <h2 className="flex items-center gap-2 text-sm font-extrabold text-ink">
            <UsersIcon size={18} weight="duotone" className="text-sakura-500" />
            用户消耗排行
          </h2>
          <div className="flex rounded-full bg-sakura-50 p-1 text-xs font-bold dark:bg-sakura-500/10">
            {STAT_RANGES.map((d) => (
              <button
                key={d}
                onClick={() => setStatDays(d)}
                className={`rounded-full px-3 py-1 transition ${
                  statDays === d ? 'bg-sakura-500 text-white shadow-sm' : 'text-ink-soft'
                }`}
              >
                {d} 天
              </button>
            ))}
          </div>
        </div>
        {stats === null ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : stats.length === 0 ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">该时段内还没有任何调用</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs font-bold text-ink-soft">
                  <th className="px-6 py-3 whitespace-nowrap">用户</th>
                  <th className="px-4 py-3 whitespace-nowrap">请求数</th>
                  <th className="px-4 py-3 whitespace-nowrap">输入 tokens</th>
                  <th className="px-4 py-3 whitespace-nowrap">输出 tokens</th>
                  <th className="px-6 py-3 text-right whitespace-nowrap">费用</th>
                </tr>
              </thead>
              <tbody>
                {stats.map((s) => (
                  <tr key={s.user_id} className="border-t border-sakura-50 dark:border-white/5">
                    <td className="px-6 py-3 font-bold text-ink">
                      {s.username || `已注销 #${s.user_id}`}
                    </td>
                    <td className="px-4 py-3 font-mono text-ink-soft">{formatNumber(s.requests)}</td>
                    <td className="px-4 py-3 font-mono text-ink-soft">{formatNumber(s.prompt_tokens)}</td>
                    <td className="px-4 py-3 font-mono text-ink-soft">{formatNumber(s.completion_tokens)}</td>
                    <td className="px-6 py-3 text-right font-mono font-bold text-ink">{formatCost(s.cost_micros)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <Modal
        open={resetting !== null}
        onClose={() => setResetting(null)}
        title={`重置「${resetting?.username ?? ''}」的密码`}
      >
        <form onSubmit={resetPassword} className="flex flex-col gap-4" autoComplete="off">
          <Field label="新密码" hint="8-64 位字符">
            <Input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="请输入新密码"
              autoComplete="new-password"
              minLength={8}
              maxLength={64}
              required
            />
          </Field>
          <Button type="submit" disabled={busy} className="mt-1">
            {busy ? '重置中…' : '确认重置'}
          </Button>
        </form>
      </Modal>
    </div>
  );
}
