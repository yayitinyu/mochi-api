import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { api } from '../lib/api';
import { ROLE_ADMIN, type User } from '../lib/types';

interface AuthState {
  user: User | null;
  loading: boolean;
  isAdmin: boolean;
  setUser: (u: User | null) => void;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  async function refresh() {
    try {
      const u = await api.get<User>('/api/auth/me');
      setUser(u);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }

  async function logout() {
    await api.post('/api/auth/logout');
    setUser(null);
  }

  useEffect(() => {
    void refresh();
    const onUnauth = () => setUser(null);
    window.addEventListener('mochi:unauthorized', onUnauth);
    return () => window.removeEventListener('mochi:unauthorized', onUnauth);
  }, []);

  return (
    <AuthContext.Provider
      value={{ user, loading, isAdmin: (user?.role ?? 0) >= ROLE_ADMIN, setUser, refresh, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
