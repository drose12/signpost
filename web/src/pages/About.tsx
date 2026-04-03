// web/src/pages/About.tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '@/api';
import type { StatusResponse } from '@/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Mail, ExternalLink } from 'lucide-react';

export function About() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .get<StatusResponse>('/status')
      .then(setStatus)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-32 text-slate-500 dark:text-slate-400">
        Loading...
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-2xl">
      <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">About</h1>

      {/* Logo + version */}
      <Card className="dark:bg-slate-800">
        <CardContent className="pt-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-10 h-10 rounded-lg bg-sky-500 flex items-center justify-center">
              <Mail className="w-6 h-6 text-white" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-slate-800 dark:text-slate-100">SignPost</h2>
              <p className="text-sm text-slate-500 dark:text-slate-400">
                {status?.version || 'dev'}
              </p>
            </div>
          </div>
          <p className="text-slate-600 dark:text-slate-300">
            DKIM-signing SMTP relay with web admin UI. Ensures outgoing mail from local
            services passes DKIM, SPF, and DMARC validation.
          </p>
        </CardContent>
      </Card>

      {/* Tech stack */}
      <Card className="dark:bg-slate-800">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">
            Technology Stack
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-3 text-sm">
            {[
              { label: 'Backend', value: 'Go' },
              { label: 'Frontend', value: 'React + TypeScript' },
              { label: 'Mail Server', value: 'Maddy' },
              { label: 'Database', value: 'SQLite' },
              { label: 'Styling', value: 'Tailwind CSS' },
              { label: 'Components', value: 'shadcn/ui' },
            ].map(({ label, value }) => (
              <div key={label}>
                <span className="text-slate-500 dark:text-slate-400">{label}:</span>{' '}
                <span className="text-slate-800 dark:text-slate-100 font-medium">{value}</span>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* System info */}
      <Card className="dark:bg-slate-800">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">
            System Information
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-slate-500 dark:text-slate-400">Version</span>
              <span className="text-slate-800 dark:text-slate-100 font-mono">
                {status?.version || 'dev'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500 dark:text-slate-400">Schema Version</span>
              <span className="text-slate-800 dark:text-slate-100 font-mono">
                {status?.schema_version ?? '—'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500 dark:text-slate-400">Maddy Status</span>
              <span className="text-slate-800 dark:text-slate-100">
                {status?.maddy_status || '—'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500 dark:text-slate-400">Domains Configured</span>
              <span className="text-slate-800 dark:text-slate-100">
                {status?.domain_count ?? 0}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Links */}
      <Card className="dark:bg-slate-800">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">
            Links
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2 text-sm">
            <a
              href="https://github.com/drose-drcs/signpost"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 text-sky-600 dark:text-sky-400 hover:underline"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              GitHub Repository
            </a>
            <a
              href="https://github.com/drose-drcs/signpost/blob/main/README.md"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 text-sky-600 dark:text-sky-400 hover:underline"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              Documentation
            </a>
            <Link
              to="/release-notes"
              className="flex items-center gap-2 text-sky-600 dark:text-sky-400 hover:underline"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              Release Notes
            </Link>
          </div>
        </CardContent>
      </Card>

      {/* License */}
      <Card className="dark:bg-slate-800">
        <CardContent className="pt-4 pb-4">
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Licensed under the{' '}
            <a
              href="https://opensource.org/licenses/MIT"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sky-600 dark:text-sky-400 hover:underline"
            >
              MIT License
            </a>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
