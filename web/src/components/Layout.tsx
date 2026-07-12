import type { ReactNode } from 'react';
import { Sidebar } from './Sidebar';

export function Layout({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex min-h-[100dvh]">
      <Sidebar />
      <main className="flex-1 overflow-x-hidden p-4 pr-6 md:p-6">
        <h1 className="mb-5 px-1 text-2xl font-extrabold text-ink">{title}</h1>
        {children}
      </main>
    </div>
  );
}
