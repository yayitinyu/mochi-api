import { useEffect, useState } from 'react';
import { CheckIcon, CopyIcon, PlusIcon, TicketIcon, TrashIcon } from '@phosphor-icons/react';
import { api, ApiError } from '../lib/api';
import type { InviteCode, RegisterMode, SiteStatus } from '../lib/types';
import { Button } from '../components/Button';
import { Card } from '../components/Card';
import { Field, Input } from '../components/Field';
import { Modal } from '../components/Modal';
import { useToast } from '../components/Toast';
import { copyText } from '../lib/clipboard';

const REGISTER_MODES: { value: RegisterMode; label: string; description: string }[] = [
  { value: 'open', label: '开放注册', description: '任何人都可以注册账号' },
  { value: 'invite', label: '邀请码注册', description: '注册时必须提供有效的邀请码' },
  { value: 'closed', label: '关闭注册', description: '不再接受新用户注册' },
];

export function SettingsPage() {
  const toast = useToast();
  const [registerMode, setRegisterMode] = useState<RegisterMode | null>(null);
  const [invites, setInvites] = useState<InviteCode[] | null>(null);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [generateCount, setGenerateCount] = useState(1);
  const [busy, setBusy] = useState(false);
  const [copiedId, setCopiedId] = useState(0);

  useEffect(() => {
    api
      .get<SiteStatus>('/api/settings')
      .then((s) => setRegisterMode(s.register_mode))
      .catch(() => toast('error', '设置加载失败'));
    void loadInvites().catch(() => toast('error', '邀请码加载失败'));
  }, []);

  async function loadInvites() {
    setInvites((await api.get<InviteCode[]>('/api/invites')) ?? []);
  }

  async function changeMode(mode: RegisterMode) {
    const previous = registerMode;
    setRegisterMode(mode);
    try {
      await api.put('/api/settings', { register_mode: mode });
      toast('success', '设置已保存');
    } catch (err) {
      setRegisterMode(previous);
      toast('error', err instanceof ApiError ? err.message : '保存失败');
    }
  }

  async function generate(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const created = await api.post<InviteCode[]>('/api/invites', { count: generateCount });
      toast('success', `已生成 ${created?.length ?? 0} 个邀请码`);
      setGenerateOpen(false);
      await loadInvites();
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '生成失败');
    } finally {
      setBusy(false);
    }
  }

  async function removeInvite(code: InviteCode) {
    if (!window.confirm('确定删除这个邀请码吗？')) return;
    try {
      await api.del(`/api/invites/${code.id}`);
      toast('success', '已删除');
      await loadInvites();
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '删除失败');
    }
  }

  async function copy(code: InviteCode) {
    if (await copyText(code.code)) {
      setCopiedId(code.id);
      setTimeout(() => setCopiedId(0), 1500);
    } else {
      toast('error', '复制失败，请手动复制');
    }
  }

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <Card>
        <h2 className="mb-3 text-sm font-extrabold text-ink">注册方式</h2>
        {registerMode === null ? (
          <div className="p-4 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : (
          <div className="flex flex-col gap-2">
            {REGISTER_MODES.map((m) => (
              <label
                key={m.value}
                className={`flex cursor-pointer items-center gap-3 rounded-2xl border px-4 py-3 transition ${
                  registerMode === m.value
                    ? 'border-sakura-300 bg-sakura-50 dark:border-sakura-500/40 dark:bg-sakura-500/10'
                    : 'border-sakura-50 hover:bg-sakura-50/50 dark:border-white/5 dark:hover:bg-sakura-500/5'
                }`}
              >
                <input
                  type="radio"
                  name="register_mode"
                  checked={registerMode === m.value}
                  onChange={() => changeMode(m.value)}
                  className="h-4 w-4 accent-sakura-500"
                />
                <div>
                  <div className="text-sm font-bold text-ink">{m.label}</div>
                  <div className="text-xs text-ink-soft">{m.description}</div>
                </div>
              </label>
            ))}
          </div>
        )}
      </Card>

      <Card className="p-0">
        <div className="flex items-center justify-between px-6 pt-5 pb-2">
          <h2 className="flex items-center gap-2 text-sm font-extrabold text-ink">
            <TicketIcon size={18} weight="duotone" className="text-sakura-500" />
            邀请码
          </h2>
          <Button className="px-3 py-1.5 text-xs" onClick={() => setGenerateOpen(true)}>
            <PlusIcon size={14} weight="bold" /> 生成邀请码
          </Button>
        </div>
        {invites === null ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : invites.length === 0 ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">
            还没有邀请码，点击右上角生成
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs font-bold text-ink-soft">
                  <th className="px-6 py-3 whitespace-nowrap">邀请码</th>
                  <th className="px-4 py-3 whitespace-nowrap">状态</th>
                  <th className="px-4 py-3 whitespace-nowrap">创建时间</th>
                  <th className="px-6 py-3 text-right whitespace-nowrap">操作</th>
                </tr>
              </thead>
              <tbody>
                {invites.map((inv) => {
                  const used = inv.used_by_user_id !== 0;
                  return (
                    <tr key={inv.id} className="border-t border-sakura-50 dark:border-white/5">
                      <td className="px-6 py-3 font-mono text-xs text-ink">{inv.code}</td>
                      <td className="px-4 py-3">
                        {used ? (
                          <span className="inline-flex items-center rounded-full bg-ink-soft/15 px-2.5 py-0.5 text-xs font-bold text-ink-soft">
                            已被 {inv.used_by_username || `#${inv.used_by_user_id}`} 使用
                          </span>
                        ) : (
                          <span className="inline-flex items-center rounded-full bg-matcha/15 px-2.5 py-0.5 text-xs font-bold text-matcha">
                            未使用
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-ink-soft">
                        {new Date(inv.created_at * 1000).toLocaleDateString()}
                      </td>
                      <td className="px-6 py-3 text-right">
                        <div className="flex justify-end gap-2">
                          {!used && (
                            <>
                              <Button
                                variant="soft"
                                className="px-3 py-1.5 text-xs"
                                onClick={() => copy(inv)}
                                aria-label="复制邀请码"
                              >
                                {copiedId === inv.id ? (
                                  <CheckIcon size={14} weight="bold" />
                                ) : (
                                  <CopyIcon size={14} weight="bold" />
                                )}
                                {copiedId === inv.id ? '已复制' : '复制'}
                              </Button>
                              <Button
                                variant="danger"
                                className="px-3 py-1.5 text-xs"
                                onClick={() => removeInvite(inv)}
                                aria-label="删除邀请码"
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

      <Modal open={generateOpen} onClose={() => setGenerateOpen(false)} title="生成邀请码">
        <form onSubmit={generate} className="flex flex-col gap-4">
          <Field label="数量" hint="一次最多生成 100 个，每个邀请码只能使用一次">
            <Input
              type="number"
              min={1}
              max={100}
              value={generateCount}
              onChange={(e) => setGenerateCount(Math.max(1, Math.min(100, Number(e.target.value) || 1)))}
              required
            />
          </Field>
          <Button type="submit" disabled={busy} className="mt-1">
            {busy ? '生成中…' : '生成'}
          </Button>
        </form>
      </Modal>
    </div>
  );
}
