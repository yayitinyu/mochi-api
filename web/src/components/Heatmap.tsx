import { useEffect, useMemo, useRef, useState } from 'react';
import type { DailyStat } from '../lib/types';
import { formatCost, formatNumber } from '../lib/format';

// GitHub-style contribution heatmap: 7 rows (Mon-Sun) x ~53 week columns,
// colored by daily token usage on a 5-step sakura scale.

const LEVEL_CLASSES = [
  'bg-sakura-500/10', // 0 requests
  'bg-sakura-200',
  'bg-sakura-300',
  'bg-sakura-400',
  'bg-sakura-600',
];

const MONTH_NAMES = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月'];

interface Cell {
  day: string; // "2026-07-12"
  date: Date;
  stat?: DailyStat;
}

function formatDay(d: Date): string {
  const pad = (x: number) => String(x).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function levelFor(tokens: number, max: number): number {
  if (tokens <= 0) return 0;
  const ratio = tokens / max;
  if (ratio <= 0.25) return 1;
  if (ratio <= 0.5) return 2;
  if (ratio <= 0.75) return 3;
  return 4;
}

export function Heatmap({ stats }: { stats: DailyStat[] }) {
  const [hover, setHover] = useState<Cell | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  // On narrow screens the grid overflows; keep the newest days (rightmost
  // columns) in view instead of showing a year-old left edge.
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollLeft = el.scrollWidth;
  }, [stats]);

  const { weeks, maxTokens } = useMemo(() => {
    const byDay = new Map(stats.map((s) => [s.day, s]));
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    // Start 52 weeks back, aligned to Monday.
    const start = new Date(today);
    start.setDate(start.getDate() - 364);
    const weekday = (start.getDay() + 6) % 7; // 0 = Monday
    start.setDate(start.getDate() - weekday);

    const weeks: Cell[][] = [];
    let cursor = new Date(start);
    while (cursor <= today) {
      const week: Cell[] = [];
      for (let i = 0; i < 7; i++) {
        if (cursor <= today) {
          const day = formatDay(cursor);
          week.push({ day, date: new Date(cursor), stat: byDay.get(day) });
        }
        cursor = new Date(cursor);
        cursor.setDate(cursor.getDate() + 1);
      }
      weeks.push(week);
    }
    const maxTokens = Math.max(0, ...stats.map((s) => s.prompt_tokens + s.completion_tokens));
    return { weeks, maxTokens };
  }, [stats]);

  // Month label above the first week column that starts a new month.
  const monthLabels = useMemo(() => {
    let prev = -1;
    return weeks.map((week) => {
      const month = week[0]!.date.getMonth();
      if (month !== prev) {
        prev = month;
        return MONTH_NAMES[month];
      }
      return '';
    });
  }, [weeks]);

  return (
    <div>
      <div ref={scrollRef} className="overflow-x-auto pb-1">
        <div className="min-w-max">
          <div className="mb-1.5 flex gap-[3px]">
            {monthLabels.map((label, i) => (
              <div key={i} className="w-3 text-[10px] font-bold text-ink-soft">
                {label && <span className="whitespace-nowrap">{label}</span>}
              </div>
            ))}
          </div>
          <div className="flex gap-[3px]">
            {weeks.map((week, wi) => (
              <div key={wi} className="flex flex-col gap-[3px]">
                {week.map((cell) => (
                  <div
                    key={cell.day}
                    onMouseEnter={() => setHover(cell)}
                    onMouseLeave={() => setHover(null)}
                    className={`h-3 w-3 rounded-[4px] transition hover:ring-2 hover:ring-sakura-400 ${
                      LEVEL_CLASSES[
                        levelFor(
                          (cell.stat?.prompt_tokens ?? 0) + (cell.stat?.completion_tokens ?? 0),
                          maxTokens,
                        )
                      ]
                    }`}
                  />
                ))}
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="mt-3 flex items-center justify-between text-xs text-ink-soft">
        <div className="h-4 font-bold">
          {hover
            ? hover.stat
              ? `${hover.day} · ${formatNumber(
                  hover.stat.prompt_tokens + hover.stat.completion_tokens,
                )} tokens · ${formatNumber(hover.stat.requests)} 次请求 · ${formatCost(hover.stat.cost_micros)}`
              : `${hover.day} · 无调用`
            : '将鼠标悬停在方块上查看当日用量'}
        </div>
        <div className="flex items-center gap-1">
          <span>少</span>
          {LEVEL_CLASSES.map((cls) => (
            <div key={cls} className={`h-3 w-3 rounded-[4px] ${cls}`} />
          ))}
          <span>多</span>
        </div>
      </div>
    </div>
  );
}
