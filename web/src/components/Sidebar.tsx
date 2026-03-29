import { NavLink } from 'react-router-dom';
import { LayoutDashboard, Globe, Mail, Wand2, Sun, Moon } from 'lucide-react';
import { useState } from 'react';
import { getTheme, setTheme } from '../theme';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard', end: true },
  { to: '/domains', icon: Globe, label: 'Domains' },
  { to: '/logs', icon: Mail, label: 'Mail Log' },
  { to: '/wizard', icon: Wand2, label: 'Setup Wizard' },
];

export function Sidebar() {
  const [theme, setThemeState] = useState(getTheme());

  function toggleTheme() {
    const next = theme === 'light' ? 'dark' : 'light';
    setTheme(next);
    setThemeState(next);
  }

  return (
    <aside className="w-[200px] min-h-screen bg-slate-900 flex flex-col shrink-0">
      {/* Logo */}
      <div className="px-4 py-5 flex items-center gap-2">
        <Mail className="text-sky-400 w-5 h-5" />
        <span className="text-sky-400 font-semibold text-lg">SignPost</span>
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

      {/* Theme toggle */}
      <div className="px-4 py-4 border-t border-slate-700">
        <button
          onClick={toggleTheme}
          className="flex items-center gap-2 text-slate-400 hover:text-white text-sm"
        >
          {theme === 'light' ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" />}
          {theme === 'light' ? 'Dark mode' : 'Light mode'}
        </button>
      </div>
    </aside>
  );
}
