import { NavLink, Link } from 'react-router-dom';
import { LayoutDashboard, Globe, Mail, Users, Wand2, Sun, Moon, Info, LogOut } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api';
import type { StatusResponse } from '../types';
import { getTheme, setTheme } from '../theme';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard', end: true },
  { to: '/domains', icon: Globe, label: 'Domains' },
  { to: '/logs', icon: Mail, label: 'Mail Log' },
  { to: '/smtp-users', icon: Users, label: 'SMTP Users' },
  { to: '/wizard', icon: Wand2, label: 'Setup Wizard' },
];

export function Sidebar({ onLogout }: { onLogout: () => void }) {
  const [theme, setThemeState] = useState(getTheme());
  const [version, setVersion] = useState('');

  useEffect(() => {
    api.get<StatusResponse>('/status').then((data) => setVersion(data.version || 'dev')).catch(() => {});
  }, []);

  function toggleTheme() {
    const next = theme === 'light' ? 'dark' : 'light';
    setTheme(next);
    setThemeState(next);
  }

  return (
    <aside className="w-[200px] min-h-screen bg-slate-900 flex flex-col shrink-0">
      {/* Logo + version + theme toggle */}
      <div className="px-4 py-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Mail className="text-sky-400 w-5 h-5" />
            <span className="text-sky-400 font-semibold text-lg">SignPost</span>
          </div>
          <button
            onClick={toggleTheme}
            className="text-slate-500 hover:text-white transition-colors"
          >
            {theme === 'light' ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" />}
          </button>
        </div>
        {version && (
          <Link to="/release-notes" className="text-xs text-slate-500 hover:text-sky-400 mt-1 pl-7 block transition-colors">
            {version}
          </Link>
        )}
      </div>

      <div className="border-b border-slate-700 mx-4" />

      {/* Nav */}
      <nav className="flex-1 py-4">
        {navItems.map(({ to, icon: Icon, label, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              `flex items-center gap-3 px-4 py-2.5 text-sm transition-colors ${
                isActive
                  ? 'bg-slate-700 text-white border-l-2 border-sky-400'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800 border-l-2 border-transparent'
              }`
            }
          >
            <Icon className="w-4 h-4" />
            {label}
          </NavLink>
        ))}
      </nav>

      {/* Footer */}
      <div className="px-4 py-4 border-t border-slate-700 space-y-2">
        <NavLink
          to="/about"
          className={({ isActive }) =>
            `flex items-center gap-2 text-sm transition-colors ${
              isActive ? 'text-white' : 'text-slate-400 hover:text-white'
            }`
          }
        >
          <Info className="w-4 h-4" />
          About
        </NavLink>
        <button
          onClick={onLogout}
          className="flex items-center gap-2 text-sm text-slate-400 hover:text-white transition-colors w-full"
        >
          <LogOut className="w-4 h-4" />
          Logout
        </button>
      </div>
    </aside>
  );
}
