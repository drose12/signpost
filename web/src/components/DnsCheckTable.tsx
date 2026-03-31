import { useEffect, useState, useCallback } from 'react';
import { toast } from 'sonner';
import { api } from '@/api';
import type { DNSCheckRecord, DNSCheckResponse } from '@/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { CopyIcon, RefreshCwIcon, InfoIcon, CheckCircleIcon, AlertTriangleIcon, XCircleIcon } from 'lucide-react';

function statusBadge(status: DNSCheckRecord['status']) {
  switch (status) {
    case 'ok':
      return <Badge className="bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 border-green-200 dark:border-green-800"><CheckCircleIcon className="h-3 w-3 mr-1" />OK</Badge>;
    case 'missing':
      return <Badge className="bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300 border-amber-200 dark:border-amber-800"><AlertTriangleIcon className="h-3 w-3 mr-1" />Missing</Badge>;
    case 'update':
      return <Badge className="bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300 border-amber-200 dark:border-amber-800"><AlertTriangleIcon className="h-3 w-3 mr-1" />Update needed</Badge>;
    case 'conflict':
      return <Badge className="bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300 border-red-200 dark:border-red-800"><XCircleIcon className="h-3 w-3 mr-1" />Conflict</Badge>;
  }
}

function purposeLabel(purpose: string) {
  switch (purpose) {
    case 'spf': return 'SPF';
    case 'dkim': return 'DKIM';
    case 'dmarc': return 'DMARC';
    default: return purpose.toUpperCase();
  }
}

function copyValue(value: string) {
  navigator.clipboard.writeText(value).then(() => toast.success('Copied!')).catch(() => toast.error('Copy failed'));
}

interface DnsCheckTableProps {
  domainId: number;
  autoCheck?: boolean;
}

export function DnsCheckTable({ domainId, autoCheck = true }: DnsCheckTableProps) {
  const [records, setRecords] = useState<DNSCheckRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [checked, setChecked] = useState(false);

  const runCheck = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.get<DNSCheckResponse>(`/domains/${domainId}/dns/check`);
      setRecords(data.records);
      setChecked(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'DNS check failed');
    } finally {
      setLoading(false);
    }
  }, [domainId]);

  useEffect(() => {
    if (autoCheck) runCheck();
  }, [autoCheck, runCheck]);

  const needsAction = records.some((r) => r.status !== 'ok');

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Alert className="border-blue-200 bg-blue-50 dark:bg-blue-950 dark:border-blue-800 flex-1">
          <InfoIcon className="h-4 w-4 text-blue-600 dark:text-blue-400" />
          <AlertDescription className="text-blue-700 dark:text-blue-300 text-sm">
            DNS changes can take up to 24–48 hours to propagate.
          </AlertDescription>
        </Alert>
        <Button variant="outline" className="ml-4" onClick={runCheck} disabled={loading}>
          <RefreshCwIcon className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          {checked ? 'Re-check DNS' : 'Check DNS'}
        </Button>
      </div>

      {loading && !checked ? (
        <p className="text-slate-500 dark:text-slate-400 text-sm">Checking DNS records...</p>
      ) : !checked ? (
        <p className="text-slate-500 dark:text-slate-400 text-sm">Click "Check DNS" to compare your current records against what's recommended.</p>
      ) : records.length === 0 ? (
        <p className="text-slate-500 dark:text-slate-400 text-sm">No DNS records to check. Generate DKIM keys first.</p>
      ) : (
        <>
          {!needsAction && (
            <Alert className="border-green-200 bg-green-50 dark:bg-green-950 dark:border-green-800">
              <CheckCircleIcon className="h-4 w-4 text-green-600 dark:text-green-400" />
              <AlertDescription className="text-green-700 dark:text-green-300 text-sm">
                All DNS records look good! No changes needed.
              </AlertDescription>
            </Alert>
          )}

          <div className="overflow-x-auto">
          <Table className="table-fixed w-full">
            <TableHeader>
              <TableRow>
                <TableHead className="w-[80px]">Record</TableHead>
                <TableHead className="w-[200px]">FQDN</TableHead>
                <TableHead className="w-[120px]">Status</TableHead>
                <TableHead>Current Value</TableHead>
                <TableHead>Recommended Value</TableHead>
                <TableHead className="w-[50px]"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.map((record) => (
                <TableRow key={record.purpose}>
                  <TableCell>
                    <div className="flex items-center gap-1.5">
                      <Badge variant="outline">{purposeLabel(record.purpose)}</Badge>
                      <span className="text-xs text-slate-400 dark:text-slate-500">{record.type}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <span className="font-mono text-xs text-slate-700 dark:text-slate-300">{record.name}</span>
                      <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => copyValue(record.name)} aria-label="Copy FQDN">
                        <CopyIcon className="h-3 w-3" />
                      </Button>
                    </div>
                  </TableCell>
                  <TableCell>{statusBadge(record.status)}</TableCell>
                  <TableCell className="align-top">
                    <div className="font-mono text-xs break-all whitespace-normal text-slate-600 dark:text-slate-400">
                      {record.current ?? <span className="italic text-slate-400 dark:text-slate-500">Not found</span>}
                    </div>
                  </TableCell>
                  <TableCell className="align-top">
                    <div className="space-y-1">
                      {record.status === 'ok' ? (
                        <span className="text-xs text-green-600 dark:text-green-400">No change needed</span>
                      ) : (
                        <div className="font-mono text-xs break-all whitespace-normal">{record.recommended}</div>
                      )}
                      <div className="text-xs text-slate-400 dark:text-slate-500">{record.message}</div>
                    </div>
                  </TableCell>
                  <TableCell>
                    {record.status !== 'ok' && (
                      <Button variant="ghost" size="icon" onClick={() => copyValue(record.recommended)} aria-label="Copy recommended value">
                        <CopyIcon className="h-4 w-4" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          </div>
        </>
      )}
    </div>
  );
}
