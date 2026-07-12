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
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink/20 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg rounded-3xl border border-white bg-cream p-6 shadow-2xl dark:border-white/10"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-extrabold text-ink">{title}</h2>
          <button
            onClick={onClose}
            aria-label="关闭"
            className="rounded-full p-1.5 text-ink-soft transition hover:bg-sakura-100 hover:text-ink dark:hover:bg-sakura-500/20"
          >
            <XIcon size={20} weight="bold" />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
