import type { InputHTMLAttributes, ReactNode } from 'react';

interface FieldProps {
  label: string;
  children: ReactNode;
  hint?: string;
}

export function Field({ label, children, hint }: FieldProps) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-sm font-bold text-ink">{label}</span>
      {children}
      {hint && <span className="text-xs text-ink-soft">{hint}</span>}
    </label>
  );
}

export function Input({ className = '', ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={`rounded-2xl border border-sakura-100 bg-surface px-4 py-2.5 text-sm text-ink outline-none transition placeholder:text-ink-soft/60 focus:border-sakura-300 focus:ring-2 focus:ring-sakura-100 dark:border-white/10 dark:focus:border-sakura-500/50 dark:focus:ring-sakura-500/20 ${className}`}
      {...rest}
    />
  );
}

export function Select({
  className = '',
  children,
  ...rest
}: React.SelectHTMLAttributes<HTMLSelectElement> & { children: ReactNode }) {
  return (
    <select
      className={`rounded-2xl border border-sakura-100 bg-surface px-4 py-2.5 text-sm text-ink outline-none transition focus:border-sakura-300 focus:ring-2 focus:ring-sakura-100 dark:border-white/10 dark:focus:border-sakura-500/50 dark:focus:ring-sakura-500/20 ${className}`}
      {...rest}
    >
      {children}
    </select>
  );
}
