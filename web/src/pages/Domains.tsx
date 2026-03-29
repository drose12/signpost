// web/src/pages/Domains.tsx
import { useEffect, useState, useCallback } from 'react';
import { toast } from 'sonner';
import { api } from '@/api';
import type { Domain, DNSRecord, RelayConfig, DKIMGenerateResponse } from '@/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { PlusIcon, CopyIcon, InfoIcon, KeyIcon, TrashIcon } from 'lucide-react';

// ---------------------------------------------------------------------------
// DNS Records Tab
// ---------------------------------------------------------------------------

function DnsRecordsTab({ domain }: { domain: Domain }) {
  const [records, setRecords] = useState<DNSRecord[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api.get<DNSRecord[]>(`/domains/${domain.id}/dns`)
      .then((data) => { if (!cancelled) { setRecords(data); setLoading(false); } })
      .catch((err) => {
        if (!cancelled) {
          toast.error(err instanceof Error ? err.message : 'Failed to load DNS records');
          setLoading(false);
        }
      });
    return () => { cancelled = true; };
  }, [domain.id]);

  function copyValue(value: string) {
    navigator.clipboard.writeText(value).then(() => toast.success('Copied!')).catch(() => toast.error('Copy failed'));
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Alert className="border-blue-200 bg-blue-50 dark:bg-blue-950 dark:border-blue-800 flex-1">
          <InfoIcon className="h-4 w-4 text-blue-600 dark:text-blue-400" />
          <AlertDescription className="text-blue-700 dark:text-blue-300 text-sm">
            DNS changes can take up to 24–48 hours to propagate.
          </AlertDescription>
        </Alert>
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="ml-4">
                <Button variant="outline" disabled>Validate DNS</Button>
              </span>
            </TooltipTrigger>
            <TooltipContent>Coming soon</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>

      {loading ? (
        <p className="text-slate-500 dark:text-slate-400 text-sm">Loading DNS records...</p>
      ) : records.length === 0 ? (
        <p className="text-slate-500 dark:text-slate-400 text-sm">No DNS records available. Generate DKIM keys first.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-20">Type</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Value</TableHead>
              <TableHead className="w-16"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {records.map((record, i) => (
              <TableRow key={i}>
                <TableCell><Badge variant="outline">{record.type}</Badge></TableCell>
                <TableCell className="font-mono text-xs">{record.name}</TableCell>
                <TableCell className="font-mono text-xs max-w-xs truncate" title={record.value}>{record.value}</TableCell>
                <TableCell>
                  <Button variant="ghost" size="icon" onClick={() => copyValue(record.value)} aria-label="Copy value">
                    <CopyIcon className="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Relay Config Tab
// ---------------------------------------------------------------------------

type RelayMethod = 'gmail' | 'isp' | 'direct' | 'custom';

const METHOD_LABELS: Record<RelayMethod, string> = {
  gmail: 'Gmail SMTP',
  isp: 'ISP Relay',
  direct: 'Direct Delivery',
  custom: 'Custom SMTP',
};

const METHOD_DEFAULTS: Record<RelayMethod, { host: string; port: string }> = {
  gmail: { host: 'smtp.gmail.com', port: '587' },
  isp: { host: '', port: '587' },
  direct: { host: '', port: '25' },
  custom: { host: '', port: '587' },
};

function RelayConfigTab({ domain }: { domain: Domain }) {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [method, setMethod] = useState<RelayMethod>('direct');
  const [host, setHost] = useState('');
  const [port, setPort] = useState('25');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [starttls, setStarttls] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api.get<RelayConfig>(`/domains/${domain.id}/relay`)
      .then((data) => {
        if (!cancelled) {
          const m = (data.method as RelayMethod) || 'direct';
          setMethod(m);
          setHost(data.host ?? METHOD_DEFAULTS[m].host);
          setPort(String(data.port || METHOD_DEFAULTS[m].port));
          setUsername(data.username ?? '');
          setPassword(''); // never pre-fill password
          setStarttls(data.starttls);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          toast.error(err instanceof Error ? err.message : 'Failed to load relay config');
          setLoading(false);
        }
      });
    return () => { cancelled = true; };
  }, [domain.id]);

  function handleMethodChange(val: string) {
    const m = val as RelayMethod;
    setMethod(m);
    if (m === 'gmail') {
      setHost('smtp.gmail.com');
      setPort('587');
      setStarttls(true);
    } else if (m === 'direct') {
      setHost('');
      setPort('25');
    }
  }

  async function handleSave() {
    setSaving(true);
    try {
      await api.put(`/domains/${domain.id}/relay`, {
        method,
        host: host || undefined,
        port: parseInt(port, 10) || 25,
        username: username || undefined,
        password: password || undefined,
        starttls,
      });
      toast.success('Relay config saved');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save relay config');
    } finally {
      setSaving(false);
    }
  }

  const showFields = method !== 'direct';

  return (
    <div className="space-y-4 max-w-md">
      {loading ? (
        <p className="text-slate-500 dark:text-slate-400 text-sm">Loading relay config...</p>
      ) : (
        <>
          <div className="space-y-2">
            <Label htmlFor="relay-method">Method</Label>
            <Select value={method} onValueChange={handleMethodChange}>
              <SelectTrigger id="relay-method">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(METHOD_LABELS) as RelayMethod[]).map((m) => (
                  <SelectItem key={m} value={m}>{METHOD_LABELS[m]}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {method === 'direct' ? (
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Mail will be delivered directly without an intermediate relay.
            </p>
          ) : null}

          {showFields && (
            <>
              <div className="space-y-2">
                <Label htmlFor="relay-host">Host</Label>
                <Input id="relay-host" value={host} onChange={(e) => setHost(e.target.value)}
                  placeholder={method === 'gmail' ? 'smtp.gmail.com' : 'smtp.example.com'} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="relay-port">Port</Label>
                <Input id="relay-port" type="number" value={port} onChange={(e) => setPort(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="relay-username">Username</Label>
                <Input id="relay-username" value={username} onChange={(e) => setUsername(e.target.value)}
                  placeholder={method === 'gmail' ? 'you@gmail.com' : 'username'} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="relay-password">Password</Label>
                <Input id="relay-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)}
                  placeholder={method === 'gmail' ? 'App password' : 'Password'} />
              </div>
              <div className="flex items-center gap-3">
                <Switch id="relay-starttls" checked={starttls} onCheckedChange={setStarttls} />
                <Label htmlFor="relay-starttls">STARTTLS</Label>
              </div>
            </>
          )}

          <div className="flex gap-2 pt-2">
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save'}
            </Button>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span>
                    <Button variant="outline" disabled>Test Connection</Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>Coming soon</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// DKIM Keys Tab
// ---------------------------------------------------------------------------

function DkimKeysTab({ domain, onRefresh }: { domain: Domain; onRefresh: () => void }) {
  const [generating, setGenerating] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [newDnsValue, setNewDnsValue] = useState<string | null>(null);

  const hasKeys = Boolean(domain.dkim_key_path);

  async function generateKeys() {
    setConfirmOpen(false);
    setGenerating(true);
    try {
      const result = await api.post<DKIMGenerateResponse>(`/domains/${domain.id}/dkim/generate`);
      setNewDnsValue(result.dns_record_value);
      toast.success('DKIM keys generated successfully');
      onRefresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to generate DKIM keys');
    } finally {
      setGenerating(false);
    }
  }

  function copyDnsValue() {
    if (!newDnsValue) return;
    navigator.clipboard.writeText(newDnsValue).then(() => toast.success('Copied!')).catch(() => toast.error('Copy failed'));
  }

  return (
    <div className="space-y-4 max-w-lg">
      {hasKeys ? (
        <div className="space-y-2 text-sm text-slate-700 dark:text-slate-300">
          <div className="flex items-center gap-2">
            <KeyIcon className="h-4 w-4 text-green-500" />
            <span className="font-medium">Keys generated</span>
          </div>
          <div className="space-y-1 text-slate-600 dark:text-slate-400">
            <p><span className="font-medium">Selector:</span> {domain.dkim_selector}</p>
            <p><span className="font-medium">Key path:</span> <span className="font-mono text-xs">{domain.dkim_key_path}</span></p>
            <p><span className="font-medium">Last updated:</span> {new Date(domain.updated_at).toLocaleString()}</p>
          </div>
        </div>
      ) : (
        <div className="text-sm text-slate-500 dark:text-slate-400 space-y-1">
          <p>No DKIM keys generated yet.</p>
          <p>Generate keys to enable DKIM signing for outbound mail.</p>
        </div>
      )}

      {newDnsValue && (
        <div className="space-y-2">
          <p className="text-sm font-medium text-slate-700 dark:text-slate-300">New DNS TXT record value:</p>
          <div className="flex items-start gap-2">
            <div className="font-mono text-xs bg-slate-100 dark:bg-slate-700 p-2 rounded flex-1 break-all">{newDnsValue}</div>
            <Button variant="ghost" size="icon" onClick={copyDnsValue} aria-label="Copy DNS value">
              <CopyIcon className="h-4 w-4" />
            </Button>
          </div>
          <p className="text-xs text-slate-500 dark:text-slate-400">Update your DNS records with this value.</p>
        </div>
      )}

      <div>
        {hasKeys ? (
          <Button variant="outline" onClick={() => setConfirmOpen(true)} disabled={generating}>
            {generating ? 'Regenerating...' : 'Regenerate Keys'}
          </Button>
        ) : (
          <Button onClick={generateKeys} disabled={generating}>
            {generating ? 'Generating...' : 'Generate DKIM Keys'}
          </Button>
        )}
      </div>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Regenerate DKIM Keys?</DialogTitle>
            <DialogDescription>
              Warning: Regenerating keys will invalidate your existing DNS record. You will need to update your DNS with the new value.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>Cancel</Button>
            <Button variant="destructive" onClick={generateKeys} disabled={generating}>
              {generating ? 'Regenerating...' : 'Regenerate'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Settings Tab
// ---------------------------------------------------------------------------

function SettingsTab({ domain, onDeleted }: { domain: Domain; onDeleted: () => void }) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleDelete() {
    setDeleting(true);
    try {
      await api.del(`/domains/${domain.id}`);
      toast.success(`Domain "${domain.name}" deleted`);
      onDeleted();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete domain');
    } finally {
      setDeleting(false);
      setConfirmOpen(false);
    }
  }

  return (
    <div className="space-y-4 max-w-md">
      <div className="text-sm text-slate-600 dark:text-slate-400 space-y-1">
        <p><span className="font-medium">Domain:</span> {domain.name}</p>
        <p><span className="font-medium">Created:</span> {new Date(domain.created_at).toLocaleString()}</p>
      </div>

      <div className="pt-2">
        <Button variant="destructive" onClick={() => setConfirmOpen(true)}>
          <TrashIcon className="h-4 w-4 mr-2" />
          Delete Domain
        </Button>
      </div>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Domain?</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete <strong>{domain.name}</strong>? This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)} disabled={deleting}>Cancel</Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleting}>
              {deleting ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Add Domain Dialog
// ---------------------------------------------------------------------------

function AddDomainDialog({ open, onOpenChange, onCreated }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (domain: Domain) => void;
}) {
  const [name, setName] = useState('');
  const [selector, setSelector] = useState('signpost');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setSubmitting(true);
    setError(null);
    try {
      const domain = await api.post<Domain>('/domains', { name: name.trim(), dkim_selector: selector.trim() || 'signpost' });
      onCreated(domain);
      setName('');
      setSelector('signpost');
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create domain');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Domain</DialogTitle>
          <DialogDescription>Configure a new domain for DKIM signing and mail relay.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="new-domain-name">Domain Name <span className="text-red-500">*</span></Label>
            <Input id="new-domain-name" value={name} onChange={(e) => setName(e.target.value)}
              placeholder="example.com" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-domain-selector">DKIM Selector</Label>
            <Input id="new-domain-selector" value={selector} onChange={(e) => setSelector(e.target.value)}
              placeholder="signpost" />
          </div>
          {error && <p className="text-red-500 text-sm">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={submitting}>Cancel</Button>
            <Button type="submit" disabled={submitting || !name.trim()}>
              {submitting ? 'Adding...' : 'Add Domain'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Main Domains Page
// ---------------------------------------------------------------------------

export function Domains() {
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [addOpen, setAddOpen] = useState(false);

  const fetchDomains = useCallback(async () => {
    try {
      const data = await api.get<Domain[]>('/domains');
      setDomains(data);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load domains');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchDomains();
  }, [fetchDomains]);

  function handleCreated(domain: Domain) {
    setDomains((prev) => [...prev, domain]);
    setSelectedId(domain.id);
  }

  function handleDeleted() {
    setDomains((prev) => prev.filter((d) => d.id !== selectedId));
    setSelectedId(null);
  }

  function handleRefresh() {
    fetchDomains();
  }

  const selectedDomain = domains.find((d) => d.id === selectedId) ?? null;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">Domains</h1>
        <Button onClick={() => setAddOpen(true)}>
          <PlusIcon className="h-4 w-4 mr-2" />
          Add Domain
        </Button>
      </div>

      {/* Domain list */}
      {loading ? (
        <p className="text-slate-500 dark:text-slate-400 text-sm">Loading domains...</p>
      ) : domains.length === 0 ? (
        <Card className="dark:bg-slate-800">
          <CardContent className="p-6 text-center text-slate-500 dark:text-slate-400 text-sm">
            No domains configured yet. Click "Add Domain" to get started.
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          {domains.map((domain) => {
            const isSelected = domain.id === selectedId;
            return (
              <div
                key={domain.id}
                onClick={() => setSelectedId(domain.id)}
                className={`p-4 rounded-lg border cursor-pointer transition-colors ${
                  isSelected
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-950 dark:border-blue-600'
                    : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 dark:hover:bg-slate-700'
                }`}
              >
                <div className="flex items-center gap-3">
                  <span className="font-medium text-slate-800 dark:text-slate-100">{domain.name}</span>
                  <Badge variant={domain.active ? 'default' : 'secondary'}>
                    {domain.active ? 'active' : 'inactive'}
                  </Badge>
                  <span className="text-xs text-slate-400 dark:text-slate-500">selector: {domain.dkim_selector}</span>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Domain detail tabs */}
      {selectedDomain && (
        <Card className="dark:bg-slate-800">
          <CardContent className="p-6">
            <div className="mb-4">
              <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100">{selectedDomain.name}</h2>
            </div>
            <Tabs key={selectedDomain.id} defaultValue="dns">
              <TabsList>
                <TabsTrigger value="dns">DNS Records</TabsTrigger>
                <TabsTrigger value="relay">Relay Config</TabsTrigger>
                <TabsTrigger value="dkim">DKIM Keys</TabsTrigger>
                <TabsTrigger value="settings">Settings</TabsTrigger>
              </TabsList>
              <TabsContent value="dns" className="mt-4">
                <DnsRecordsTab domain={selectedDomain} />
              </TabsContent>
              <TabsContent value="relay" className="mt-4">
                <RelayConfigTab domain={selectedDomain} />
              </TabsContent>
              <TabsContent value="dkim" className="mt-4">
                <DkimKeysTab domain={selectedDomain} onRefresh={handleRefresh} />
              </TabsContent>
              <TabsContent value="settings" className="mt-4">
                <SettingsTab domain={selectedDomain} onDeleted={handleDeleted} />
              </TabsContent>
            </Tabs>
          </CardContent>
        </Card>
      )}

      <AddDomainDialog open={addOpen} onOpenChange={setAddOpen} onCreated={handleCreated} />
    </div>
  );
}
