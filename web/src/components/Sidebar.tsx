import { useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import {
  ChartLineUpIcon,
  GearSixIcon,
  KeyIcon,
  MoonIcon,
  PlugsConnectedIcon,
  TagIcon,
  ScrollIcon,
  SignOutIcon,
  SunIcon,
  UsersIcon,
  type Icon,
} from '@phosphor-icons/react';
import { useAuth } from '../context/AuthContext';
import { isDark, toggleTheme } from '../lib/theme';
import { Logo } from './Logo';

interface NavItem {
  to: string;
  label: string;
  icon: Icon;
  adminOnly?: boolean;
}

const items: NavItem[] = [
  { to: '/', label: '仪表盘', icon: ChartLineUpIcon },
  { to: '/keys', label: 'API 密钥', icon: KeyIcon },
  { to: '/logs', label: '调用日志', icon: ScrollIcon },
  { to: '/channels', label: '渠道管理', icon: PlugsConnectedIcon, adminOnly: true },
  { to: '/prices', label: '模型价格', icon: TagIcon, adminOnly: true },
  { to: '/users', label: '用户管理', icon: UsersIcon, adminOnly: true },
  { to: '/settings', label: '站点设置', icon: GearSixIcon, adminOnly: true },
];

export function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const { user, isAdmin, logout } = useAuth();
  const navigate = useNavigate();
  const [dark, setDark] = useState(isDark());

  async function onLogout() {
    await logout();
    navigate('/login');
  }

  return (
    <aside className="flex h-full w-60 shrink-0 flex-col gap-2 p-4">
      <div className="flex items-center gap-2.5 px-3 py-4">
        <Logo size={40} className="drop-shadow-sm" />
        <div className="flex-1">
          <div className="text-lg font-extrabold leading-tight text-ink">Mochi</div>
          <div className="text-xs text-ink-soft">API 聚合网关</div>
        </div>
        <button
          onClick={() => setDark(toggleTheme())}
          aria-label={dark ? '切换到浅色模式' : '切换到深色模式'}
          className="rounded-full p-2 text-ink-soft transition hover:bg-sakura-50 hover:text-ink dark:hover:bg-sakura-500/10"
        >
          {dark ? <SunIcon size={18} weight="duotone" /> : <MoonIcon size={18} weight="duotone" />}
        </button>
      </div>

      <nav className="flex flex-1 flex-col gap-1">
        {items
          .filter((it) => !it.adminOnly || isAdmin)
          .map(({ to, label, icon: IconCmp }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              onClick={onNavigate}
              className={({ isActive }) =>
                `flex items-center gap-3 rounded-2xl px-3.5 py-2.5 text-sm font-bold transition ${
                  isActive
                    ? 'bg-sakura-100 text-sakura-700 dark:bg-sakura-500/20 dark:text-sakura-200'
                    : 'text-ink-soft hover:bg-sakura-50 hover:text-ink dark:hover:bg-sakura-500/10'
                }`
              }
            >
              <IconCmp size={20} weight="duotone" />
              {label}
            </NavLink>
          ))}
      </nav>

      <div className="rounded-2xl bg-surface/70 p-3">
        <div className="mb-2 px-1">
          <div className="truncate text-sm font-bold text-ink">{user?.username}</div>
          <div className="text-xs text-ink-soft">{isAdmin ? '管理员' : '普通用户'}</div>
        </div>
        <button
          onClick={onLogout}
          className="flex w-full items-center gap-2 rounded-xl px-2 py-2 text-sm font-bold text-ink-soft transition hover:bg-sakura-50 hover:text-sakura-600 dark:hover:bg-sakura-500/10 dark:hover:text-sakura-300"
        >
          <SignOutIcon size={18} weight="bold" />
          退出登录
        </button>
      </div>
    </aside>
  );
}
