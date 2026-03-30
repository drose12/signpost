// web/src/pages/Dashboard.tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '@/api';
import type { StatusResponse, MailLogEntry } from '@/types';
import { StatusBadge } from '@/components/StatusBadge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { ServerIcon, GlobeIcon, ShieldIcon, InfoIcon, NetworkIcon } from 'lucide-react';

function formatTime(ts: string): string {
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return ts;
  }
}

function statusVariant(status: string): 'default' | 'destructive' | 'secondary' | 'outline' {
  switch (status.toLowerCase()) {
    case 'sent': return 'default';
    case 'failed': return 'destructive';
    case 'deferred': return 'secondary';
    default: return 'outline';
  }
}

export function Dashboard() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [logs, setLogs] = useState<MailLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function fetchData() {
      try {
        setLoading(true);
        setError(null);
        const [statusData, logsData] = await Promise.all([
          api.get<StatusResponse>('/status'),
          api.get<MailLogEntry[]>('/logs?limit=10'),
        ]);
        setStatus(statusData);
        setLogs(logsData);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load dashboard data');
      } finally {
        setLoading(false);
      }
    }
    fetchData();
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-32 text-slate-500 dark:text-slate-400">
        Loading...
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-red-500 dark:text-red-400 p-4">
        Error: {error}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">Dashboard</h1>

      {/* Status cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card className="dark:bg-slate-800">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">Maddy Status</CardTitle>
            <ServerIcon className="h-4 w-4 text-slate-400" />
          </CardHeader>
          <CardContent>
            <StatusBadge status={status?.maddy_status ?? 'unknown'} />
          </CardContent>
        </Card>

        <Card className="dark:bg-slate-800">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">Domains</CardTitle>
            <GlobeIcon className="h-4 w-4 text-slate-400" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-slate-800 dark:text-slate-100">{status?.domain_count ?? 0}</div>
            <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">Domains configured</p>
          </CardContent>
        </Card>

        <Card className="dark:bg-slate-800">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">TLS Mode</CardTitle>
            <ShieldIcon className="h-4 w-4 text-slate-400" />
          </CardHeader>
          <CardContent>
            <div className="text-lg font-semibold text-slate-800 dark:text-slate-100 capitalize">{status?.tls_mode ?? '—'}</div>
          </CardContent>
        </Card>
      </div>

      {/* Listeners */}
      {status?.listeners && status.listeners.length > 0 && (
        <Card className="dark:bg-slate-800">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">Listeners</CardTitle>
            <NetworkIcon className="h-4 w-4 text-slate-400" />
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Service</TableHead>
                  <TableHead>Bind Address</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {status.listeners.map((listener) => (
                  <TableRow key={listener.name}>
                    <TableCell className="text-sm font-medium">{listener.name}</TableCell>
                    <TableCell className="text-sm font-mono text-slate-600 dark:text-slate-400">{listener.bind}</TableCell>
                    <TableCell><StatusBadge status={listener.status} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* Empty state */}
      {status?.domain_count === 0 && (
        <Alert className="border-blue-200 bg-blue-50 dark:bg-blue-950 dark:border-blue-800">
          <InfoIcon className="h-4 w-4 text-blue-600 dark:text-blue-400" />
          <AlertDescription className="text-blue-700 dark:text-blue-300">
            No domains configured.{' '}
            <Link to="/wizard" className="underline font-medium hover:no-underline">
              Run the Setup Wizard to get started.
            </Link>
          </AlertDescription>
        </Alert>
      )}

      {/* Recent activity */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100">Recent Activity</h2>
          <Link to="/logs" className="text-sm text-blue-600 dark:text-blue-400 hover:underline">
            View all
          </Link>
        </div>

        <Card className="dark:bg-slate-800">
          <CardContent className="p-0">
            {logs.length === 0 ? (
              <p className="text-slate-500 dark:text-slate-400 text-sm p-4 text-center">No recent activity</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Time</TableHead>
                    <TableHead>From</TableHead>
                    <TableHead>To</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((entry) => (
                    <TableRow key={entry.id}>
                      <TableCell className="text-xs text-slate-500 dark:text-slate-400 whitespace-nowrap">
                        {formatTime(entry.timestamp)}
                      </TableCell>
                      <TableCell className="text-sm max-w-[160px] truncate">{entry.from_addr}</TableCell>
                      <TableCell className="text-sm max-w-[160px] truncate">{entry.to_addr}</TableCell>
                      <TableCell>
                        <Badge variant={statusVariant(entry.status)}>{entry.status}</Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
