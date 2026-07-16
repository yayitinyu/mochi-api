import { useState, useEffect, type ReactNode } from 'react';
import { ListIcon, XIcon } from '@phosphor-icons/react';
import { Sidebar } from './Sidebar';

export function Layout({ title, children }: { title: string; children: ReactNode }) {
  const [drawerOpen, setDrawerOpen] = useState(false);

  useEffect(() => {
    if (drawerOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }

    const handleResize = () => {
      if (window.innerWidth >= 768) {
        setDrawerOpen(false);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => {
      document.body.style.overflow = '';
      window.removeEventListener('resize', handleResize);
    };
  }, [drawerOpen]);

  return (
    <div className="min-h-[100dvh] md:flex">
      {/* Static sidebar on desktop */}
      <div className="hidden md:block h-screen sticky top-0 overflow-y-auto">
        <Sidebar />
      </div>

      {/* Top bar on mobile */}
      <header className="sticky top-0 z-30 flex items-center gap-3 border-b border-white/60 bg-cream/85 px-4 py-3 backdrop-blur md:hidden dark:border-white/10">
        <button
          onClick={() => setDrawerOpen(true)}
          aria-label="打开菜单"
          className="rounded-xl p-1.5 text-ink transition hover:bg-sakura-50 dark:hover:bg-sakura-500/10"
        >
          <ListIcon size={22} weight="bold" />
        </button>
        <img
          src="/mochi-wordmark.svg"
          alt="Mochi"
          width={115}
          height={40}
          className="h-8 w-auto"
        />
      </header>

      {/* Drawer on mobile */}
      {drawerOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div
            className="absolute inset-0 bg-ink/30 backdrop-blur-sm"
            onClick={() => setDrawerOpen(false)}
          />
          <div className="absolute inset-y-0 left-0 w-64 overflow-y-auto rounded-r-3xl bg-cream shadow-2xl">
            <button
              onClick={() => setDrawerOpen(false)}
              aria-label="关闭菜单"
              className="absolute right-3 top-3 rounded-full p-1.5 text-ink-soft transition hover:bg-sakura-50 dark:hover:bg-sakura-500/10"
            >
              <XIcon size={18} weight="bold" />
            </button>
            <Sidebar onNavigate={() => setDrawerOpen(false)} />
          </div>
        </div>
      )}

      <main className="flex-1 overflow-x-hidden p-4 md:p-6 md:pr-6">
        <h1 className="mb-5 px-1 text-2xl font-extrabold text-ink">{title}</h1>
        {children}
      </main>
    </div>
  );
}
