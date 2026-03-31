// web/src/pages/Dashboard.tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { toast } from 'sonner';
import { api } from '@/api';
import type { StatusResponse, MailLogEntry, Domain, TestSendResponse } from '@/types';
import { StatusBadge } from '@/components/StatusBadge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Switch } from '@/components/ui/switch';
import { ServerIcon, GlobeIcon, InfoIcon, Send } from 'lucide-react';

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

function TestEmailCard() {
  const [domains, setDomains] = useState<Domain[]>([]);
  const [selectedDomain, setSelectedDomain] = useState('');
  const [fromUser, setFromUser] = useState('test');
  const [to, setTo] = useState('');
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<TestSendResponse | null>(null);

  useEffect(() => {
    api.get<Domain[]>('/domains').then((data) => {
      setDomains(data);
      if (data.length > 0) setSelectedDomain(data[0].name);
    }).catch(() => {});
  }, []);

  async function handleSend(e: React.FormEvent) {
    e.preventDefault();
    if (!to.trim() || !selectedDomain) return;
    setSending(true);
    setResult(null);
    try {
      const fromAddr = `${fromUser.trim() || 'test'}@${selectedDomain}`;
      const resp = await api.post<TestSendResponse>('/test/send', {
        from: fromAddr,
        to: to.trim(),
        subject: 'SignPost Test Email',
        body: `This is a test email sent from SignPost for domain ${selectedDomain}.`,
      });
      setResult(resp);
      if (resp.status === 'sent' || resp.status === 'queued') {
        toast.success('Test email sent');
      } else {
        toast.error(resp.error || 'Test email failed');
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to send';
      setResult({ status: 'failed', error: msg });
      toast.error(msg);
    } finally {
      setSending(false);
    }
  }

  return (
    <Card className="dark:bg-slate-800">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">Send Test Email</CardTitle>
        <Send className="h-4 w-4 text-slate-400" />
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSend} className="flex items-end gap-3 flex-wrap">
          <div className="space-y-1.5">
            <Label htmlFor="test-from-user" className="text-xs">From</Label>
            <div className="flex items-center">
              <Input
                id="test-from-user"
                value={fromUser}
                onChange={(e) => setFromUser(e.target.value)}
                placeholder="test"
                className="h-9 rounded-r-none w-[120px]"
              />
              <span className="h-9 flex items-center px-2 border border-l-0 rounded-r-md bg-slate-100 dark:bg-slate-700 text-sm text-slate-500 dark:text-slate-400">@</span>
              <Select value={selectedDomain} onValueChange={setSelectedDomain}>
                <SelectTrigger className="h-9 rounded-l-none border-l-0 min-w-[140px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {domains.map((d) => (
                    <SelectItem key={d.id} value={d.name}>{d.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-1.5 flex-1 min-w-[200px]">
            <Label htmlFor="test-to" className="text-xs">To</Label>
            <Input
              id="test-to"
              type="email"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              placeholder="recipient@example.com"
              className="h-9"
              required
            />
          </div>
          <Button type="submit" size="sm" disabled={sending || !to.trim() || !selectedDomain}>
            <Send className="h-3.5 w-3.5 mr-1.5" />
            {sending ? 'Sending...' : 'Send'}
          </Button>
        </form>
        {result && (
          <div className="mt-3">
            {result.status === 'sent' || result.status === 'queued' ? (
              <p className="text-sm text-green-600 dark:text-green-400">Sent as {fromUser.trim() || 'test'}@{selectedDomain}</p>
            ) : (
              <p className="text-sm text-red-500">{result.error}</p>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function TLSStatusCard() {
  const [tlsInfo, setTlsInfo] = useState<{ mode: string; cert_path?: string; cert_exists?: boolean } | null>(null);
  const [generating, setGenerating] = useState(false);

  useEffect(() => {
    api.get<{ mode: string; cert_path?: string; cert_exists?: boolean }>('/tls')
      .then(setTlsInfo)
      .catch(() => {});
  }, []);

  async function handleGenerate() {
    setGenerating(true);
    try {
      await api.post('/tls/generate-selfsigned');
      toast.success('Self-signed certificate generated');
      const info = await api.get<{ mode: string; cert_path?: string; cert_exists?: boolean }>('/tls');
      setTlsInfo(info);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to generate certificate');
    } finally {
      setGenerating(false);
    }
  }

  return (
    <Card className="dark:bg-slate-800">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">TLS</CardTitle>
        <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={handleGenerate} disabled={generating || !tlsInfo}>
          {generating ? '...' : tlsInfo?.cert_exists ? 'Regen' : 'Generate'}
        </Button>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-2">
          {tlsInfo?.cert_exists && <span className="w-2 h-2 rounded-full bg-green-500 shrink-0" />}
          {tlsInfo?.cert_exists === false && <span className="w-2 h-2 rounded-full bg-amber-500 shrink-0" />}
          <span className="text-lg font-semibold text-slate-800 dark:text-slate-100 capitalize">
            {tlsInfo?.mode || '—'}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}

function SMTPPortsCard() {
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [toggling, setToggling] = useState<string | null>(null);
  const [userCount, setUserCount] = useState(0);

  useEffect(() => {
    Promise.all([
      api.get<Record<string, string>>('/settings'),
      api.get<unknown[]>('/smtp-users'),
    ]).then(([settingsData, users]) => {
      setSettings(settingsData);
      setUserCount(users.length);
      setLoading(false);
    }).catch(() => setLoading(false));
  }, []);

  async function toggle(key: string, current: boolean) {
    const newValue = current ? 'false' : 'true';
    try {
      setToggling(key);
      await api.put('/settings', { [key]: newValue });
      setSettings((prev) => ({ ...prev, [key]: newValue }));
      toast.success(`${key === 'smtp_enabled' ? 'Port 25 (SMTP)' : 'Port 587 (Submission)'} ${newValue === 'true' ? 'enabled' : 'disabled'}`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to update setting';
      toast.error(msg);
    } finally {
      setToggling(null);
    }
  }

  if (loading) return null;

  const smtpEnabled = settings.smtp_enabled !== 'false';
  const submissionEnabled = settings.submission_enabled === 'true';

  return (
    <Card className="dark:bg-slate-800">
      <CardContent className="pt-4 space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium text-slate-800 dark:text-slate-100">Port 25 (SMTP)</p>
            <p className="text-xs text-slate-500 dark:text-slate-400">Network-trust, no authentication</p>
          </div>
          <Switch
            checked={smtpEnabled}
            onCheckedChange={() => toggle('smtp_enabled', smtpEnabled)}
            disabled={toggling !== null}
          />
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div>
              <p className="text-sm font-medium text-slate-800 dark:text-slate-100">
                Port 587 (Submission)
                {submissionEnabled && userCount > 0 && (
                  <span className="text-xs font-normal text-green-600 dark:text-green-400 ml-2">({userCount} user{userCount !== 1 ? 's' : ''} configured)</span>
                )}
              </p>
              {submissionEnabled && userCount === 0 && (
                <p className="text-xs text-amber-600 dark:text-amber-400">No users — <a href="/smtp-users" className="underline">add one</a></p>
              )}
              {!submissionEnabled && (
                <p className="text-xs text-slate-500 dark:text-slate-400">SMTP AUTH (username/password)</p>
              )}
            </div>
          </div>
          <Switch
            checked={submissionEnabled}
            onCheckedChange={() => toggle('submission_enabled', submissionEnabled)}
            disabled={toggling !== null}
          />
        </div>
      </CardContent>
    </Card>
  );
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
          </CardContent>
        </Card>

        <TLSStatusCard />
      </div>

      {/* SMTP Ports */}
      <SMTPPortsCard />

      {/* Listeners */}
      {status?.listeners && status.listeners.length > 0 && (
        <Card className="dark:bg-slate-800">
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

      {/* Send Test Email */}
      {status && status.domain_count > 0 && <TestEmailCard />}

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
