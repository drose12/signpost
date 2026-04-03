// web/src/App.tsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { Toaster } from 'sonner';
import { Layout } from './components/Layout';
import { LoginDialog } from './components/LoginDialog';
import { Dashboard } from './pages/Dashboard';
import { Domains } from './pages/Domains';
import { MailLog } from './pages/MailLog';
import { Wizard } from './pages/Wizard';
import { SMTPUsers } from './pages/SMTPUsers';
import { ReleaseNotes } from './pages/ReleaseNotes';
import { About } from './pages/About';
import { hasCredentials } from './api';
import { initTheme } from './theme';

export default function App() {
  const [loggedIn, setLoggedIn] = useState(hasCredentials());

  useEffect(() => { initTheme(); }, []);

  if (!loggedIn) {
    return <LoginDialog onLogin={() => setLoggedIn(true)} />;
  }

  return (
    <BrowserRouter>
      <Toaster position="top-right" />
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/domains" element={<Domains />} />
          <Route path="/logs" element={<MailLog />} />
          <Route path="/smtp-users" element={<SMTPUsers />} />
          <Route path="/wizard" element={<Wizard />} />
          <Route path="/release-notes" element={<ReleaseNotes />} />
          <Route path="/about" element={<About />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
