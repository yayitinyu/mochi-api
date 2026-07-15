import { useEffect, useState } from 'react';
import {
  ArrowsLeftRightIcon,
  PencilSimpleIcon,
  PlusIcon,
  TrashIcon,
} from '@phosphor-icons/react';
import { api, ApiError } from '../lib/api';
import type { ModelMapping } from '../lib/types';
import { Button } from '../components/Button';
import { Card } from '../components/Card';
import { Field, Input } from '../components/Field';
import { Modal } from '../components/Modal';
import { useToast } from '../components/Toast';

interface Form {
  alias: string;
  upstream_name: string;
}

const empty: Form = { alias: '', upstream_name: '' };

export function ModelMappingsPage() {
  const toast = useToast();
  const [mappings, setMappings] = useState<ModelMapping[] | null>(null);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ModelMapping | null>(null);
  const [form, setForm] = useState<Form>(empty);
  const [busy, setBusy] = useState(false);

  async function load() {
    setMappings((await api.get<ModelMapping[]>('/api/model_mappings')) ?? []);
  }
  useEffect(() => {
    void load().catch(() => toast('error', '加载失败'));
  }, []);

  function openCreate() {
    setEditing(null);
    setForm(empty);
    setOpen(true);
  }
  function openEdit(m: ModelMapping) {
    setEditing(m);
    setForm({ alias: m.alias, upstream_name: m.upstream_name });
    setOpen(true);
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      if (editing) {
        await api.put(`/api/model_mappings/${editing.id}`, form);
        toast('success', '已更新');
      } else {
        await api.post('/api/model_mappings', form);
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

  async function remove(m: ModelMapping) {
    if (!window.confirm(`确定删除映射「${m.alias}」吗？`)) return;
    try {
      await api.del(`/api/model_mappings/${m.id}`);
      toast('success', '已删除');
      await load();
    } catch {
      toast('error', '删除失败');
    }
  }

  return (
    <div className="max-w-4xl">
      <div className="mb-4 flex items-center justify-between">
        <p className="px-1 text-sm text-ink-soft">将用户请求的模型别名映射到上游实际模型名。</p>
        <Button onClick={openCreate}>
          <PlusIcon size={16} weight="bold" /> 新建映射
        </Button>
      </div>

      <Card className="p-0">
        {mappings === null ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : mappings.length === 0 ? (
          <div className="flex flex-col items-center gap-3 p-12 text-center">
            <div className="grid h-14 w-14 place-items-center rounded-3xl bg-sakura-100 dark:bg-sakura-500/20">
              <ArrowsLeftRightIcon size={26} weight="duotone" className="text-sakura-500" />
            </div>
            <p className="font-bold text-ink">还没有设置模型映射</p>
            <p className="text-sm text-ink-soft">添加映射后，用户可使用别名请求 API</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs font-bold text-ink-soft">
                <th className="px-6 py-4 whitespace-nowrap">别名</th>
                <th className="px-4 py-4 whitespace-nowrap">上游模型名</th>
                <th className="px-6 py-4 text-right whitespace-nowrap">操作</th>
              </tr>
            </thead>
            <tbody>
              {mappings.map((m) => (
                <tr key={m.id} className="border-t border-sakura-50 dark:border-white/5">
                  <td className="px-6 py-3.5">
                    <span className="font-mono font-bold text-ink">{m.alias}</span>
                  </td>
                  <td className="px-4 py-3.5 font-mono text-ink">{m.upstream_name}</td>
                  <td className="px-6 py-3.5 text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="soft"
                        className="px-3 py-1.5 text-xs"
                        onClick={() => openEdit(m)}
                        aria-label={`编辑 ${m.alias}`}
                      >
                        <PencilSimpleIcon size={14} weight="bold" />
                      </Button>
                      <Button
                        variant="danger"
                        className="px-3 py-1.5 text-xs"
                        onClick={() => remove(m)}
                        aria-label={`删除 ${m.alias}`}
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

      <Modal open={open} onClose={() => setOpen(false)} title={editing ? '编辑映射' : '新建映射'}>
        <form onSubmit={save} className="flex flex-col gap-4">
          <Field label="别名" hint="用户使用此名称请求 API">
            <Input
              value={form.alias}
              onChange={(e) => setForm({ ...form, alias: e.target.value })}
              placeholder="my-smart-model"
              required
            />
          </Field>
          <Field label="上游模型名" hint="实际转发给上游渠道的模型名">
            <Input
              value={form.upstream_name}
              onChange={(e) => setForm({ ...form, upstream_name: e.target.value })}
              placeholder="gpt-4o"
              required
            />
          </Field>
          <Button type="submit" disabled={busy} className="mt-1">
            {busy ? '保存中…' : '保存'}
          </Button>
        </form>
      </Modal>
    </div>
  );
}
