// web/src/pages/Wizard.tsx
import { Fragment, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api';
import type { Domain, DKIMGenerateResponse, TestSendResponse } from '@/types';
import { DnsCheckTable } from '@/components/DnsCheckTable';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Globe,
  Key,
  Send,
  Settings,
  InfoIcon,
  PlusIcon,
} from 'lucide-react';

// ---------------------------------------------------------------------------
// Types & constants
// ---------------------------------------------------------------------------

interface StepDef {
  number: number;
  title: string;
  icon: React.ReactNode;
}

const STEPS: StepDef[] = [
  { number: 1, title: 'Add Domain', icon: <Globe className="h-4 w-4" /> },
  { number: 2, title: 'Generate DKIM', icon: <Key className="h-4 w-4" /> },
  { number: 3, title: 'DNS Records', icon: <InfoIcon className="h-4 w-4" /> },
  { number: 4, title: 'Configure Relay', icon: <Settings className="h-4 w-4" /> },
  { number: 5, title: 'Send Test Email', icon: <Send className="h-4 w-4" /> },
];

type RelayMethod = 'gmail' | 'isp' | 'direct' | 'custom';

const METHOD_LABELS: Record<RelayMethod, string> = {
  gmail: 'Gmail SMTP',
  isp: 'ISP Relay',
  direct: 'Direct Delivery',
  custom: 'Custom SMTP',
};

// ---------------------------------------------------------------------------
// Step Indicator
// ---------------------------------------------------------------------------

function StepIndicator({
  step,
  isCurrent,
  isCompleted,
  onClick,
}: {
  step: StepDef;
  isCurrent: boolean;
  isCompleted: boolean;
  onClick: () => void;
}) {
  let circleClass: string;
  let textClass: string;

  if (isCompleted) {
    circleClass =
      'bg-green-500 text-white border-green-500 cursor-pointer hover:bg-green-600';
    textClass = 'text-green-700 dark:text-green-400 font-medium';
  } else if (isCurrent) {
    circleClass =
      'bg-blue-500 text-white border-blue-500 cursor-pointer';
    textClass = 'text-blue-700 dark:text-blue-400 font-medium';
  } else {
    circleClass =
      'bg-slate-200 dark:bg-slate-700 text-slate-400 dark:text-slate-500 border-slate-300 dark:border-slate-600';
    textClass = 'text-slate-400 dark:text-slate-500';
  }

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!isCurrent && !isCompleted}
      className="flex items-center gap-3 w-full text-left group"
    >
      <div
        className={`flex items-center justify-center w-8 h-8 rounded-full border-2 shrink-0 transition-colors ${circleClass}`}
      >
        {isCompleted ? <Check className="h-4 w-4" /> : <span className="text-sm font-semibold">{step.number}</span>}
      </div>
      <span className={`text-sm transition-colors ${textClass}`}>{step.title}</span>
    </button>
  );
}

// ---------------------------------------------------------------------------
// Step 1: Add Domain
// ---------------------------------------------------------------------------

function StepAddDomain({
  onComplete,
}: {
  onComplete: (domainId: number, domainName: string) => void;
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
      const domain = await api.post<Domain>('/domains', {
        name: name.trim(),
        dkim_selector: selector.trim() || 'signpost',
      });
      toast.success(`Domain "${domain.name}" created`);
      onComplete(domain.id, domain.name);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to create domain';
      setError(msg);
      toast.error(msg);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Card className="dark:bg-slate-800">
      <CardContent className="p-6">
        <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">
          Add Domain
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mb-4">
          Enter the domain name you want to configure for DKIM signing and mail relay.
        </p>
        <form onSubmit={handleSubmit} className="space-y-4 max-w-md">
          <div className="space-y-2">
            <Label htmlFor="wiz-domain-name">
              Domain Name <span className="text-red-500">*</span>
            </Label>
            <Input
              id="wiz-domain-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="example.com"
              required
              autoFocus
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="wiz-domain-selector">DKIM Selector</Label>
            <Input
              id="wiz-domain-selector"
              value={selector}
              onChange={(e) => setSelector(e.target.value)}
              placeholder="signpost"
            />
            <p className="text-xs text-slate-400 dark:text-slate-500">
              Default is "signpost". Only change if you know what you are doing.
            </p>
          </div>
          {error && <p className="text-red-500 text-sm">{error}</p>}
          <Button type="submit" disabled={submitting || !name.trim()}>
            {submitting ? 'Creating...' : 'Add Domain'}
            {!submitting && <ChevronRight className="h-4 w-4 ml-1" />}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Step 2: Generate DKIM
// ---------------------------------------------------------------------------

function StepGenerateDkim({
  domainId,
  domainName,
  onComplete,
  onBack,
}: {
  domainId: number;
  domainName: string;
  onComplete: (resp: DKIMGenerateResponse) => void;
  onBack: () => void;
}) {
  const [generating, setGenerating] = useState(false);

  async function handleGenerate() {
    setGenerating(true);
    try {
      const result = await api.post<DKIMGenerateResponse>(
        `/domains/${domainId}/dkim/generate`,
      );
      toast.success('DKIM keys generated successfully');
      onComplete(result);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to generate DKIM keys';
      toast.error(msg);
    } finally {
      setGenerating(false);
    }
  }

  return (
    <Card className="dark:bg-slate-800">
      <CardContent className="p-6">
        <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">
          Generate DKIM Keys
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mb-4">
          Generate a DKIM signing key for <strong>{domainName}</strong>. This key will be used to
          cryptographically sign outgoing emails.
        </p>
        <div className="flex gap-2">
          <Button onClick={handleGenerate} disabled={generating}>
            <Key className="h-4 w-4 mr-2" />
            {generating ? 'Generating...' : 'Generate DKIM Keys'}
          </Button>
          <Button variant="outline" onClick={onBack}>
            <ChevronLeft className="h-4 w-4 mr-1" />
            Back
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Step 2 complete summary
// ---------------------------------------------------------------------------

function DkimSummary({ dkimResponse }: { dkimResponse: DKIMGenerateResponse }) {
  return (
    <div className="text-sm text-slate-600 dark:text-slate-400 space-y-1">
      <div className="flex items-center gap-2">
        <Key className="h-4 w-4 text-green-500" />
        <span className="font-medium text-slate-700 dark:text-slate-300">Keys generated</span>
      </div>
      <p>
        <span className="font-medium">Selector:</span> {dkimResponse.selector}
      </p>
      <p>
        <span className="font-medium">Key path:</span>{' '}
        <span className="font-mono text-xs">{dkimResponse.key_path}</span>
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Step 3: DNS Records
// ---------------------------------------------------------------------------

function StepDnsRecords({
  domainId,
  onComplete,
  onSkip,
  onBack,
}: {
  domainId: number;
  onComplete: () => void;
  onSkip: () => void;
  onBack: () => void;
}) {
  return (
    <Card className="dark:bg-slate-800">
      <CardContent className="p-6">
        <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">
          Configure DNS Records
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mb-4">
          Review your current DNS records against what SignPost recommends. Copy any records that
          need updating to your DNS provider.
        </p>

        <DnsCheckTable domainId={domainId} />

        <div className="flex gap-2 mt-4">
          <Button onClick={onComplete}>
            Next
            <ChevronRight className="h-4 w-4 ml-1" />
          </Button>
          <Button variant="outline" onClick={onSkip}>
            Skip for now
          </Button>
          <Button variant="outline" onClick={onBack}>
            <ChevronLeft className="h-4 w-4 mr-1" />
            Back
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Step 4: Configure Relay
// ---------------------------------------------------------------------------

const METHOD_HELP: Record<RelayMethod, string> = {
  gmail: 'To relay through Gmail, you need a Google App Password. Go to myaccount.google.com → Security → 2-Step Verification → App passwords. Create one for "Mail" and paste it below. Your username is your full Gmail address.',
  isp: 'Use the SMTP server provided by your ISP or hosting provider. Check their documentation for the host, port, and credentials. This is a good option if your ISP allows outbound SMTP.',
  custom: 'Enter the details for any SMTP server you want to relay through. You\'ll need the hostname, port, and authentication credentials from your mail provider.',
  direct: '',
};

function StepConfigureRelay({
  domainId,
  onComplete,
  onBack,
}: {
  domainId: number;
  onComplete: () => void;
  onBack: () => void;
}) {
  const [method, setMethod] = useState<RelayMethod>('gmail');
  const [host, setHost] = useState('smtp.gmail.com');
  const [port, setPort] = useState('587');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [starttls, setStarttls] = useState(true);
  const [saving, setSaving] = useState(false);

  function handleMethodChange(val: string) {
    const m = val as RelayMethod;
    setMethod(m);
    switch (m) {
      case 'gmail':
        setHost('smtp.gmail.com');
        setPort('587');
        setStarttls(true);
        break;
      case 'isp':
      case 'custom':
        setHost('');
        setPort('587');
        setStarttls(true);
        break;
      case 'direct':
        setHost('');
        setPort('25');
        setStarttls(false);
        break;
    }
  }

  async function handleSave() {
    setSaving(true);
    try {
      await api.put(`/domains/${domainId}/relay`, {
        method,
        host: host || undefined,
        port: parseInt(port, 10) || 25,
        username: username || undefined,
        password: password || undefined,
        starttls,
      });
      toast.success('Relay configuration saved');
      onComplete();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to save relay config';
      toast.error(msg);
    } finally {
      setSaving(false);
    }
  }

  const showFields = method !== 'direct';

  return (
    <Card className="dark:bg-slate-800">
      <CardContent className="p-6">
        <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">
          Configure Relay
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mb-4">
          Choose how outgoing mail is delivered. For most setups, relaying through Gmail or your
          ISP is recommended.
        </p>

        <div className="space-y-4 max-w-md">
          <div className="space-y-2">
            <Label htmlFor="wiz-relay-method">Relay Method</Label>
            <Select value={method} onValueChange={handleMethodChange}>
              <SelectTrigger id="wiz-relay-method">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(METHOD_LABELS) as RelayMethod[]).map((m) => (
                  <SelectItem key={m} value={m}>
                    {METHOD_LABELS[m]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {method === 'direct' ? (
            <Alert className="border-amber-200 bg-amber-50 dark:bg-amber-950 dark:border-amber-800">
              <InfoIcon className="h-4 w-4 text-amber-600 dark:text-amber-400" />
              <AlertDescription className="text-amber-700 dark:text-amber-300 text-sm">
                Direct delivery sends mail without a relay. This may fail if your server IP is
                on blocklists or lacks proper reverse DNS. Consider using Gmail or ISP relay
                instead.
              </AlertDescription>
            </Alert>
          ) : (
            <Alert className="border-blue-200 bg-blue-50 dark:bg-blue-950 dark:border-blue-800">
              <InfoIcon className="h-4 w-4 text-blue-600 dark:text-blue-400" />
              <AlertDescription className="text-blue-700 dark:text-blue-300 text-sm">
                {METHOD_HELP[method]}
              </AlertDescription>
            </Alert>
          )}

          {showFields && (
            <>
              <div className="space-y-2">
                <Label htmlFor="wiz-relay-host">Host</Label>
                <Input
                  id="wiz-relay-host"
                  value={host}
                  onChange={(e) => setHost(e.target.value)}
                  placeholder={method === 'gmail' ? 'smtp.gmail.com' : 'smtp.example.com'}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="wiz-relay-port">Port</Label>
                <Input
                  id="wiz-relay-port"
                  type="number"
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="wiz-relay-username">Username</Label>
                <Input
                  id="wiz-relay-username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder={method === 'gmail' ? 'you@gmail.com' : 'username'}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="wiz-relay-password">Password</Label>
                <Input
                  id="wiz-relay-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={method === 'gmail' ? 'App password' : 'Password'}
                />
              </div>
              <div className="flex items-center gap-3">
                <Switch
                  id="wiz-relay-starttls"
                  checked={starttls}
                  onCheckedChange={setStarttls}
                />
                <Label htmlFor="wiz-relay-starttls">STARTTLS</Label>
              </div>
            </>
          )}

          <div className="flex gap-2">
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save & Continue'}
              {!saving && <ChevronRight className="h-4 w-4 ml-1" />}
            </Button>
            <Button variant="outline" onClick={onBack}>
              <ChevronLeft className="h-4 w-4 mr-1" />
              Back
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Step 5: Send Test Email
// ---------------------------------------------------------------------------

function StepTestEmail({
  domainName,
  onComplete,
  onBack,
}: {
  domainName: string;
  onComplete: () => void;
  onBack: () => void;
}) {
  const [to, setTo] = useState('');
  const [from] = useState(`test@${domainName}`);
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<TestSendResponse | null>(null);

  async function handleSend(e: React.FormEvent) {
    e.preventDefault();
    if (!to.trim()) return;
    setSending(true);
    setResult(null);
    try {
      const resp = await api.post<TestSendResponse>('/test/send', {
        from,
        to: to.trim(),
        subject: 'SignPost Test Email',
        body: `This is a test email sent from SignPost for domain ${domainName}.`,
      });
      setResult(resp);
      if (resp.status === 'sent' || resp.status === 'queued') {
        toast.success('Test email sent');
        onComplete();
      } else {
        toast.error(resp.error || 'Test email failed');
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to send test email';
      setResult({ status: 'failed', error: msg });
      toast.error(msg);
    } finally {
      setSending(false);
    }
  }

  return (
    <Card className="dark:bg-slate-800">
      <CardContent className="p-6">
        <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">
          Send Test Email
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mb-4">
          Verify your setup by sending a test email through SignPost.
        </p>

        <form onSubmit={handleSend} className="space-y-4 max-w-md">
          <div className="space-y-2">
            <Label htmlFor="wiz-test-from">From</Label>
            <Input id="wiz-test-from" value={from} disabled />
          </div>
          <div className="space-y-2">
            <Label htmlFor="wiz-test-to">
              To <span className="text-red-500">*</span>
            </Label>
            <Input
              id="wiz-test-to"
              type="email"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              placeholder="you@example.com"
              required
              autoFocus
            />
          </div>
          <div className="flex gap-2">
            <Button type="submit" disabled={sending || !to.trim()}>
              <Send className="h-4 w-4 mr-2" />
              {sending ? 'Sending...' : 'Send Test Email'}
            </Button>
            <Button type="button" variant="outline" onClick={onBack}>
              <ChevronLeft className="h-4 w-4 mr-1" />
              Back
            </Button>
          </div>
        </form>

        {result && (
          <div className="mt-4">
            {result.status === 'sent' || result.status === 'queued' ? (
              <Alert className="border-green-200 bg-green-50 dark:bg-green-950 dark:border-green-800">
                <Check className="h-4 w-4 text-green-600 dark:text-green-400" />
                <AlertDescription className="text-green-700 dark:text-green-300 text-sm">
                  {result.message || 'Test email sent successfully! Check your inbox.'}
                </AlertDescription>
              </Alert>
            ) : (
              <Alert className="border-red-200 bg-red-50 dark:bg-red-950 dark:border-red-800">
                <InfoIcon className="h-4 w-4 text-red-600 dark:text-red-400" />
                <AlertDescription className="text-red-700 dark:text-red-300 text-sm">
                  {result.error || 'Test email failed. Check your relay configuration.'}
                </AlertDescription>
              </Alert>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Main Wizard
// ---------------------------------------------------------------------------

export function Wizard() {
  const [currentStep, setCurrentStep] = useState(1);
  const [completedSteps, setCompletedSteps] = useState<Set<number>>(new Set());
  const [domainId, setDomainId] = useState<number | null>(null);
  const [domainName, setDomainName] = useState('');
  const [dkimResponse, setDkimResponse] = useState<DKIMGenerateResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [existingDomains, setExistingDomains] = useState<Domain[]>([]);
  const [started, setStarted] = useState(false);

  // First-run detection
  useEffect(() => {
    let cancelled = false;
    api
      .get<Domain[]>('/domains')
      .then((domains) => {
        if (!cancelled) {
          setExistingDomains(domains);
          if (domains.length === 0) {
            setStarted(true);
          }
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setLoading(false);
          setStarted(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function completeStep(step: number) {
    setCompletedSteps((prev) => new Set(prev).add(step));
    if (step < 5) {
      setCurrentStep(step + 1);
    }
  }

  function handleStepClick(stepNumber: number) {
    if (completedSteps.has(stepNumber) || stepNumber === currentStep) {
      setCurrentStep(stepNumber);
    }
  }

  const allDone = completedSteps.size === 5;

  if (loading) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">
          Setup Wizard
        </h1>
        <p className="text-slate-500 dark:text-slate-400 text-sm">Loading...</p>
      </div>
    );
  }

  async function deleteDomain(domain: Domain) {
    if (!confirm(`Delete "${domain.name}" and all its config? This cannot be undone.`)) return;
    try {
      await api.del(`/domains/${domain.id}`);
      toast.success(`Deleted ${domain.name}`);
      const updated = existingDomains.filter((d) => d.id !== domain.id);
      setExistingDomains(updated);
      if (updated.length === 0) setStarted(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete domain');
    }
  }

  function continueWithDomain(domain: Domain) {
    setDomainId(domain.id);
    setDomainName(domain.name);
    // Mark step 1 as done since domain exists
    setCompletedSteps((prev) => new Set(prev).add(1));
    // Skip to step 2 if no DKIM, step 3 if DKIM exists
    if (domain.dkim_key_path) {
      setCompletedSteps((prev) => new Set(prev).add(2));
      setCurrentStep(3);
    } else {
      setCurrentStep(2);
    }
    setStarted(true);
  }

  // Show intro screen when domains already exist
  if (!started) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">
          Setup Wizard
        </h1>

        {/* Existing domains */}
        <Card className="dark:bg-slate-800">
          <CardContent className="p-6 space-y-4">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100">
                Existing domains
              </h2>
              <p className="text-sm text-slate-500 dark:text-slate-400">
                Continue configuring an existing domain, or add a new one.
              </p>
            </div>
            <div className="space-y-2">
              {existingDomains.map((domain) => (
                <div
                  key={domain.id}
                  className="flex items-center justify-between p-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900"
                >
                  <div className="flex items-center gap-3">
                    <Globe className="h-4 w-4 text-slate-400" />
                    <span className="font-medium text-slate-800 dark:text-slate-100">{domain.name}</span>
                    {domain.dkim_key_path ? (
                      <span className="text-xs text-green-600 dark:text-green-400 flex items-center gap-1">
                        <Check className="h-3 w-3" /> DKIM
                      </span>
                    ) : (
                      <span className="text-xs text-amber-600 dark:text-amber-400">No DKIM</span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <Button variant="outline" size="sm" onClick={() => continueWithDomain(domain)}>
                      Continue setup
                      <ChevronRight className="h-4 w-4 ml-1" />
                    </Button>
                    <Button variant="ghost" size="sm" className="text-red-500 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-950" onClick={() => deleteDomain(domain)}>
                      Delete
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Add new domain option */}
        <Card className="dark:bg-slate-800">
          <CardContent className="p-6 space-y-4">
            <div className="space-y-2">
              <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100">
                Add a new domain
              </h2>
              <p className="text-sm text-slate-500 dark:text-slate-400">
                Set up a new domain with DKIM signing, DNS records, relay settings, and test email delivery.
              </p>
            </div>
            <Button onClick={() => setStarted(true)}>
              <PlusIcon className="h-4 w-4 mr-2" />
              Add new domain
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">
          Setup Wizard
        </h1>
        {existingDomains.length === 0 && currentStep === 1 && !completedSteps.has(1) && (
          <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
            Welcome to SignPost! Let's set up your first domain for DKIM-signed email delivery.
          </p>
        )}
      </div>

      {allDone && (
        <Alert className="border-green-200 bg-green-50 dark:bg-green-950 dark:border-green-800">
          <Check className="h-4 w-4 text-green-600 dark:text-green-400" />
          <AlertDescription className="text-green-700 dark:text-green-300 text-sm">
            Setup complete! Your domain <strong>{domainName}</strong> is configured and ready.
            Visit the{' '}
            <a href="/domains" className="underline font-medium">
              Domains
            </a>{' '}
            page to manage it.
          </AlertDescription>
        </Alert>
      )}

      <div className="flex gap-8">
        {/* Timeline sidebar */}
        <div className="flex flex-col items-start shrink-0">
          {STEPS.map((step, i) => (
            <Fragment key={step.number}>
              <StepIndicator
                step={step}
                isCurrent={step.number === currentStep}
                isCompleted={completedSteps.has(step.number)}
                onClick={() => handleStepClick(step.number)}
              />
              {i < STEPS.length - 1 && (
                <div
                  className={`w-px h-8 ml-4 ${
                    completedSteps.has(step.number)
                      ? 'bg-green-300 dark:bg-green-700'
                      : 'bg-slate-300 dark:bg-slate-600'
                  }`}
                />
              )}
            </Fragment>
          ))}
        </div>

        {/* Step content */}
        <div className="flex-1 min-w-0">
          {currentStep === 1 && (
            completedSteps.has(1) ? (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6">
                  <div className="flex items-center gap-2 text-green-600 dark:text-green-400 mb-2">
                    <Check className="h-5 w-5" />
                    <span className="font-medium">Domain added</span>
                  </div>
                  <p className="text-sm text-slate-600 dark:text-slate-400">
                    <strong>{domainName}</strong> has been created.
                  </p>
                </CardContent>
              </Card>
            ) : (
              <StepAddDomain
                onComplete={(id, name) => {
                  setDomainId(id);
                  setDomainName(name);
                  completeStep(1);
                }}
              />
            )
          )}

          {currentStep === 2 && (
            completedSteps.has(2) ? (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6">
                  <div className="flex items-center gap-2 text-green-600 dark:text-green-400 mb-2">
                    <Check className="h-5 w-5" />
                    <span className="font-medium">DKIM keys generated</span>
                  </div>
                  {dkimResponse && <DkimSummary dkimResponse={dkimResponse} />}
                </CardContent>
              </Card>
            ) : domainId ? (
              <StepGenerateDkim
                domainId={domainId}
                domainName={domainName}
                onComplete={(resp) => {
                  setDkimResponse(resp);
                  completeStep(2);
                }}
                onBack={() => setCurrentStep(1)}
              />
            ) : (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6 text-slate-500 dark:text-slate-400 text-sm">
                  Complete step 1 first.
                </CardContent>
              </Card>
            )
          )}

          {currentStep === 3 && (
            completedSteps.has(3) ? (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6">
                  <div className="flex items-center gap-2 text-green-600 dark:text-green-400 mb-2">
                    <Check className="h-5 w-5" />
                    <span className="font-medium">DNS records reviewed</span>
                  </div>
                  <p className="text-sm text-slate-600 dark:text-slate-400">
                    DNS records are available on the Domains page.
                  </p>
                </CardContent>
              </Card>
            ) : domainId ? (
              <StepDnsRecords
                domainId={domainId}
                onComplete={() => completeStep(3)}
                onSkip={() => completeStep(3)}
                onBack={() => setCurrentStep(2)}
              />
            ) : (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6 text-slate-500 dark:text-slate-400 text-sm">
                  Complete earlier steps first.
                </CardContent>
              </Card>
            )
          )}

          {currentStep === 4 && (
            completedSteps.has(4) ? (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6">
                  <div className="flex items-center gap-2 text-green-600 dark:text-green-400 mb-2">
                    <Check className="h-5 w-5" />
                    <span className="font-medium">Relay configured</span>
                  </div>
                  <p className="text-sm text-slate-600 dark:text-slate-400">
                    Relay settings saved. You can adjust them on the Domains page.
                  </p>
                </CardContent>
              </Card>
            ) : domainId ? (
              <StepConfigureRelay
                domainId={domainId}
                onComplete={() => completeStep(4)}
                onBack={() => setCurrentStep(3)}
              />
            ) : (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6 text-slate-500 dark:text-slate-400 text-sm">
                  Complete earlier steps first.
                </CardContent>
              </Card>
            )
          )}

          {currentStep === 5 && (
            completedSteps.has(5) ? (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6">
                  <div className="flex items-center gap-2 text-green-600 dark:text-green-400 mb-2">
                    <Check className="h-5 w-5" />
                    <span className="font-medium">Test email sent</span>
                  </div>
                  <p className="text-sm text-slate-600 dark:text-slate-400">
                    Setup is complete for <strong>{domainName}</strong>.
                  </p>
                </CardContent>
              </Card>
            ) : domainName ? (
              <StepTestEmail
                domainName={domainName}
                onComplete={() => completeStep(5)}
                onBack={() => setCurrentStep(4)}
              />
            ) : (
              <Card className="dark:bg-slate-800">
                <CardContent className="p-6 text-slate-500 dark:text-slate-400 text-sm">
                  Complete earlier steps first.
                </CardContent>
              </Card>
            )
          )}
        </div>
      </div>
    </div>
  );
}
