import { useEffect, useState } from 'react';
import { PencilSimpleIcon, PlusIcon, TagIcon, TrashIcon } from '@phosphor-icons/react';
import { api, ApiError } from '../lib/api';
import type { ModelPrice } from '../lib/types';
import { Button } from '../components/Button';
import { Card } from '../components/Card';
import { Field, Input } from '../components/Field';
import { Modal } from '../components/Modal';
import { ModelIcon } from '../components/ModelIcon';
import { useToast } from '../components/Toast';

interface Form {
  model: string;
  input_price: number;
  output_price: number;
}

const empty: Form = { model: '', input_price: 0, output_price: 0 };

export function PricesPage() {
  const toast = useToast();
  const [prices, setPrices] = useState<ModelPrice[] | null>(null);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ModelPrice | null>(null);
  const [form, setForm] = useState<Form>(empty);
  const [busy, setBusy] = useState(false);

  async function load() {
    setPrices((await api.get<ModelPrice[]>('/api/prices')) ?? []);
  }
  useEffect(() => {
    void load().catch(() => toast('error', '加载失败'));
  }, []);

  function openCreate() {
    setEditing(null);
    setForm(empty);
    setOpen(true);
  }
  function openEdit(p: ModelPrice) {
    setEditing(p);
    setForm({ model: p.model, input_price: p.input_price, output_price: p.output_price });
    setOpen(true);
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      if (editing) {
        await api.put(`/api/prices/${editing.id}`, form);
        toast('success', '已更新');
      } else {
        await api.post('/api/prices', form);
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

  async function remove(p: ModelPrice) {
    if (!window.confirm(`确定删除「${p.model}」的价格吗？`)) return;
    try {
      await api.del(`/api/prices/${p.id}`);
      toast('success', '已删除');
      await load();
    } catch {
      toast('error', '删除失败');
    }
  }

  return (
    <div className="max-w-4xl">
      <div className="mb-4 flex items-center justify-between">
        <p className="px-1 text-sm text-ink-soft">价格单位：美元 / 每百万 tokens。模型名支持 * 结尾通配。</p>
        <Button onClick={openCreate}>
          <PlusIcon size={16} weight="bold" /> 新建价格
        </Button>
      </div>

      <Card className="p-0">
        {prices === null ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : prices.length === 0 ? (
          <div className="flex flex-col items-center gap-3 p-12 text-center">
            <div className="grid h-14 w-14 place-items-center rounded-3xl bg-sakura-100 dark:bg-sakura-500/20">
              <TagIcon size={26} weight="duotone" className="text-sakura-500" />
            </div>
            <p className="font-bold text-ink">还没有设置模型价格</p>
            <p className="text-sm text-ink-soft">未设置价格的模型，其调用费用会记为 $0</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs font-bold text-ink-soft">
                <th className="px-6 py-4">模型</th>
                <th className="px-4 py-4">输入价 / 1M</th>
                <th className="px-4 py-4">输出价 / 1M</th>
                <th className="px-6 py-4 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {prices.map((p) => (
                <tr key={p.id} className="border-t border-sakura-50 dark:border-white/5">
                  <td className="px-6 py-3.5">
                    <span className="inline-flex items-center gap-2 font-mono font-bold text-ink">
                      <ModelIcon name={p.model} size={15} />
                      {p.model}
                    </span>
                  </td>
                  <td className="px-4 py-3.5 text-ink">${p.input_price}</td>
                  <td className="px-4 py-3.5 text-ink">${p.output_price}</td>
                  <td className="px-6 py-3.5 text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="soft"
                        className="px-3 py-1.5 text-xs"
                        onClick={() => openEdit(p)}
                        aria-label={`编辑 ${p.model}`}
                      >
                        <PencilSimpleIcon size={14} weight="bold" />
                      </Button>
                      <Button
                        variant="danger"
                        className="px-3 py-1.5 text-xs"
                        onClick={() => remove(p)}
                        aria-label={`删除 ${p.model}`}
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

      <Modal open={open} onClose={() => setOpen(false)} title={editing ? '编辑价格' : '新建价格'}>
        <form onSubmit={save} className="flex flex-col gap-4">
          <Field label="模型名" hint="精确匹配优先；claude-3-5* 可匹配该前缀所有模型">
            <Input
              value={form.model}
              onChange={(e) => setForm({ ...form, model: e.target.value })}
              placeholder="gpt-4o"
              required
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="输入价（$/1M）">
              <Input
                type="number"
                step="any"
                min="0"
                value={form.input_price}
                onChange={(e) => setForm({ ...form, input_price: Number(e.target.value) })}
                required
              />
            </Field>
            <Field label="输出价（$/1M）">
              <Input
                type="number"
                step="any"
                min="0"
                value={form.output_price}
                onChange={(e) => setForm({ ...form, output_price: Number(e.target.value) })}
                required
              />
            </Field>
          </div>
          <Button type="submit" disabled={busy} className="mt-1">
            {busy ? '保存中…' : '保存'}
          </Button>
        </form>
      </Modal>
    </div>
  );
}
