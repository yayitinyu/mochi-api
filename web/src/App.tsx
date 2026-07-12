import { Navigate, Route, Routes } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { ToastProvider } from './components/Toast';
import { ErrorBoundary } from './components/ErrorBoundary';
import { Layout } from './components/Layout';
import { LoginPage } from './pages/LoginPage';
import { DashboardPage } from './pages/DashboardPage';
import { KeysPage } from './pages/KeysPage';
import { LogsPage } from './pages/LogsPage';
import { ChannelsPage } from './pages/ChannelsPage';
import { PricesPage } from './pages/PricesPage';
import type { ReactNode } from 'react';

function Loading() {
  return (
    <div className="grid min-h-[100dvh] place-items-center text-ink-soft">
      <div className="animate-pulse text-lg font-bold">加载中… 🌸</div>
    </div>
  );
}

function Guard({ children, admin }: { children: ReactNode; admin?: boolean }) {
  const { user, loading, isAdmin } = useAuth();
  if (loading) return <Loading />;
  if (!user) return <Navigate to="/login" replace />;
  if (admin && !isAdmin) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function Router() {
  const { user, loading } = useAuth();
  return (
    <Routes>
      <Route
        path="/login"
        element={loading ? <Loading /> : user ? <Navigate to="/" replace /> : <LoginPage />}
      />
      <Route
        path="/"
        element={
          <Guard>
            <Layout title="仪表盘">
              <DashboardPage />
            </Layout>
          </Guard>
        }
      />
      <Route
        path="/keys"
        element={
          <Guard>
            <Layout title="API 密钥">
              <KeysPage />
            </Layout>
          </Guard>
        }
      />
      <Route
        path="/logs"
        element={
          <Guard>
            <Layout title="调用日志">
              <LogsPage />
            </Layout>
          </Guard>
        }
      />
      <Route
        path="/channels"
        element={
          <Guard admin>
            <Layout title="渠道管理">
              <ChannelsPage />
            </Layout>
          </Guard>
        }
      />
      <Route
        path="/prices"
        element={
          <Guard admin>
            <Layout title="模型价格">
              <PricesPage />
            </Layout>
          </Guard>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export function App() {
  return (
    <ErrorBoundary>
      <AuthProvider>
        <ToastProvider>
          <Router />
        </ToastProvider>
      </AuthProvider>
    </ErrorBoundary>
  );
}
