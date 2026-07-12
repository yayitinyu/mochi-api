import type { ButtonHTMLAttributes, ReactNode } from 'react';

type Variant = 'primary' | 'ghost' | 'soft' | 'danger';

const variants: Record<Variant, string> = {
  primary:
    'bg-sakura-500 text-white hover:bg-sakura-600 shadow-sm shadow-sakura-200 active:scale-[0.98] dark:shadow-none',
  ghost:
    'text-ink-soft hover:text-ink hover:bg-sakura-50 active:scale-[0.98] dark:hover:bg-sakura-500/10',
  soft: 'bg-sakura-100 text-sakura-700 hover:bg-sakura-200 active:scale-[0.98] dark:bg-sakura-500/20 dark:text-sakura-200 dark:hover:bg-sakura-500/30',
  danger:
    'bg-surface text-sakura-600 border border-sakura-200 hover:bg-sakura-50 active:scale-[0.98] dark:border-sakura-500/30 dark:text-sakura-300 dark:hover:bg-sakura-500/10',
};

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  children: ReactNode;
}

export function Button({ variant = 'primary', className = '', children, ...rest }: Props) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-2 rounded-full px-4 py-2 text-sm font-bold transition disabled:cursor-not-allowed disabled:opacity-50 ${variants[variant]} ${className}`}
      {...rest}
    >
      {children}
    </button>
  );
}
