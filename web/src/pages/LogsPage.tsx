import { useEffect, useState } from 'react';
import { ScrollIcon } from '@phosphor-icons/react';
import { api } from '../lib/api';
import type { LogEntry } from '../lib/types';
import { formatCost, formatNumber, formatTime } from '../lib/format';
import { Card } from '../components/Card';
import { Input } from '../components/Field';
import { Button } from '../components/Button';
import { CodeBadge } from '../components/Badge';
import { ModelIcon } from '../components/ModelIcon';
import { useToast } from '../components/Toast';

const PAGE_SIZE = 20;

export function LogsPage() {
  const toast = useToast();
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [modelFilter, setModelFilter] = useState('');
  const [loading, setLoading] = useState(true);

  async function load(p: number, model: string) {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: String(p), page_size: String(PAGE_SIZE) });
      if (model) params.set('model', model);
      const res = await api.get<{ items: LogEntry[]; total: number }>(`/api/logs?${params}`);
      setLogs(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch {
      toast('error', '加载失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load(page, modelFilter);
  }, [page]);

  function applyFilter(e: React.FormEvent) {
    e.preventDefault();
    setPage(1);
    void load(1, modelFilter);
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="max-w-6xl">
      <form onSubmit={applyFilter} className="mb-4 flex gap-2">
        <Input
          value={modelFilter}
          onChange={(e) => setModelFilter(e.target.value)}
          placeholder="按模型名筛选，例如 gpt-4o"
          className="w-64"
        />
        <Button type="submit" variant="soft">
          筛选
        </Button>
        {modelFilter && (
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              setModelFilter('');
              setPage(1);
              void load(1, '');
            }}
          >
            清除
          </Button>
        )}
      </form>

      <Card className="p-0">
        {loading ? (
          <div className="p-10 text-center text-sm font-bold text-ink-soft">加载中…</div>
        ) : logs.length === 0 ? (
          <div className="flex flex-col items-center gap-3 p-12 text-center">
            <div className="grid h-14 w-14 place-items-center rounded-3xl bg-sakura-100 dark:bg-sakura-500/20">
              <ScrollIcon size={26} weight="duotone" className="text-sakura-500" />
            </div>
            <p className="font-bold text-ink">还没有调用记录</p>
            <p className="text-sm text-ink-soft">通过 API 密钥发起请求后，日志会显示在这里</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs font-bold text-ink-soft">
                  <th className="px-6 py-4">时间</th>
                  <th className="px-4 py-4">模型</th>
                  <th className="px-4 py-4">密钥</th>
                  <th className="px-4 py-4 text-right">输入</th>
                  <th className="px-4 py-4 text-right">输出</th>
                  <th className="px-4 py-4 text-right">费用</th>
                  <th className="px-4 py-4 text-right">耗时</th>
                  <th className="px-6 py-4 text-center">状态</th>
                </tr>
              </thead>
              <tbody>
                {logs.map((l) => (
                  <tr key={l.id} className="border-t border-sakura-50 dark:border-white/5">
                    <td className="whitespace-nowrap px-6 py-3 text-ink-soft">{formatTime(l.created_at)}</td>
                    <td className="px-4 py-3">
                      <span className="inline-flex items-center gap-1.5 font-bold text-ink">
                        <ModelIcon name={l.model_name} size={15} />
                        {l.model_name}
                      </span>
                      {l.is_stream && (
                        <span className="ml-1.5 rounded-full bg-honey/15 px-1.5 py-0.5 text-[10px] font-bold text-honey">
                          流式
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-ink-soft">{l.token_name}</td>
                    <td className="px-4 py-3 text-right font-mono text-ink-soft">
                      {formatNumber(l.prompt_tokens)}
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-ink-soft">
                      {formatNumber(l.completion_tokens)}
                    </td>
                    <td className="px-4 py-3 text-right font-mono font-bold text-ink">
                      {l.cost_micros > 0 ? (
                        formatCost(l.cost_micros)
                      ) : (
                        <span className="font-sans text-xs font-bold text-ink-soft/60">未定价</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-ink-soft">{l.use_time_ms}ms</td>
                    <td className="px-6 py-3 text-center">
                      <CodeBadge code={l.code} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {total > PAGE_SIZE && (
        <div className="mt-4 flex items-center justify-center gap-3">
          <Button variant="soft" disabled={page <= 1} onClick={() => setPage(page - 1)}>
            上一页
          </Button>
          <span className="text-sm font-bold text-ink-soft">
            {page} / {totalPages}
          </span>
          <Button variant="soft" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
            下一页
          </Button>
        </div>
      )}
    </div>
  );
}
