import { useEffect, useState } from 'react';
import {
  DownloadSimpleIcon,
  LightningIcon,
  PencilSimpleIcon,
  PlusIcon,
  PlugsConnectedIcon,
  TrashIcon,
  XIcon,
} from '@phosphor-icons/react';
import { api, ApiError } from '../lib/api';
import type { Channel, ResponsesMode } from '../lib/types';
import { Button } from '../components/Button';
import { Card } from '../components/Card';
import { Field, Input, Select } from '../components/Field';
import { Modal } from '../components/Modal';
import { StatusBadge } from '../components/Badge';
import { ChannelIcon, ModelIcon, ProviderIcon } from '../components/ModelIcon';
import { ChannelPresetPicker } from '../components/ChannelPresetPicker';
import { CHANNEL_PRESETS, type ChannelPreset } from '../lib/channelPresets';
import { useToast } from '../components/Toast';

interface Form {
  name: string;
  type: string;
  base_url: string;
  api_key: string;
  models: string;
  responses_mode: ResponsesMode;
  icon: string;
  priority: number | '';
  status: number;
}

type ModelPickerMode = 'add' | 'remove' | null;

const empty: Form = {
  name: '',
  type: 'openai',
  base_url: '',
  api_key: '',
  models: '',
  responses_mode: 'chat',
  icon: '',
  priority: '',
  status: 1,
};

function parseModels(value: string): string[] {
  return [...new Set(value.split(',').map((model) => model.trim()).filter(Boolean))];
}

export function ChannelsPage() {
  const toast = useToast();
  const [channels, setChannels] = useState<Channel[] | null>(null);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Channel | null>(null);
  const [form, setForm] = useState<Form>(empty);
  const [busy, setBusy] = useState(false);
  const [testingId, setTestingId] = useState(0);
  const [fetchingModels, setFetchingModels] = useState(false);
  const [availableModels, setAvailableModels] = useState<string[]>([]);
  const [selectedModels, setSelectedModels] = useState<Set<string>>(new Set());
  const [modelQuery, setModelQuery] = useState('');
  const [modelPickerMode, setModelPickerMode] = useState<ModelPickerMode>(null);
  const [preset, setPreset] = useState('custom');

  async function load() {
    setChannels((await api.get<Channel[]>('/api/channels')) ?? []);
  }
  useEffect(() => {
    void load().catch(() => toast('error', '加载失败'));
  }, []);

  function openCreate() {
    setEditing(null);
    setForm(empty);
    setPreset('custom');
    setAvailableModels([]);
    setSelectedModels(new Set());
    setModelQuery('');
    setModelPickerMode(null);
    setOpen(true);
  }
  function openEdit(ch: Channel) {
    setEditing(ch);
    setForm({ ...ch, api_key: '', responses_mode: ch.responses_mode || 'chat' });
    setPreset('custom');
    setAvailableModels([]);
    setSelectedModels(new Set());
    setModelQuery('');
    setModelPickerMode(null);
    setOpen(true);
  }

  function applyPreset(p: ChannelPreset | null) {
    if (!p) {
      setPreset('custom');
      return;
    }
    setPreset(p.id);
    setForm((f) => {
      // Keep a hand-typed name, but replace an untouched preset label when
      // the user switches presets.
      const nameIsAuto = f.name === '' || CHANNEL_PRESETS.some((x) => x.label === f.name);
      return {
        ...f,
        name: nameIsAuto ? p.label : f.name,
        type: p.type,
        base_url: p.baseUrl,
        responses_mode: p.type === 'openai' ? f.responses_mode : 'chat',
        icon: p.icon,
      };
    });
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    const payload = { ...form, priority: form.priority === '' ? 0 : form.priority };
    try {
      if (editing) {
        await api.put(`/api/channels/${editing.id}`, payload);
        toast('success', '已更新');
      } else {
        await api.post('/api/channels', payload);
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
      const fetchedModels = [...new Set((res.models ?? []).map((model) => model.trim()).filter(Boolean))];
      if (fetchedModels.length === 0) {
        toast('error', '上游没有返回任何模型');
      } else {
        const existing = new Set(parseModels(form.models));
        const newModels = fetchedModels.filter((model) => !existing.has(model));
        if (newModels.length === 0) {
          setAvailableModels([]);
          setSelectedModels(new Set());
          setModelQuery('');
          setModelPickerMode(null);
          toast('success', `上游返回的 ${fetchedModels.length} 个模型均已添加`);
          return;
        }
        setAvailableModels(newModels);
        setSelectedModels(new Set(newModels));
        setModelQuery('');
        setModelPickerMode('add');
        const excluded = fetchedModels.length - newModels.length;
        toast(
          'success',
          excluded > 0
            ? `发现 ${newModels.length} 个新模型，已排除 ${excluded} 个已添加模型`
            : `已获取 ${newModels.length} 个模型，请勾选要加入的模型`,
        );
      }
    } catch (err) {
      toast('error', err instanceof ApiError ? err.message : '获取失败');
    } finally {
      setFetchingModels(false);
    }
  }

  function toggleModel(model: string) {
    setSelectedModels((current) => {
      const next = new Set(current);
      if (next.has(model)) next.delete(model);
      else next.add(model);
      return next;
    });
  }

  function addSelectedModels() {
    const existing = parseModels(form.models);
    const merged = [...new Set([...existing, ...availableModels.filter((model) => selectedModels.has(model))])];
    setForm((current) => ({ ...current, models: merged.join(', ') }));
    toast('success', `已加入 ${merged.length - existing.length} 个新模型，保存后生效`);
    closeModelPicker();
  }

  function openModelCleanup() {
    if (configuredModels.length === 0) {
      toast('error', '当前没有可清理的模型');
      return;
    }
    setAvailableModels([]);
    setSelectedModels(new Set());
    setModelQuery('');
    setModelPickerMode('remove');
  }

  function removeSelectedModels() {
    const remaining = configuredModels.filter((model) => !selectedModels.has(model));
    const removedCount = configuredModels.length - remaining.length;
    setForm((current) => ({ ...current, models: remaining.join(', ') }));
    toast('success', `已移除 ${removedCount} 个模型，保存后生效`);
    closeModelPicker();
  }

  function closeModelPicker() {
    setAvailableModels([]);
    setSelectedModels(new Set());
    setModelQuery('');
    setModelPickerMode(null);
  }

  const configuredModels = parseModels(form.models);
  const pickerModels = modelPickerMode === 'remove' ? configuredModels : availableModels;
  const visibleModels = pickerModels.filter((model) =>
    model.toLocaleLowerCase().includes(modelQuery.trim().toLocaleLowerCase()),
  );
  const selectedModelCount = pickerModels.filter((model) => selectedModels.has(model)).length;

  return (
    <div className="max-w-6xl">
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
          <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs font-bold text-ink-soft">
                <th className="px-6 py-4 whitespace-nowrap">名称</th>
                <th className="px-4 py-4 whitespace-nowrap">类型</th>
                <th className="px-4 py-4 whitespace-nowrap">模型</th>
                <th className="px-4 py-4 whitespace-nowrap">优先级</th>
                <th className="px-4 py-4 whitespace-nowrap">状态</th>
                <th className="px-6 py-4 text-right whitespace-nowrap">操作</th>
              </tr>
            </thead>
            <tbody>
              {channels.map((ch) => (
                <tr key={ch.id} className="border-t border-sakura-50 dark:border-white/5">
                  <td className="px-6 py-3.5">
                    <div className="flex items-center gap-2">
                      <ChannelIcon icon={ch.icon} type={ch.type} size={18} />
                      <div className="min-w-0">
                        <div className="font-bold text-ink">{ch.name}</div>
                        <div className="text-xs text-ink-soft">{ch.base_url}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3.5">
                    <span className="inline-flex items-center gap-1.5 rounded-full bg-sky/15 px-2.5 py-0.5 text-xs font-bold text-sky">
                      <ProviderIcon type={ch.type} size={13} />
                      {ch.type === 'anthropic' ? 'Anthropic' : ch.type === 'gemini' ? 'Gemini' : 'OpenAI'}
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
                  <td className="px-4 py-3.5 whitespace-nowrap">
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
          </div>
        )}
      </Card>

      <Modal open={open} onClose={() => setOpen(false)} title={editing ? '编辑渠道' : '新建渠道'}>
        <form onSubmit={save} className="flex flex-col gap-4" autoComplete="off">
          {/* Decoy fields absorb the browser's username/password autofill so it
              doesn't land in Base URL / API Key. */}
          <input type="text" name="username" autoComplete="username" className="hidden" tabIndex={-1} aria-hidden />
          <input type="password" name="password" autoComplete="current-password" className="hidden" tabIndex={-1} aria-hidden />
          {!editing && (
            <Field label="官方渠道" hint="选择后自动填入类型、Base URL 与图标，只需再粘贴 API Key">
              <ChannelPresetPicker value={preset} onSelect={applyPreset} />
            </Field>
          )}
          <Field label="名称">
            <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          </Field>
          <Field
            label="图标"
            hint="可选；填写图片 URL 自定义图标，选择官方渠道时自动填入"
          >
            <div className="flex items-center gap-2">
              <span className="grid h-10 w-10 shrink-0 place-items-center rounded-2xl border border-sakura-100 bg-surface dark:border-white/10">
                <ChannelIcon icon={form.icon} type={form.type} size={18} />
              </span>
              <Input
                value={form.icon}
                onChange={(e) => setForm({ ...form, icon: e.target.value })}
                placeholder="https://example.com/logo.png"
                name="channel_icon"
                autoComplete="off"
                className="flex-1 min-w-0"
              />
            </div>
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="类型">
              <Select
                value={form.type}
                onChange={(e) => {
                  const type = e.target.value;
                  setForm((current) => ({
                    ...current,
                    type,
                    responses_mode: type === 'openai' ? current.responses_mode : 'chat',
                  }));
                }}
              >
                <option value="openai">OpenAI 兼容</option>
                <option value="anthropic">Anthropic 兼容</option>
                <option value="gemini">Google Gemini</option>
              </Select>
            </Field>
            <Field label="优先级" hint="数值越大越优先">
              <Input
                type="number"
                value={form.priority}
                placeholder="0"
                onChange={(e) => {
                  const priority = e.target.value === '' ? '' : Number(e.target.value);
                  setForm((current) => ({ ...current, priority }));
                }}
              />
            </Field>
          </div>
          {form.type === 'openai' && (
            <Field
              label="Responses 兼容模式"
              hint="默认通过 Chat Completions 转换，兼容多数渠道；仅在上游完整支持 /v1/responses（包括流式事件）时选择原生模式"
            >
              <Select
                value={form.responses_mode}
                onChange={(e) =>
                  setForm((current) => ({
                    ...current,
                    responses_mode: e.target.value as ResponsesMode,
                  }))
                }
              >
                <option value="chat">Chat 转换（推荐）</option>
                <option value="native">原生 Responses</option>
              </Select>
            </Field>
          )}
          <Field
            label="Base URL"
            hint={
              form.type === 'gemini'
                ? '例如 https://generativelanguage.googleapis.com，不含 /v1beta 路径'
                : '例如 https://api.openai.com（自动追加 /v1/…）；以 / 结尾表示完整 API 前缀，以 # 结尾表示完整端点 URL'
            }
          >
            <Input
              value={form.base_url}
              onChange={(e) => setForm({ ...form, base_url: e.target.value })}
              placeholder="https://api.openai.com"
              name="channel_base_url"
              autoComplete="off"
              required
            />
          </Field>
          <Field label="API Key" hint={editing ? '留空表示保持原有密钥不变' : undefined}>
            <Input
              type="password"
              value={form.api_key}
              onChange={(e) => setForm({ ...form, api_key: e.target.value })}
              placeholder={editing ? '••••••••' : form.type === 'gemini' ? 'AIza...' : 'sk-...'}
              name="channel_api_key"
              autoComplete="new-password"
            />
          </Field>
          <Field label="支持的模型" hint="用英文逗号分隔；聊天与图片模型可放在同一渠道，也可以从上游获取或清理">
            <div className="flex flex-col gap-2 sm:flex-row">
              <Input
                value={form.models}
                onChange={(e) => setForm({ ...form, models: e.target.value })}
                placeholder="gpt-4o, gpt-image-2"
                required
                className="flex-1 min-w-0"
              />
              <div className="flex shrink-0 gap-2">
                <Button
                  type="button"
                  variant="soft"
                  className="flex-1 px-3 text-xs sm:flex-none"
                  disabled={fetchingModels}
                  onClick={fetchModels}
                >
                  <DownloadSimpleIcon size={14} weight="bold" />
                  {fetchingModels ? '获取中…' : '一键获取模型'}
                </Button>
                <Button
                  type="button"
                  variant="danger"
                  className="flex-1 px-3 text-xs sm:flex-none"
                  disabled={configuredModels.length === 0}
                  onClick={openModelCleanup}
                >
                  <TrashIcon size={14} weight="bold" />
                  清理模型
                </Button>
              </div>
            </div>
          </Field>
          {modelPickerMode !== null && pickerModels.length > 0 && (
            <section className="rounded-2xl border border-sakura-100 bg-sakura-50/60 p-3 dark:border-white/10 dark:bg-sakura-500/5">
              <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                <span className="text-sm font-bold text-ink">
                  {modelPickerMode === 'remove' ? '选择要移除的模型' : '选择要加入的模型'}
                </span>
                <div className="flex items-center gap-2 text-xs font-bold">
                  <button
                    type="button"
                    className="text-sakura-600 hover:text-sakura-700 dark:text-sakura-300"
                    onClick={() => setSelectedModels(new Set(pickerModels))}
                  >
                    全选
                  </button>
                  <button
                    type="button"
                    className="text-ink-soft hover:text-ink"
                    onClick={() => setSelectedModels(new Set())}
                  >
                    清空
                  </button>
                  <button
                    type="button"
                    aria-label="关闭模型选择"
                    className="rounded-full p-1 text-ink-soft transition hover:bg-surface hover:text-ink"
                    onClick={closeModelPicker}
                  >
                    <XIcon size={14} weight="bold" />
                  </button>
                </div>
              </div>
              <Input
                value={modelQuery}
                onChange={(event) => setModelQuery(event.target.value)}
                placeholder="搜索模型"
                className="mb-2 w-full bg-surface"
              />
              <div className="max-h-44 overflow-y-auto rounded-xl bg-surface/70 p-1">
                {visibleModels.length === 0 ? (
                  <p className="p-3 text-center text-xs font-bold text-ink-soft">没有匹配的模型</p>
                ) : (
                  visibleModels.map((model) => (
                    <label
                      key={model}
                      className="flex cursor-pointer items-center gap-2 rounded-xl px-2.5 py-2 text-sm transition hover:bg-sakura-50 dark:hover:bg-sakura-500/10"
                    >
                      <input
                        type="checkbox"
                        checked={selectedModels.has(model)}
                        onChange={() => toggleModel(model)}
                        className="h-4 w-4 accent-sakura-500"
                      />
                      <ModelIcon name={model} size={14} />
                      <span className="min-w-0 break-all font-mono text-xs text-ink">{model}</span>
                    </label>
                  ))
                )}
              </div>
              <Button
                type="button"
                variant={modelPickerMode === 'remove' ? 'danger' : 'soft'}
                className="mt-2 w-full text-xs"
                disabled={selectedModelCount === 0}
                onClick={modelPickerMode === 'remove' ? removeSelectedModels : addSelectedModels}
              >
                {modelPickerMode === 'remove' ? '移除' : '加入'}已选模型（{selectedModelCount}）
              </Button>
            </section>
          )}
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
