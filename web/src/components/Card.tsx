import type { ReactNode } from 'react';

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={`rounded-3xl border border-white/70 bg-surface/80 p-6 shadow-[0_10px_40px_-20px_rgba(230,62,121,0.25)] backdrop-blur-sm dark:border-white/10 dark:shadow-[0_10px_40px_-20px_rgba(0,0,0,0.5)] ${className}`}
    >
      {children}
    </div>
  );
}
