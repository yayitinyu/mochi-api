import { useEffect, useState } from 'react';
import {
  CoinsIcon,
  CpuIcon,
  LightningIcon,
  type Icon,
} from '@phosphor-icons/react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { api } from '../lib/api';
import type { DailyStat, ModelStat, StatsSummary } from '../lib/types';
import { formatCost, formatNumber } from '../lib/format';
import { Card } from '../components/Card';
import { Heatmap } from '../components/Heatmap';
import { ModelIcon } from '../components/ModelIcon';
import { useToast } from '../components/Toast';

function StatCard({
  icon: IconCmp,
  tint,
  label,
  requests,
  tokens,
  cost,
}: {
  icon: Icon;
  tint: string;
  label: string;
  requests: number;
  tokens: number;
  cost: number;
}) {
  return (
    <Card className="flex flex-col gap-3">
      <div className="flex items-center gap-2.5">
        <div className={`grid h-10 w-10 place-items-center rounded-2xl ${tint}`}>
          <IconCmp size={20} weight="duotone" />
        </div>
        <span className="text-sm font-bold text-ink-soft">{label}</span>
      </div>
      <div className="text-3xl font-extrabold text-ink">{formatCost(cost)}</div>
      <div className="flex gap-4 text-xs font-bold text-ink-soft">
        <span>{formatNumber(requests)} 次请求</span>
        <span>{formatNumber(tokens)} tokens</span>
      </div>
    </Card>
  );
}

export function DashboardPage() {
  const toast = useToast();
  const [summary, setSummary] = useState<StatsSummary | null>(null);
  const [daily, setDaily] = useState<DailyStat[]>([]);
  const [models, setModels] = useState<ModelStat[]>([]);

  useEffect(() => {
    Promise.all([
      api.get<StatsSummary>('/api/stats/summary'),
      api.get<DailyStat[]>('/api/stats/daily?days=365'),
      api.get<ModelStat[]>('/api/stats/models?days=30'),
    ])
      .then(([s, d, m]) => {
        setSummary(s);
        setDaily(d);
        setModels(m);
      })
      .catch(() => toast('error', '加载统计失败'));
  }, []);

  // Last 30 days trend, filled to a continuous series.
  const trend = (() => {
    const byDay = new Map(daily.map((s) => [s.day, s]));
    const out: { day: string; label: string; cost: number; tokens: number }[] = [];
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    for (let i = 29; i >= 0; i--) {
      const d = new Date(today);
      d.setDate(d.getDate() - i);
      const pad = (x: number) => String(x).padStart(2, '0');
      const key = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
      const s = byDay.get(key);
      out.push({
        day: key,
        label: `${d.getMonth() + 1}/${d.getDate()}`,
        cost: s ? s.cost_micros / 1_000_000 : 0,
        tokens: s ? s.prompt_tokens + s.completion_tokens : 0,
      });
    }
    return out;
  })();

  const maxModelCost = Math.max(1, ...models.map((m) => m.cost_micros));

  return (
    <div className="flex flex-col gap-5">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <StatCard
          icon={LightningIcon}
          tint="bg-sakura-100 text-sakura-500"
          label="今日"
          requests={summary?.today.requests ?? 0}
          tokens={summary?.today.tokens ?? 0}
          cost={summary?.today.cost_micros ?? 0}
        />
        <StatCard
          icon={CpuIcon}
          tint="bg-sky/15 text-sky"
          label="本周"
          requests={summary?.week.requests ?? 0}
          tokens={summary?.week.tokens ?? 0}
          cost={summary?.week.cost_micros ?? 0}
        />
        <StatCard
          icon={CoinsIcon}
          tint="bg-honey/15 text-honey"
          label="本月"
          requests={summary?.month.requests ?? 0}
          tokens={summary?.month.tokens ?? 0}
          cost={summary?.month.cost_micros ?? 0}
        />
      </div>

      <Card>
        <h2 className="mb-4 text-base font-extrabold text-ink">过去一年的调用热力图</h2>
        <Heatmap stats={daily} />
      </Card>

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-5">
        <Card className="lg:col-span-3">
          <h2 className="mb-4 text-base font-extrabold text-ink">近 30 天费用趋势</h2>
          <ResponsiveContainer width="100%" height={240}>
            <AreaChart data={trend} margin={{ top: 4, right: 8, left: -8, bottom: 0 }}>
              <defs>
                <linearGradient id="costFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#f95d92" stopOpacity={0.35} />
                  <stop offset="100%" stopColor="#f95d92" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="rgba(249,93,146,0.16)" vertical={false} />
              <XAxis
                dataKey="label"
                tick={{ fontSize: 11, fill: '#9a8b93' }}
                tickLine={false}
                axisLine={false}
                interval={4}
              />
              <YAxis
                tick={{ fontSize: 11, fill: '#9a8b93' }}
                tickLine={false}
                axisLine={false}
                width={48}
                tickFormatter={(v) => '$' + v}
              />
              <Tooltip
                contentStyle={{
                  borderRadius: 16,
                  border: '1px solid rgba(249,93,146,0.25)',
                  backgroundColor: 'var(--color-surface)',
                  color: 'var(--color-ink)',
                  fontSize: 12,
                  fontWeight: 700,
                }}
                formatter={(v: number) => ['$' + v.toFixed(4), '费用']}
                labelFormatter={(l) => `${l}`}
              />
              <Area
                type="monotone"
                dataKey="cost"
                stroke="#f95d92"
                strokeWidth={2.5}
                fill="url(#costFill)"
              />
            </AreaChart>
          </ResponsiveContainer>
        </Card>

        <Card className="lg:col-span-2">
          <h2 className="mb-4 text-base font-extrabold text-ink">模型用量排行（近 30 天）</h2>
          {models.length === 0 ? (
            <p className="py-8 text-center text-sm font-bold text-ink-soft">暂无数据</p>
          ) : (
            <div className="flex flex-col gap-3">
              {models.slice(0, 6).map((m) => (
                <div key={m.model_name}>
                  <div className="mb-1 flex items-center justify-between text-sm">
                    <span className="inline-flex items-center gap-1.5 font-bold text-ink">
                      <ModelIcon name={m.model_name} size={15} />
                      {m.model_name}
                    </span>
                    <span className="font-mono text-xs text-ink-soft">{formatCost(m.cost_micros)}</span>
                  </div>
                  <div className="h-2 overflow-hidden rounded-full bg-sakura-50 dark:bg-sakura-500/10">
                    <div
                      className="h-full rounded-full bg-sakura-400"
                      style={{ width: `${Math.max(4, (m.cost_micros / maxModelCost) * 100)}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
