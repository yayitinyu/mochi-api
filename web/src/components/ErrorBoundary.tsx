import { Component, type ReactNode } from 'react';

interface State {
  error: Error | null;
}

// Catches render errors so a bug shows a friendly message
// instead of unmounting the whole app into a blank page.
export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <div className="grid min-h-[100dvh] place-items-center p-6">
          <div className="w-full max-w-md rounded-3xl border border-white bg-surface/85 p-8 text-center shadow-xl dark:border-white/10">
            <div className="mb-3 text-4xl">🥀</div>
            <h1 className="mb-2 text-lg font-extrabold text-ink">页面出了点问题</h1>
            <p className="mb-4 break-all text-xs text-ink-soft">{this.state.error.message}</p>
            <button
              onClick={() => window.location.reload()}
              className="rounded-full bg-sakura-500 px-5 py-2 text-sm font-bold text-white transition hover:bg-sakura-600"
            >
              刷新页面
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
