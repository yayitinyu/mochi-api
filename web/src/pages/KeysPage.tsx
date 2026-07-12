import { useEffect, useState } from 'react';
import { CheckIcon, CopyIcon, KeyIcon, PlusIcon, TrashIcon } from '@phosphor-icons/react';
import { api, ApiError } from '../lib/api';
import type { Token } from '../lib/types';
import { formatTime } from '../lib/format';
import { Button } from '../components/Button';
import { Card } from '../components/Card';
import { Field, Input } from '../components/Field';
import { Modal } from '../components/Modal';
import { StatusBadge } from '../components/Badge';
import { useToast } from '../components/Toast';

export function KeysPage() {
  const toast = useToast();
  const [tokens, setTokens] = useState<Token[] | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState('');
  const [createdKey, setCreatedKey] = useState('');
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);

  async function load() {
    setTokens((await api.get<Token[]>('/api/tokens')) ?? []);
  }
  useEffect(() => {
    void load().catch(() => toast('error', '加载失败'));
  }, []);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await api.post<{ key: string }>('/api/tokens', { name });
      setCreatedKey(res.key);
      setName('');
      await load();
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '创建失败');
    } finally {
      setBusy(false);
    }
  }

  async function toggle(t: Token) {
    try {
      await api.put(`/api/tokens/${t.id}`, { status: t.status === 1 ? 2 : 1 });
      await load();
    } catch {
      toast('error', '操作失败');
    }
  }

  async function remove(t: Token) {
    if (!window.confirm(`确定删除密钥「${t.name}」吗？此操作不可撤销。`)) return;
    try {
      await api.del(`/api/tokens/${t.id}`);
      toast('success', '已删除');
      await load();
    } catch {
      toast('error', '删除失败');
    }
  }

  async function copyKey() {
    await navigator.clipboard.writeText(createdKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  return (
    <div className="max-w-4xl">
      <div className="mb-4 flex justify-end">
        <Button onClick={() => setCreateOpen(true)}>
          <PlusIcon size={16} weight="bold" /> 新建密钥
        </Button>
      </div>

      <Card className="p-0">
        {tokens === null ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : tokens.length === 0 ? (
          <div className="flex flex-col items-center gap-3 p-12 text-center">
            <div className="grid h-14 w-14 place-items-center rounded-3xl bg-sakura-100 dark:bg-sakura-500/20">
              <KeyIcon size={26} weight="duotone" className="text-sakura-500" />
            </div>
            <p className="font-bold text-ink">还没有 API 密钥</p>
            <p className="text-sm text-ink-soft">创建一个密钥，就可以开始调用聚合接口啦</p>
            <Button onClick={() => setCreateOpen(true)} className="mt-1">
              <PlusIcon size={16} weight="bold" /> 新建密钥
            </Button>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs font-bold text-ink-soft">
                <th className="px-6 py-4">名称</th>
                <th className="px-4 py-4">密钥</th>
                <th className="px-4 py-4">状态</th>
                <th className="px-4 py-4">创建时间</th>
                <th className="px-6 py-4 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {tokens.map((t) => (
                <tr key={t.id} className="border-t border-sakura-50 dark:border-white/5">
                  <td className="px-6 py-3.5 font-bold text-ink">{t.name}</td>
                  <td className="px-4 py-3.5 font-mono text-xs text-ink-soft">{t.key_preview}</td>
                  <td className="px-4 py-3.5">
                    <StatusBadge enabled={t.status === 1} />
                  </td>
                  <td className="px-4 py-3.5 text-ink-soft">{formatTime(t.created_at)}</td>
                  <td className="px-6 py-3.5 text-right">
                    <div className="flex justify-end gap-2">
                      <Button variant="soft" className="px-3 py-1.5 text-xs" onClick={() => toggle(t)}>
                        {t.status === 1 ? '停用' : '启用'}
                      </Button>
                      <Button
                        variant="danger"
                        className="px-3 py-1.5 text-xs"
                        onClick={() => remove(t)}
                        aria-label={`删除 ${t.name}`}
                      >
                        <TrashIcon size={14} weight="bold" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>

      <Modal
        open={createOpen}
        onClose={() => {
          setCreateOpen(false);
          setCreatedKey('');
        }}
        title={createdKey ? '密钥创建成功 🎉' : '新建 API 密钥'}
      >
        {createdKey ? (
          <div className="flex flex-col gap-4">
            <p className="text-sm text-ink-soft">
              完整密钥只显示这一次，请立即复制保存：
            </p>
            <div className="flex items-center gap-2 rounded-2xl bg-sakura-50 p-3 dark:bg-sakura-500/10">
              <code className="flex-1 break-all font-mono text-xs text-ink">{createdKey}</code>
              <Button variant="soft" className="shrink-0 px-3 py-1.5 text-xs" onClick={copyKey}>
                {copied ? <CheckIcon size={14} weight="bold" /> : <CopyIcon size={14} weight="bold" />}
                {copied ? '已复制' : '复制'}
              </Button>
            </div>
            <Button
              onClick={() => {
                setCreateOpen(false);
                setCreatedKey('');
              }}
            >
              我已保存好
            </Button>
          </div>
        ) : (
          <form onSubmit={create} className="flex flex-col gap-4">
            <Field label="密钥名称" hint="例如：Claude Code、我的脚本">
              <Input value={name} onChange={(e) => setName(e.target.value)} required maxLength={50} />
            </Field>
            <Button type="submit" disabled={busy}>
              {busy ? '创建中…' : '创建'}
            </Button>
          </form>
        )}
      </Modal>
    </div>
  );
}
