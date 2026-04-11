import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';

export function Layout({ onLogout }: { onLogout: () => void }) {
  return (
    <div className="flex h-screen">
      <Sidebar onLogout={onLogout} />
      <main className="flex-1 overflow-auto bg-slate-50 dark:bg-slate-900 p-6">
        <Outlet />
      </main>
    </div>
  );
}
