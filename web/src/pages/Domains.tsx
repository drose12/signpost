// web/src/pages/Domains.tsx
import { useEffect, useState, useCallback, useRef } from 'react';
import { toast } from 'sonner';
import { api } from '@/api';
import type { Domain, RelayConfig, DKIMGenerateResponse, RelayTestResponse, PublicIPResponse } from '@/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';

import {
  PlusIcon, CopyIcon, KeyIcon, TrashIcon, Eye, EyeOff,
  Mail, Globe, Server, Settings, AlertTriangle, Pencil, Zap,
  DownloadIcon, UploadIcon,
} from 'lucide-react';
import { DnsCheckTable } from '@/components/DnsCheckTable';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type RelayMethod = 'gmail' | 'isp' | 'direct' | 'custom';

interface MethodFormValues {
  host: string;
  portPreset: string;
  customPort: string;
  username: string;
  password: string;
  starttls: boolean;
  authMethod?: string;
}

const METHOD_LABELS: Record<RelayMethod, string> = {
  gmail: 'Gmail SMTP',
  isp: 'ISP Relay',
  direct: 'Direct Delivery',
  custom: 'Custom SMTP',
};

const METHOD_ICONS: Record<RelayMethod, typeof Mail> = {
  gmail: Mail,
  isp: Server,
  direct: Globe,
  custom: Settings,
};

const METHOD_DEFAULTS: Record<RelayMethod, MethodFormValues> = {
  gmail: { host: 'smtp.gmail.com', portPreset: '587', customPort: '', username: '', password: '', starttls: true },
  isp: { host: '', portPreset: '587', customPort: '', username: '', password: '', starttls: true },
  direct: { host: '', portPreset: '25', customPort: '', username: '', password: '', starttls: false },
  custom: { host: '', portPreset: '587', customPort: '', username: '', password: '', starttls: true },
};

const ALL_METHODS: RelayMethod[] = ['gmail', 'isp', 'direct', 'custom'];

// ---------------------------------------------------------------------------
// DNS Records Card
// ---------------------------------------------------------------------------

function DnsRecordsCard({ domain, refreshKey }: { domain: Domain; refreshKey?: number }) {
  return (
    <Card className="dark:bg-slate-800">
      <CardHeader>
        <CardTitle className="text-base">DNS Records</CardTitle>
      </CardHeader>
      <CardContent>
        <DnsCheckTable key={`${domain.id}-${refreshKey ?? 0}`} domainId={domain.id} />
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Egress Host Field (for Direct Delivery SPF)
// ---------------------------------------------------------------------------

function EgressHostField({ publicIP, onSaved }: { publicIP: string | null; onSaved?: () => void }) {
  const [egressHost, setEgressHost] = useState('');
  const [resolvedIP, setResolvedIP] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    api.get<Record<string, string>>('/settings').then((s) => {
      setEgressHost(s.egress_host || '');
      setLoaded(true);
    }).catch(() => setLoaded(false));
  }, []);

  async function validate(host: string) {
    if (!host.trim()) {
      setResolvedIP(null);
      return;
    }
    try {
      // Use DNS lookup via our API — resolve the FQDN
      const resp = await fetch(`https://dns.google/resolve?name=${encodeURIComponent(host.trim())}&type=A`);
      const data = await resp.json();
      if (data.Answer && data.Answer.length > 0) {
        setResolvedIP(data.Answer[0].data);
      } else {
        setResolvedIP(null);
      }
    } catch {
      setResolvedIP(null);
    }
  }

  async function handleSave() {
    setSaving(true);
    try {
      await api.put('/settings', { egress_host: egressHost.trim() });
      toast.success(egressHost.trim() ? `Egress host set to ${egressHost.trim()}` : 'Egress host cleared');
      if (egressHost.trim()) validate(egressHost.trim());
      onSaved?.();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  }

  if (!loaded) return null;

  return (
    <div className="mt-3 pl-7 space-y-2">
      <div className="space-y-1.5">
        <Label htmlFor="egress-host" className="text-xs text-slate-600 dark:text-slate-400">
          Egress hostname (dynamic DNS)
        </Label>
        <div className="flex gap-2 items-center">
          <Input
            id="egress-host"
            value={egressHost}
            onChange={(e) => {
              setEgressHost(e.target.value);
              setResolvedIP(null);
            }}
            onBlur={() => validate(egressHost)}
            placeholder="e.g., myhost.dyndns.org"
            className="h-8 text-sm max-w-[280px]"
          />
          <Button size="sm" className="h-8" onClick={handleSave} disabled={saving}>
            {saving ? '...' : 'Save'}
          </Button>
        </div>
        {egressHost.trim() && resolvedIP && (
          <p className="text-xs text-green-600 dark:text-green-400">
            Resolves to {resolvedIP}
            {publicIP && resolvedIP === publicIP && ' — matches your public IP'}
            {publicIP && resolvedIP !== publicIP && <span className="text-amber-600 dark:text-amber-400"> — does not match your public IP ({publicIP})</span>}
          </p>
        )}
        {egressHost.trim() && resolvedIP === null && loaded && (
          <p className="text-xs text-slate-400">Enter hostname and click Save to validate</p>
        )}
        {!egressHost.trim() && publicIP && (
          <p className="text-xs text-slate-500 dark:text-slate-400">
            No hostname set — SPF will use <code className="font-mono bg-slate-100 dark:bg-slate-700 px-1 rounded">ip4:{publicIP}</code> (may change if your IP is dynamic)
          </p>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Relay Method Sub-Card
// ---------------------------------------------------------------------------

function RelayMethodCard({
  method,
  values,
  isActive,
  isConfigured,
  publicIP,
  onActivate,
  onEdit,
  onDnsRefresh,
}: {
  method: RelayMethod;
  values: MethodFormValues | null;
  isActive: boolean;
  isConfigured: boolean;
  publicIP: string | null;
  onActivate: () => void;
  onEdit: () => void;
  onDnsRefresh?: () => void;
}) {
  const Icon = METHOD_ICONS[method];
  const isLogin = values?.authMethod === 'login';
  const showLoginWarning = isLogin && (method === 'isp' || method === 'custom');

  function summaryLine(): string {
    if (!isConfigured || !values) return 'Not configured';
    if (method === 'direct') {
      return publicIP ? `Public IP: ${publicIP}` : 'Deliver directly from server';
    }
    const port = values.portPreset === 'custom' ? values.customPort : values.portPreset;
    const hostPart = values.host ? `${values.host}:${port}` : `port ${port}`;
    const userPart = values.username ? ` \u2022 ${values.username}` : '';
    return `${hostPart}${userPart}`;
  }

  return (
    <div className={`rounded-lg border p-4 transition-colors ${
      isActive
        ? 'border-green-300 bg-green-50/50 dark:border-green-700 dark:bg-green-950/30'
        : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/50'
    }`}>
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0 flex-1">
          <Icon className="h-4 w-4 flex-shrink-0 text-slate-500 dark:text-slate-400" />
          <span className="font-medium text-sm text-slate-800 dark:text-slate-100">{METHOD_LABELS[method]}</span>
          {isActive && (
            <Badge className="bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 border-green-200 dark:border-green-800 text-xs">
              Active
            </Badge>
          )}
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          {isConfigured && !isActive && (
            <Button variant="outline" size="sm" onClick={onActivate}>
              <Zap className="h-3 w-3 mr-1" />
              Activate
            </Button>
          )}
          <Button variant="ghost" size="sm" onClick={onEdit}>
            {isConfigured ? (
              <><Pencil className="h-3 w-3 mr-1" /> Edit</>
            ) : (
              <><PlusIcon className="h-3 w-3 mr-1" /> Setup</>
            )}
          </Button>
        </div>
      </div>
      <div className="mt-2 text-sm text-slate-600 dark:text-slate-400 pl-7">
        {summaryLine()}
      </div>

      {showLoginWarning && (
        <div className="mt-3 ml-7 p-3 rounded-md bg-amber-50 dark:bg-amber-950/40 border border-amber-200 dark:border-amber-800">
          <div className="flex gap-2">
            <AlertTriangle className="h-4 w-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" />
            <p className="text-xs text-amber-800 dark:text-amber-300">
              <strong>LOGIN auth relay:</strong> Mail is always DKIM signed. Web UI sends use Go
              to relay through this ISP with LOGIN auth. External clients (port 587) go through
              Maddy which can't do LOGIN — those emails are DKIM signed but delivered directly
              from your server IP (SPF may not pass unless your IP is in the SPF record).
            </p>
          </div>
        </div>
      )}

      {method === 'direct' && <EgressHostField publicIP={publicIP} onSaved={onDnsRefresh} />}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Relay Edit Dialog
// ---------------------------------------------------------------------------

function RelayEditDialog({
  open,
  onOpenChange,
  method,
  values,
  onSave,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  method: RelayMethod;
  values: MethodFormValues;
  onSave: (vals: MethodFormValues) => void;
}) {
  const [host, setHost] = useState(values.host);
  const [portPreset, setPortPreset] = useState(values.portPreset);
  const [customPort, setCustomPort] = useState(values.customPort);
  const [username, setUsername] = useState(values.username);
  const [password, setPassword] = useState(values.password);
  const [starttls, setStarttls] = useState(values.starttls);
  const [showPassword, setShowPassword] = useState(false);

  // Reset form when dialog opens with new values
  useEffect(() => {
    if (open) {
      setHost(values.host);
      setPortPreset(values.portPreset);
      setCustomPort(values.customPort);
      setUsername(values.username);
      setPassword(values.password);
      setStarttls(values.starttls);
      setShowPassword(false);
    }
  }, [open, values]);

  function handlePortPresetChange(val: string) {
    setPortPreset(val);
    if (val === '587') setStarttls(true);
    else if (val === '25') setStarttls(false);
    else if (val === '465') setStarttls(false); // implicit TLS
  }

  function handleSave() {
    const effectivePort = portPreset === 'custom' ? customPort : portPreset;
    onSave({
      host,
      portPreset,
      customPort,
      username,
      password,
      starttls: method === 'gmail' ? true : starttls,
      authMethod: values.authMethod,
    });
    // effectivePort used implicitly via portPreset/customPort
    void effectivePort;
    onOpenChange(false);
  }

  const isDirect = method === 'direct';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{METHOD_LABELS[method]} Configuration</DialogTitle>
          <DialogDescription>
            {method === 'gmail'
              ? 'Configure Gmail SMTP relay. Requires a Google App Password.'
              : method === 'isp'
              ? 'Configure your ISP mail relay.'
              : method === 'direct'
              ? 'Direct delivery sends mail from your server IP. No relay needed.'
              : 'Configure a custom SMTP relay server.'}
          </DialogDescription>
        </DialogHeader>

        {isDirect ? (
          <div className="space-y-3 py-2">
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Mail will be delivered directly from your server without an intermediate relay.
              Ensure your server IP is in your SPF record.
            </p>
          </div>
        ) : (
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="relay-host">Host</Label>
              <Input
                id="relay-host"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder={method === 'gmail' ? 'smtp.gmail.com' : 'smtp.example.com'}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="relay-port">Port</Label>
              {method === 'gmail' ? (
                <p className="text-sm text-slate-600 dark:text-slate-400 font-mono">587 (STARTTLS)</p>
              ) : (
                <>
                  <Select value={portPreset} onValueChange={handlePortPresetChange}>
                    <SelectTrigger id="relay-port">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="587">587 -- Submission (STARTTLS)</SelectItem>
                      <SelectItem value="465">465 -- SMTPS (Implicit TLS)</SelectItem>
                      <SelectItem value="25">25 -- SMTP (No encryption)</SelectItem>
                      <SelectItem value="custom">Custom port...</SelectItem>
                    </SelectContent>
                  </Select>
                  {portPreset === 'custom' && (
                    <Input
                      type="number"
                      value={customPort}
                      onChange={(e) => setCustomPort(e.target.value)}
                      placeholder="Port number"
                      className="mt-2"
                    />
                  )}
                </>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="relay-username">Username</Label>
              <Input
                id="relay-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder={method === 'gmail' ? 'you@gmail.com' : 'username'}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="relay-password">Password</Label>
              <div className="relative">
                <Input
                  id="relay-password"
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={method === 'gmail' ? 'App password' : 'Password'}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>

            {method === 'gmail' ? (
              <p className="text-xs text-slate-400 dark:text-slate-500">
                STARTTLS is always enabled for Gmail.{' '}
                <a
                  href="https://support.google.com/accounts/answer/185833"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline hover:text-slate-600"
                >
                  Get an App Password
                </a>
              </p>
            ) : portPreset === '25' ? (
              <p className="text-xs text-slate-400 dark:text-slate-500">Port 25 -- no encryption.</p>
            ) : portPreset === '465' ? (
              <p className="text-xs text-slate-400 dark:text-slate-500">Port 465 -- implicit TLS (no STARTTLS needed).</p>
            ) : (
              <div className="flex items-center gap-3">
                <Switch id="relay-starttls" checked={starttls} onCheckedChange={setStarttls} />
                <Label htmlFor="relay-starttls">STARTTLS</Label>
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSave}>Save</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Relay Configuration Card
// ---------------------------------------------------------------------------

function RelayConfigCard({ domain, onDnsRefresh }: { domain: Domain; onDnsRefresh?: () => void }) {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [activeMethod, setActiveMethod] = useState<RelayMethod>('direct');
  const [methodCache, setMethodCache] = useState<Record<string, MethodFormValues>>({});
  const [editingMethod, setEditingMethod] = useState<RelayMethod | null>(null);
  const [publicIP, setPublicIP] = useState<string | null>(null);

  // Parse a RelayConfig from the API into form values
  function parseRelayConfig(rc: RelayConfig): MethodFormValues {
    const m = (rc.method as RelayMethod) || 'direct';
    const p = String(rc.port || METHOD_DEFAULTS[m].portPreset);
    let portPreset: string;
    let customPort = '';
    if (['25', '465', '587'].includes(p)) {
      portPreset = p;
    } else {
      portPreset = 'custom';
      customPort = p;
    }
    return {
      host: rc.host ?? METHOD_DEFAULTS[m].host,
      portPreset,
      customPort,
      username: rc.username ?? '',
      password: rc.password ?? '',
      starttls: rc.starttls,
      authMethod: rc.auth_method,
    };
  }

  // Load ALL relay configs from API
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api.get<RelayConfig[]>(`/domains/${domain.id}/relay/all`)
      .then((configs) => {
        if (cancelled) return;
        const cache: Record<string, MethodFormValues> = {};
        let foundActive: RelayMethod | null = null;
        for (const rc of configs) {
          const m = rc.method as RelayMethod;
          cache[m] = parseRelayConfig(rc);
          if (rc.active) {
            foundActive = m;
          }
        }
        setMethodCache(cache);
        setActiveMethod(foundActive ?? 'direct');
        setLoading(false);
      })
      .catch((err) => {
        if (!cancelled) {
          toast.error(err instanceof Error ? err.message : 'Failed to load relay config');
          setLoading(false);
        }
      });
    return () => { cancelled = true; };
  }, [domain.id]);

  // Fetch public IP for direct delivery card
  useEffect(() => {
    api.get<PublicIPResponse>('/network/public-ip')
      .then((data) => setPublicIP(data.ip))
      .catch(() => setPublicIP(null));
  }, []);

  function isMethodConfigured(m: RelayMethod): boolean {
    if (m === 'direct') return m === activeMethod || !!methodCache[m];
    const vals = methodCache[m];
    if (!vals) return false;
    return !!vals.host;
  }

  function getMethodValues(m: RelayMethod): MethodFormValues | null {
    return methodCache[m] ?? null;
  }

  // Save a method config to the API. If activate=true, it becomes the active method.
  async function saveMethodToAPI(m: RelayMethod, vals: MethodFormValues, activate: boolean) {
    setSaving(true);
    const effectivePort = vals.portPreset === 'custom' ? vals.customPort : vals.portPreset;
    try {
      await api.put(`/domains/${domain.id}/relay`, {
        method: m,
        host: vals.host || undefined,
        port: parseInt(effectivePort, 10) || 587,
        username: vals.username || undefined,
        password: vals.password || undefined,
        starttls: m === 'gmail' ? true : vals.starttls,
        active: activate,
      });
      if (activate) {
        toast.success(`${METHOD_LABELS[m]} saved and activated`);
        setActiveMethod(m);
      } else {
        toast.success(`${METHOD_LABELS[m]} configuration saved`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save relay config');
    } finally {
      setSaving(false);
    }
  }

  function handleEditSave(m: RelayMethod, vals: MethodFormValues) {
    // Update local cache
    setMethodCache((prev) => ({ ...prev, [m]: vals }));
    // Always persist to DB; activate only if this is the active method
    const isCurrentlyActive = m === activeMethod;
    saveMethodToAPI(m, vals, isCurrentlyActive);
  }

  async function handleActivate(m: RelayMethod) {
    setSaving(true);
    try {
      await api.put(`/domains/${domain.id}/relay/${m}/activate`);
      toast.success(`${METHOD_LABELS[m]} activated`);
      setActiveMethod(m);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to activate relay method');
    } finally {
      setSaving(false);
    }
  }

  function handleEdit(m: RelayMethod) {
    setEditingMethod(m);
  }

  async function handleTestConnection() {
    setTesting(true);
    try {
      const result = await api.post<RelayTestResponse>(`/domains/${domain.id}/relay/test`);
      if (result.status === 'ok') {
        toast.success(result.message || 'Connection successful');
      } else {
        toast.error(result.error || 'Connection test failed');
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to test connection');
    } finally {
      setTesting(false);
    }
  }

  const editingValues = editingMethod
    ? (methodCache[editingMethod] ?? METHOD_DEFAULTS[editingMethod])
    : METHOD_DEFAULTS.direct;

  return (
    <Card className="dark:bg-slate-800">
      <CardHeader>
        <CardTitle className="text-base">Relay Configuration</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-slate-500 dark:text-slate-400 text-sm">Loading relay config...</p>
        ) : (
          <div className="space-y-3">
            {ALL_METHODS.map((m) => (
              <RelayMethodCard
                key={m}
                method={m}
                values={getMethodValues(m)}
                isActive={m === activeMethod}
                isConfigured={isMethodConfigured(m)}
                publicIP={m === 'direct' ? publicIP : null}
                onActivate={() => handleActivate(m)}
                onEdit={() => handleEdit(m)}
                onDnsRefresh={onDnsRefresh}
              />
            ))}

            <div className="pt-2">
              <Button variant="outline" onClick={handleTestConnection} disabled={testing || saving}>
                {testing ? 'Testing...' : 'Test Connection'}
              </Button>
            </div>
          </div>
        )}

        {editingMethod !== null && (
          <RelayEditDialog
            open={true}
            onOpenChange={(open) => { if (!open) setEditingMethod(null); }}
            method={editingMethod}
            values={editingValues}
            onSave={(vals) => handleEditSave(editingMethod, vals)}
          />
        )}
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// DKIM Keys Card
// ---------------------------------------------------------------------------

function DkimKeysCard({ domain, onRefresh }: { domain: Domain; onRefresh: () => void }) {
  const [generating, setGenerating] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [newDnsValue, setNewDnsValue] = useState<string | null>(null);
  const importInputRef = useRef<HTMLInputElement>(null);

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const result = await api.post<DKIMGenerateResponse>(`/domains/${domain.id}/dkim/import`, { pem: text });
      toast.success('DKIM key imported successfully');
      setNewDnsValue(result.dns_record_value);
      onRefresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Import failed');
    } finally {
      if (importInputRef.current) importInputRef.current.value = '';
    }
  }

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
    <Card className="dark:bg-slate-800">
      <CardHeader>
        <CardTitle className="text-base">DKIM Keys</CardTitle>
      </CardHeader>
      <CardContent>
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

          <div className="flex flex-wrap gap-2">
            {hasKeys ? (
              <>
                <Button variant="outline" onClick={() => setConfirmOpen(true)} disabled={generating}>
                  {generating ? 'Regenerating...' : 'Regenerate Keys'}
                </Button>
                <Button variant="outline" onClick={() => {
                  window.open(`/api/v1/domains/${domain.id}/dkim/export`, '_blank');
                }}>
                  <DownloadIcon className="h-4 w-4 mr-1.5" />
                  Export
                </Button>
                <Button variant="outline" onClick={() => importInputRef.current?.click()}>
                  <UploadIcon className="h-4 w-4 mr-1.5" />
                  Import
                </Button>
                <input
                  ref={importInputRef}
                  type="file"
                  accept=".pem,.key"
                  className="hidden"
                  onChange={handleImport}
                />
              </>
            ) : (
              <>
                <Button onClick={generateKeys} disabled={generating}>
                  {generating ? 'Generating...' : 'Generate DKIM Keys'}
                </Button>
                <Button variant="outline" onClick={() => importInputRef.current?.click()}>
                  <UploadIcon className="h-4 w-4 mr-1.5" />
                  Import
                </Button>
                <input
                  ref={importInputRef}
                  type="file"
                  accept=".pem,.key"
                  className="hidden"
                  onChange={handleImport}
                />
              </>
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
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Settings Card
// ---------------------------------------------------------------------------

function SettingsCard({ domain, onDeleted }: { domain: Domain; onDeleted: () => void }) {
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
    <Card className="dark:bg-slate-800">
      <CardHeader>
        <CardTitle className="text-base">Settings</CardTitle>
      </CardHeader>
      <CardContent>
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
      </CardContent>
    </Card>
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

type DomainHealth = 'green' | 'amber' | 'unknown';

export function Domains() {
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [dnsRefreshKey, setDnsRefreshKey] = useState(0);
  const [domainHealth, setDomainHealth] = useState<Record<number, DomainHealth>>({});

  const fetchDomains = useCallback(async () => {
    try {
      const data = await api.get<Domain[]>('/domains');
      setDomains(data);
      // Check DNS health for each domain in background
      for (const d of data) {
        api.get<{ records: Array<{ status: string }> }>(`/domains/${d.id}/dns/check`)
          .then((check) => {
            const allOk = check.records.every((r) => r.status === 'ok');
            setDomainHealth((prev) => ({ ...prev, [d.id]: allOk ? 'green' : 'amber' }));
          })
          .catch(() => {
            setDomainHealth((prev) => ({ ...prev, [d.id]: 'unknown' }));
          });
      }
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

  const [deleteTarget, setDeleteTarget] = useState<Domain | null>(null);
  const [deleting, setDeleting] = useState(false);

  function handleDeleted() {
    setDomains((prev) => prev.filter((d) => d.id !== selectedId));
    setSelectedId(null);
  }

  async function handleDeleteDomain(domain: Domain) {
    setDeleting(true);
    try {
      await api.del(`/domains/${domain.id}`);
      toast.success(`Deleted ${domain.name}`);
      setDomains((prev) => prev.filter((d) => d.id !== domain.id));
      if (selectedId === domain.id) setSelectedId(null);
      setDeleteTarget(null);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Delete failed');
    } finally {
      setDeleting(false);
    }
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
            const health = domainHealth[domain.id] ?? 'unknown';
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
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className={`w-2.5 h-2.5 rounded-full shrink-0 ${
                      health === 'green' ? 'bg-green-500' : health === 'amber' ? 'bg-amber-500' : 'bg-slate-300 dark:bg-slate-600'
                    }`} />
                    <span className="font-medium text-slate-800 dark:text-slate-100">{domain.name}</span>
                    {health === 'amber' && (
                      <span className="text-xs text-amber-600 dark:text-amber-400">DNS issues</span>
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-950"
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleteTarget(domain);
                    }}
                  >
                    <TrashIcon className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Domain detail cards */}
      {selectedDomain && (
        <div className="space-y-4">
          <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100">{selectedDomain.name}</h2>
          <DnsRecordsCard domain={selectedDomain} refreshKey={dnsRefreshKey} />
          <RelayConfigCard key={`relay-${selectedDomain.id}`} domain={selectedDomain} onDnsRefresh={() => setDnsRefreshKey((k) => k + 1)} />
          <DkimKeysCard domain={selectedDomain} onRefresh={handleRefresh} />
          <SettingsCard domain={selectedDomain} onDeleted={handleDeleted} />
        </div>
      )}

      <AddDomainDialog open={addOpen} onOpenChange={setAddOpen} onCreated={handleCreated} />

      {/* Delete domain confirmation dialog */}
      <Dialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Domain</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete <strong>{deleteTarget?.name}</strong>? This will remove all relay configuration, DKIM keys, and DNS records associated with this domain. This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)} disabled={deleting}>Cancel</Button>
            <Button variant="destructive" onClick={() => deleteTarget && handleDeleteDomain(deleteTarget)} disabled={deleting}>
              {deleting ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
