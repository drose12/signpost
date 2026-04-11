// web/src/pages/Dashboard.tsx
import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { toast } from 'sonner';
import { api } from '@/api';
import type { StatusResponse, Domain, TestSendResponse, TLSResponse } from '@/types';
import { StatusBadge } from '@/components/StatusBadge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Switch } from '@/components/ui/switch';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { InfoIcon, Send, DownloadIcon, UploadIcon, ShieldCheck, RefreshCw } from 'lucide-react';

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
  const [tlsInfo, setTlsInfo] = useState<TLSResponse | null>(null);
  const [mode, setMode] = useState('self-signed');
  const [email, setEmail] = useState('');
  const [mailHostname, setMailHostname] = useState('');
  const [cfToken, setCfToken] = useState('');
  const [saving, setSaving] = useState(false);
  const [generating, setGenerating] = useState(false);

  async function loadTLS() {
    try {
      const info = await api.get<TLSResponse>('/tls');
      setTlsInfo(info);
      setMode(info.mode);
      setEmail(info.acme_email || '');
      setMailHostname(info.hostname || '');
    } catch { /* ignore */ }
  }

  useEffect(() => { loadTLS(); }, []);

  async function handleSave() {
    setSaving(true);
    try {
      const body: Record<string, string> = { mode };
      if (mailHostname) body.hostname = mailHostname;
      if (mode === 'acme') {
        body.email = email;
        body.provider = 'cloudflare';
        if (cfToken) body.cf_token = cfToken;
      }
      await api.put('/tls', body);
      toast.success(mode === 'acme' ? 'Switched to Let\'s Encrypt — Maddy will acquire cert' : 'Switched to self-signed');
      setCfToken('');
      await loadTLS();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update TLS');
    } finally {
      setSaving(false);
    }
  }

  async function handleRegenerate() {
    setGenerating(true);
    try {
      if (tlsInfo?.mode === 'acme') {
        await api.put('/tls', { mode: 'acme', email, provider: 'cloudflare' });
        toast.success('Renewal triggered — Maddy will re-acquire cert');
      } else {
        await api.post('/tls/generate-selfsigned');
        toast.success('Self-signed certificate regenerated');
      }
      await loadTLS();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed');
    } finally {
      setGenerating(false);
    }
  }

  function expiryColor(days?: number) {
    if (days == null) return 'bg-slate-400';
    if (days > 30) return 'bg-green-500';
    if (days >= 7) return 'bg-amber-500';
    return 'bg-red-500';
  }

  const dirty = mode !== tlsInfo?.mode || mailHostname !== (tlsInfo?.hostname || '') || (mode === 'acme' && (email !== (tlsInfo?.acme_email || '') || cfToken !== ''));

  return (
    <Card className="dark:bg-slate-800 col-span-1 sm:col-span-3">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-4 w-4 text-slate-500" />
          <CardTitle className="text-sm font-medium text-slate-600 dark:text-slate-300">TLS Certificate</CardTitle>
        </div>
        <Button size="sm" className="h-7 px-3 text-xs" onClick={handleRegenerate} disabled={generating || !tlsInfo?.cert_exists}>
          <RefreshCw className={`h-3 w-3 mr-1 ${generating ? 'animate-spin' : ''}`} />
          {tlsInfo?.mode === 'acme' ? 'Renew Now' : 'Regenerate'}
        </Button>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Left: Mode config */}
          <div className="space-y-3">
            <div>
              <Label className="text-xs">Mail Hostname</Label>
              <Input className="h-8 text-sm" value={mailHostname} onChange={e => setMailHostname(e.target.value)} placeholder="mail.example.com" />
            </div>
            <div>
              <Label className="text-xs">Mode</Label>
              <Select value={mode} onValueChange={setMode}>
                <SelectTrigger className="h-8 text-sm"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="self-signed">Self-Signed</SelectItem>
                  <SelectItem value="acme">Let's Encrypt</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {mode === 'acme' && (
              <>
                <div>
                  <Label className="text-xs">ACME Email</Label>
                  <Input className="h-8 text-sm" value={email} onChange={e => setEmail(e.target.value)} placeholder="you@example.com" />
                </div>
                <div>
                  <Label className="text-xs">Cloudflare API Token</Label>
                  <Input className="h-8 text-sm font-mono" type="password" value={cfToken}
                    onChange={e => setCfToken(e.target.value)}
                    placeholder={tlsInfo?.has_cf_token ? '(stored)' : 'Enter token'} />
                </div>
              </>
            )}

            {dirty && (
              <Button size="sm" className="h-8 text-xs w-full" onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : 'Save TLS Configuration'}
              </Button>
            )}
          </div>

          {/* Right: Cert details */}
          <div className="space-y-2 text-sm">
            {tlsInfo?.cert_exists ? (
              <>
                <div className="flex items-center gap-2">
                  <span className={`w-2 h-2 rounded-full ${expiryColor(tlsInfo.cert_days_remaining)} shrink-0`} />
                  <span className="font-medium text-slate-800 dark:text-slate-100">
                    {tlsInfo.cert_days_remaining != null ? `${tlsInfo.cert_days_remaining} days remaining` : 'Certificate active'}
                  </span>
                </div>
                <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs text-slate-600 dark:text-slate-400">
                  <span className="font-medium">Issuer</span>
                  <span className="truncate">{tlsInfo.cert_issuer}</span>
                  <span className="font-medium">Subject</span>
                  <span>{tlsInfo.cert_subject}</span>
                  {tlsInfo.cert_sans && tlsInfo.cert_sans.length > 0 && (
                    <>
                      <span className="font-medium">SANs</span>
                      <span>{tlsInfo.cert_sans.join(', ')}</span>
                    </>
                  )}
                  <span className="font-medium">Valid</span>
                  <span>{tlsInfo.cert_not_before ? new Date(tlsInfo.cert_not_before).toLocaleDateString() : '—'} — {tlsInfo.cert_not_after ? new Date(tlsInfo.cert_not_after).toLocaleDateString() : '—'}</span>
                  <span className="font-medium">Serial</span>
                  <span className="font-mono truncate">{tlsInfo.cert_serial}</span>
                </div>
              </>
            ) : (
              <div className="flex items-center gap-2 text-slate-500">
                <span className="w-2 h-2 rounded-full bg-amber-500 shrink-0" />
                No certificate found
              </div>
            )}
          </div>
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

function SystemBackupCard() {
  const [restoring, setRestoring] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const importFileRef = useRef<HTMLInputElement>(null);

  async function handleBackup() {
    try {
      const blob = await api.blob('/backup');
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      const date = new Date().toISOString().slice(0, 10);
      a.download = `signpost-backup-${date}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast.success('Backup downloaded');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Backup failed');
    }
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setPendingFile(file);
    setShowConfirm(true);
    if (importFileRef.current) importFileRef.current.value = '';
  }

  async function handleRestore() {
    if (!pendingFile) return;
    try {
      setRestoring(true);
      const text = await pendingFile.text();
      const payload = JSON.parse(text);
      const result = await api.post<{ domains: number; smtp_users: number; settings_keys: number }>('/backup/restore', payload);
      toast.success(`Restored ${result.domains} domain(s), ${result.smtp_users} user(s), ${result.settings_keys} setting(s)`);
      setShowConfirm(false);
      setPendingFile(null);
      // Reload page to reflect restored state
      window.location.reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Restore failed');
    } finally {
      setRestoring(false);
    }
  }

  return (
    <>
      <Card className="dark:bg-slate-800">
        <CardContent className="pt-4 pb-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-500 dark:text-slate-400">System Backup</span>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" className="h-7 px-3 text-xs" onClick={handleBackup}>
                <DownloadIcon className="h-3.5 w-3.5 mr-1" />
                Backup
              </Button>
              <Button variant="outline" size="sm" className="h-7 px-3 text-xs" onClick={() => importFileRef.current?.click()}>
                <UploadIcon className="h-3.5 w-3.5 mr-1" />
                Restore
              </Button>
              <input
                ref={importFileRef}
                type="file"
                accept=".json"
                className="hidden"
                onChange={handleFileSelect}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <Dialog open={showConfirm} onOpenChange={(open) => { if (!open) { setShowConfirm(false); setPendingFile(null); } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Restore from Backup</DialogTitle>
            <DialogDescription>
              This will restore all domains, DKIM keys, relay configs, SMTP users, and settings from{' '}
              <strong>{pendingFile?.name}</strong>. Existing entries will be updated.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setShowConfirm(false); setPendingFile(null); }}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleRestore} disabled={restoring}>
              {restoring ? 'Restoring...' : 'Restore'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function Dashboard() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function fetchData() {
      try {
        setLoading(true);
        setError(null);
        const statusData = await api.get<StatusResponse>('/status');
        setStatus(statusData);
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
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Card className="dark:bg-slate-800">
          <CardContent className="pt-4 pb-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-slate-500 dark:text-slate-400">Maddy</span>
              <StatusBadge status={status?.maddy_status ?? 'unknown'} />
            </div>
          </CardContent>
        </Card>

        <Card className="dark:bg-slate-800">
          <CardContent className="pt-4 pb-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-slate-500 dark:text-slate-400">Domains</span>
              <span className="text-lg font-semibold text-slate-800 dark:text-slate-100">{status?.domain_count ?? 0}</span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* TLS Certificate */}
      <TLSStatusCard />

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

      {/* System Backup */}
      <SystemBackupCard />
    </div>
  );
}
