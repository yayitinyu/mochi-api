import type { ReactNode } from 'react';
import { XIcon } from '@phosphor-icons/react';

interface Props {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
}

export function Modal({ open, onClose, title, children }: Props) {
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-ink/20 p-0 backdrop-blur-sm sm:items-center sm:p-4"
      onClick={onClose}
    >
      <div
        className="flex max-h-[92dvh] w-full max-w-lg flex-col overflow-hidden rounded-t-3xl border border-white bg-cream shadow-2xl sm:max-h-[88dvh] sm:rounded-3xl dark:border-white/10"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 pb-3 pt-6">
          <h2 className="text-lg font-extrabold text-ink">{title}</h2>
          <button
            onClick={onClose}
            aria-label="关闭"
            className="rounded-full p-1.5 text-ink-soft transition hover:bg-sakura-100 hover:text-ink dark:hover:bg-sakura-500/20"
          >
            <XIcon size={20} weight="bold" />
          </button>
        </div>
        <div className="overflow-y-auto px-6 pb-6">{children}</div>
      </div>
    </div>
  );
}
