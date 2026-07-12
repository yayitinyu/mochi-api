import { useEffect, useState } from 'react';
import {
  DownloadSimpleIcon,
  LightningIcon,
  PencilSimpleIcon,
  PlusIcon,
  PlugsConnectedIcon,
  TrashIcon,
} from '@phosphor-icons/react';
import { api, ApiError } from '../lib/api';
import type { Channel } from '../lib/types';
import { Button } from '../components/Button';
import { Card } from '../components/Card';
import { Field, Input, Select } from '../components/Field';
import { Modal } from '../components/Modal';
import { StatusBadge } from '../components/Badge';
import { ModelIcon, ProviderIcon } from '../components/ModelIcon';
import { useToast } from '../components/Toast';

interface Form {
  name: string;
  type: string;
  base_url: string;
  api_key: string;
  models: string;
  priority: number;
  status: number;
}

const empty: Form = {
  name: '',
  type: 'openai',
  base_url: '',
  api_key: '',
  models: '',
  priority: 0,
  status: 1,
};

export function ChannelsPage() {
  const toast = useToast();
  const [channels, setChannels] = useState<Channel[] | null>(null);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Channel | null>(null);
  const [form, setForm] = useState<Form>(empty);
  const [busy, setBusy] = useState(false);
  const [testingId, setTestingId] = useState(0);
  const [fetchingModels, setFetchingModels] = useState(false);

  async function load() {
    setChannels(await api.get<Channel[]>('/api/channels'));
  }
  useEffect(() => {
    void load().catch(() => toast('error', '加载失败'));
  }, []);

  function openCreate() {
    setEditing(null);
    setForm(empty);
    setOpen(true);
  }
  function openEdit(ch: Channel) {
    setEditing(ch);
    setForm({ ...ch, api_key: '' });
    setOpen(true);
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      if (editing) {
        await api.put(`/api/channels/${editing.id}`, form);
        toast('success', '已更新');
      } else {
        await api.post('/api/channels', form);
        toast('success', '已创建');
      }
      setOpen(false);
      await load();
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '保存失败');
    } finally {
      setBusy(false);
    }
  }

  async function remove(ch: Channel) {
    if (!window.confirm(`确定删除渠道「${ch.name}」吗？`)) return;
    try {
      await api.del(`/api/channels/${ch.id}`);
      toast('success', '已删除');
      await load();
    } catch {
      toast('error', '删除失败');
    }
  }

  async function testChannel(ch: Channel) {
    setTestingId(ch.id);
    try {
      const res = await api.post<{ latency_ms: number; model_count: number }>(
        `/api/channels/${ch.id}/test`,
      );
      toast('success', `连通正常 · ${res.latency_ms}ms · 上游共 ${res.model_count} 个模型`);
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '连接失败');
    } finally {
      setTestingId(0);
    }
  }

  async function fetchModels() {
    if (!form.base_url) {
      toast('error', '请先填写 Base URL');
      return;
    }
    setFetchingModels(true);
    try {
      const res = await api.post<{ models: string[] }>('/api/channels/fetch_models', {
        type: form.type,
        base_url: form.base_url,
        api_key: form.api_key,
        channel_id: editing?.id ?? 0,
      });
      if (res.models.length === 0) {
        toast('error', '上游没有返回任何模型');
      } else {
        setForm({ ...form, models: res.models.join(', ') });
        toast('success', `已获取 ${res.models.length} 个模型`);
      }
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '获取失败');
    } finally {
      setFetchingModels(false);
    }
  }

  return (
    <div className="max-w-5xl">
      <div className="mb-4 flex justify-end">
        <Button onClick={openCreate}>
          <PlusIcon size={16} weight="bold" /> 新建渠道
        </Button>
      </div>

      <Card className="p-0">
        {channels === null ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : channels.length === 0 ? (
          <div className="flex flex-col items-center gap-3 p-12 text-center">
            <div className="grid h-14 w-14 place-items-center rounded-3xl bg-sakura-100 dark:bg-sakura-500/20">
              <PlugsConnectedIcon size={26} weight="duotone" className="text-sakura-500" />
            </div>
            <p className="font-bold text-ink">还没有配置上游渠道</p>
            <p className="text-sm text-ink-soft">添加一个 OpenAI 或 Anthropic 兼容的上游即可开始转发</p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs font-bold text-ink-soft">
                <th className="px-6 py-4">名称</th>
                <th className="px-4 py-4">类型</th>
                <th className="px-4 py-4">模型</th>
                <th className="px-4 py-4">优先级</th>
                <th className="px-4 py-4">状态</th>
                <th className="px-6 py-4 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {channels.map((ch) => (
                <tr key={ch.id} className="border-t border-sakura-50 dark:border-white/5">
                  <td className="px-6 py-3.5">
                    <div className="font-bold text-ink">{ch.name}</div>
                    <div className="text-xs text-ink-soft">{ch.base_url}</div>
                  </td>
                  <td className="px-4 py-3.5">
                    <span className="inline-flex items-center gap-1.5 rounded-full bg-sky/15 px-2.5 py-0.5 text-xs font-bold text-sky">
                      <ProviderIcon type={ch.type} size={13} />
                      {ch.type === 'anthropic' ? 'Anthropic' : 'OpenAI'}
                    </span>
                  </td>
                  <td className="max-w-[16rem] px-4 py-3.5">
                    <div className="flex flex-wrap gap-1">
                      {ch.models
                        .split(',')
                        .map((m) => m.trim())
                        .filter(Boolean)
                        .map((m) => (
                          <span
                            key={m}
                            className="inline-flex items-center gap-1 rounded-full bg-sakura-50 px-2 py-0.5 dark:bg-sakura-500/10 text-xs text-ink-soft"
                          >
                            <ModelIcon name={m} size={12} />
                            {m}
                          </span>
                        ))}
                    </div>
                  </td>
                  <td className="px-4 py-3.5 font-mono text-ink-soft">{ch.priority}</td>
                  <td className="px-4 py-3.5">
                    <StatusBadge enabled={ch.status === 1} />
                  </td>
                  <td className="px-6 py-3.5 text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="soft"
                        className="px-3 py-1.5 text-xs"
                        disabled={testingId === ch.id}
                        onClick={() => testChannel(ch)}
                        aria-label={`测试 ${ch.name}`}
                      >
                        <LightningIcon size={14} weight="bold" />
                        {testingId === ch.id ? '测试中' : '测试'}
                      </Button>
                      <Button
                        variant="soft"
                        className="px-3 py-1.5 text-xs"
                        onClick={() => openEdit(ch)}
                        aria-label={`编辑 ${ch.name}`}
                      >
                        <PencilSimpleIcon size={14} weight="bold" />
                      </Button>
                      <Button
                        variant="danger"
                        className="px-3 py-1.5 text-xs"
                        onClick={() => remove(ch)}
                        aria-label={`删除 ${ch.name}`}
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

      <Modal open={open} onClose={() => setOpen(false)} title={editing ? '编辑渠道' : '新建渠道'}>
        <form onSubmit={save} className="flex flex-col gap-4">
          <Field label="名称">
            <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="类型">
              <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
                <option value="openai">OpenAI 兼容</option>
                <option value="anthropic">Anthropic 兼容</option>
              </Select>
            </Field>
            <Field label="优先级" hint="数值越大越优先">
              <Input
                type="number"
                value={form.priority}
                onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })}
              />
            </Field>
          </div>
          <Field label="Base URL" hint="例如 https://api.openai.com，不含 /v1 路径">
            <Input
              value={form.base_url}
              onChange={(e) => setForm({ ...form, base_url: e.target.value })}
              placeholder="https://api.openai.com"
              required
            />
          </Field>
          <Field label="API Key" hint={editing ? '留空表示保持原有密钥不变' : undefined}>
            <Input
              type="password"
              value={form.api_key}
              onChange={(e) => setForm({ ...form, api_key: e.target.value })}
              placeholder={editing ? '••••••••' : 'sk-...'}
            />
          </Field>
          <Field label="支持的模型" hint="用英文逗号分隔；也可以点右侧按钮从上游自动获取">
            <div className="flex gap-2">
              <Input
                value={form.models}
                onChange={(e) => setForm({ ...form, models: e.target.value })}
                placeholder="gpt-4o, gpt-4o-mini"
                required
                className="flex-1"
              />
              <Button
                type="button"
                variant="soft"
                className="shrink-0 px-3 text-xs"
                disabled={fetchingModels}
                onClick={fetchModels}
              >
                <DownloadSimpleIcon size={14} weight="bold" />
                {fetchingModels ? '获取中…' : '获取模型'}
              </Button>
            </div>
          </Field>
          <Field label="状态">
            <Select
              value={form.status}
              onChange={(e) => setForm({ ...form, status: Number(e.target.value) })}
            >
              <option value={1}>启用</option>
              <option value={2}>停用</option>
            </Select>
          </Field>
          <Button type="submit" disabled={busy} className="mt-1">
            {busy ? '保存中…' : '保存'}
          </Button>
        </form>
      </Modal>
    </div>
  );
}
