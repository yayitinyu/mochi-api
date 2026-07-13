export function StatusBadge({ enabled }: { enabled: boolean }) {
  return enabled ? (
    <span className="inline-flex min-w-14 shrink-0 items-center justify-center whitespace-nowrap rounded-full bg-matcha/15 px-2.5 py-1 text-xs font-bold text-matcha">
      启用
    </span>
  ) : (
    <span className="inline-flex min-w-14 shrink-0 items-center justify-center whitespace-nowrap rounded-full bg-ink-soft/15 px-2.5 py-1 text-xs font-bold text-ink-soft">
      停用
    </span>
  );
}

export function CodeBadge({ code }: { code: number }) {
  const ok = code >= 200 && code < 300;
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold ${
        ok ? 'bg-matcha/15 text-matcha' : 'bg-sakura-100 text-sakura-600'
      }`}
    >
      {code}
    </span>
  );
}
