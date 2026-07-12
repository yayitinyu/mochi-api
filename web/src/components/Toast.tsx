import { createContext, useCallback, useContext, useState, type ReactNode } from 'react';
import { CheckCircleIcon, WarningCircleIcon } from '@phosphor-icons/react';

type ToastKind = 'success' | 'error';
interface ToastItem {
  id: number;
  kind: ToastKind;
  text: string;
}

const ToastContext = createContext<(kind: ToastKind, text: string) => void>(() => {});

let nextId = 1;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const push = useCallback((kind: ToastKind, text: string) => {
    const id = nextId++;
    setItems((prev) => [...prev, { id, kind, text }]);
    setTimeout(() => setItems((prev) => prev.filter((t) => t.id !== id)), 3000);
  }, []);

  return (
    <ToastContext.Provider value={push}>
      {children}
      <div className="pointer-events-none fixed left-1/2 top-6 z-[60] flex -translate-x-1/2 flex-col gap-2">
        {items.map((t) => (
          <div
            key={t.id}
            className="flex items-center gap-2 rounded-full border border-white bg-surface/95 px-4 py-2.5 text-sm font-bold text-ink shadow-lg backdrop-blur dark:border-white/10"
          >
            {t.kind === 'success' ? (
              <CheckCircleIcon size={18} weight="fill" className="text-matcha" />
            ) : (
              <WarningCircleIcon size={18} weight="fill" className="text-sakura-500" />
            )}
            {t.text}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  return useContext(ToastContext);
}
