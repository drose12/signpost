// web/src/pages/MailLog.tsx
import { useCallback, useEffect, useState } from 'react';
import { api } from '@/api';
import type { MailLogEntry } from '@/types';
import { StatusBadge } from '@/components/StatusBadge';
import { Card, CardContent } from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';
import { MailIcon, AlertCircleIcon } from 'lucide-react';

const PAGE_SIZE = 50;

function formatTime(ts: string): string {
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return ts;
  }
}

export function MailLog() {
  const [entries, setEntries] = useState<MailLogEntry[]>([]);
  const [filter, setFilter] = useState<string>('all');
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);

  const buildUrl = useCallback(
    (currentOffset: number) => {
      let url = `/logs?limit=${PAGE_SIZE}&offset=${currentOffset}`;
      if (filter !== 'all') {
        url += `&status=${filter}`;
      }
      return url;
    },
    [filter],
  );

  // Initial fetch and filter change
  useEffect(() => {
    let cancelled = false;

    async function fetchLogs() {
      try {
        setLoading(true);
        setOffset(0);
        setHasMore(true);
        const data = await api.get<MailLogEntry[]>(buildUrl(0));
        if (cancelled) return;
        setEntries(data);
        setHasMore(data.length >= PAGE_SIZE);
      } catch (err) {
        if (cancelled) return;
        const msg = err instanceof Error ? err.message : 'Failed to load mail logs';
        toast.error(msg);
        setEntries([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchLogs();
    return () => {
      cancelled = true;
    };
  }, [filter, buildUrl]);

  async function loadMore() {
    const nextOffset = offset + PAGE_SIZE;
    try {
      setLoadingMore(true);
      const data = await api.get<MailLogEntry[]>(buildUrl(nextOffset));
      setEntries((prev) => [...prev, ...data]);
      setOffset(nextOffset);
      setHasMore(data.length >= PAGE_SIZE);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to load more logs';
      toast.error(msg);
    } finally {
      setLoadingMore(false);
    }
  }

  function handleFilterChange(value: string) {
    setFilter(value);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">Mail Log</h1>
        <Select value={filter} onValueChange={handleFilterChange}>
          <SelectTrigger className="w-[140px]">
            <SelectValue placeholder="Filter status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="sent">Sent</SelectItem>
            <SelectItem value="failed">Failed</SelectItem>
            <SelectItem value="deferred">Deferred</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-32 text-slate-500 dark:text-slate-400">
          Loading...
        </div>
      ) : entries.length === 0 ? (
        <Card className="dark:bg-slate-800">
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <MailIcon className="h-10 w-10 text-slate-300 dark:text-slate-600 mb-3" />
            <p className="text-slate-500 dark:text-slate-400 text-sm">
              No mail log entries yet. Send a test email to see activity here.
            </p>
          </CardContent>
        </Card>
      ) : (
        <Card className="dark:bg-slate-800">
          <CardContent className="p-0">
            <TooltipProvider>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Time</TableHead>
                    <TableHead>From</TableHead>
                    <TableHead>To</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Error</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {entries.map((entry) => (
                    <TableRow key={entry.id}>
                      <TableCell className="text-xs text-slate-500 dark:text-slate-400 whitespace-nowrap">
                        {formatTime(entry.timestamp)}
                      </TableCell>
                      <TableCell className="text-sm max-w-[180px] truncate">
                        {entry.from_addr}
                      </TableCell>
                      <TableCell className="text-sm max-w-[180px] truncate">
                        {entry.to_addr}
                      </TableCell>
                      <TableCell>
                        <StatusBadge status={entry.status} />
                      </TableCell>
                      <TableCell className="text-sm max-w-[200px]">
                        {entry.error ? (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span className="inline-flex items-center gap-1 text-red-600 dark:text-red-400 cursor-default truncate max-w-[200px]">
                                <AlertCircleIcon className="h-3.5 w-3.5 shrink-0" />
                                <span className="truncate">{entry.error}</span>
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top" className="max-w-xs">
                              {entry.error}
                            </TooltipContent>
                          </Tooltip>
                        ) : (
                          <span className="text-slate-400 dark:text-slate-500">&mdash;</span>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TooltipProvider>
          </CardContent>
        </Card>
      )}

      {!loading && hasMore && entries.length > 0 && (
        <div className="flex justify-center">
          <Button variant="outline" onClick={loadMore} disabled={loadingMore}>
            {loadingMore ? 'Loading...' : 'Load more'}
          </Button>
        </div>
      )}
    </div>
  );
}
