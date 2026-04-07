// web/src/pages/MailLog.tsx
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '@/api';
import type { MailLogEntry, QueueItem, QueueResponse } from '@/types';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';
import {
  MailIcon,
  Trash2,
  Check,
  X,
  ChevronDown,
  ChevronUp,
  CheckCircle2,
  AlertTriangle,
  Search,
} from 'lucide-react';

const PAGE_SIZE = 50;

function relativeTime(ts: string): string {
  const now = Date.now();
  const then = new Date(ts).getTime();
  const diff = now - then;
  if (diff < 60000) return 'just now';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  return `${Math.floor(diff / 86400000)}d ago`;
}

function formatTime(ts: string): string {
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return ts;
  }
}

function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return debounced;
}

// --- Mail Log Tab ---

function MailLogTab() {
  const [entries, setEntries] = useState<MailLogEntry[]>([]);
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [searchText, setSearchText] = useState('');
  const debouncedSearch = useDebounce(searchText, 300);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const filtersActive = statusFilter !== 'all' || debouncedSearch.length > 0;

  const buildUrl = useCallback(
    (currentOffset: number) => {
      let url = `/logs?limit=${PAGE_SIZE}&offset=${currentOffset}`;
      if (statusFilter !== 'all') {
        url += `&status=${statusFilter}`;
      }
      if (debouncedSearch) {
        url += `&search=${encodeURIComponent(debouncedSearch)}`;
      }
      return url;
    },
    [statusFilter, debouncedSearch],
  );

  // Initial fetch and filter/search change
  useEffect(() => {
    let cancelled = false;

    async function fetchLogs() {
      try {
        setLoading(true);
        setOffset(0);
        setHasMore(true);
        setExpandedId(null);
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
  }, [statusFilter, debouncedSearch, buildUrl]);

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

  function clearFilters() {
    setStatusFilter('all');
    setSearchText('');
  }

  function toggleExpanded(id: number) {
    setExpandedId((prev) => (prev === id ? null : id));
  }

  return (
    <div className="space-y-4">
      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative flex-1 min-w-[200px] max-w-sm">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
          <Input
            placeholder="Search from, to, message ID, error..."
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-[140px]">
            <SelectValue placeholder="Filter status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="accepted">Accepted</SelectItem>
            <SelectItem value="sent">Sent</SelectItem>
            <SelectItem value="failed">Failed</SelectItem>
            <SelectItem value="deferred">Deferred</SelectItem>
            <SelectItem value="rejected">Rejected</SelectItem>
          </SelectContent>
        </Select>
        {filtersActive && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            Clear filters
          </Button>
        )}
        {entries.length > 0 && (
          <Button
            variant="outline"
            size="sm"
            className="ml-auto text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950"
            onClick={async () => {
              if (!confirm('Clear all mail log entries?')) return;
              try {
                await api.del('/logs');
                toast.success('Mail log cleared');
                setEntries([]);
              } catch (err) {
                toast.error(err instanceof Error ? err.message : 'Failed to clear logs');
              }
            }}
          >
            <Trash2 className="h-3.5 w-3.5 mr-1" />
            Clear logs
          </Button>
        )}
      </div>

      {/* Table */}
      {loading ? (
        <div className="flex items-center justify-center h-32 text-slate-500 dark:text-slate-400">
          Loading...
        </div>
      ) : entries.length === 0 ? (
        <Card className="dark:bg-slate-800">
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <MailIcon className="h-10 w-10 text-slate-300 dark:text-slate-600 mb-3" />
            <p className="text-slate-500 dark:text-slate-400 text-sm">
              {filtersActive
                ? 'No log entries match your filters.'
                : 'No mail log entries yet. Send a test email to see activity here.'}
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
                    <TableHead>Relay</TableHead>
                    <TableHead>DKIM</TableHead>
                    <TableHead className="w-8"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {entries.map((entry) => (
                    <>
                      <TableRow
                        key={entry.id}
                        className="cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-700/50"
                        onClick={() => toggleExpanded(entry.id)}
                      >
                        <TableCell className="text-xs text-slate-500 dark:text-slate-400 whitespace-nowrap">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span>{relativeTime(entry.timestamp)}</span>
                            </TooltipTrigger>
                            <TooltipContent side="top">
                              {formatTime(entry.timestamp)}
                            </TooltipContent>
                          </Tooltip>
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
                        <TableCell className="text-sm text-slate-600 dark:text-slate-400">
                          {entry.relay_host || (entry.status === 'sent' ? 'direct' : '\u2014')}
                        </TableCell>
                        <TableCell>
                          {entry.dkim_signed ? (
                            <Check className="h-4 w-4 text-green-500" />
                          ) : (
                            <X className="h-4 w-4 text-slate-400" />
                          )}
                        </TableCell>
                        <TableCell className="w-8 pr-3">
                          {expandedId === entry.id ? (
                            <ChevronUp className="h-4 w-4 text-slate-400" />
                          ) : (
                            <ChevronDown className="h-4 w-4 text-slate-400" />
                          )}
                        </TableCell>
                      </TableRow>
                      {expandedId === entry.id && (
                        <TableRow key={`${entry.id}-detail`}>
                          <TableCell colSpan={7} className="bg-slate-50 dark:bg-slate-800/80 px-6 py-3">
                            <div className="grid grid-cols-2 gap-x-8 gap-y-1 text-sm max-w-xl">
                              <span className="text-slate-500 dark:text-slate-400">Message ID</span>
                              <span className="font-mono text-xs break-all">
                                {entry.msg_id || '\u2014'}
                              </span>
                              <span className="text-slate-500 dark:text-slate-400">Source</span>
                              <span>
                                {entry.source_ip
                                  ? `${entry.source_ip} via port ${entry.source_port || '?'}`
                                  : '\u2014'}
                              </span>
                              <span className="text-slate-500 dark:text-slate-400">Attempts</span>
                              <span>{entry.attempt_count}</span>
                              {entry.subject && (
                                <>
                                  <span className="text-slate-500 dark:text-slate-400">Subject</span>
                                  <span>{entry.subject}</span>
                                </>
                              )}
                              {entry.error && (
                                <>
                                  <span className="text-slate-500 dark:text-slate-400">Error</span>
                                  <span className="text-red-600 dark:text-red-400 break-all">
                                    {entry.error}
                                  </span>
                                </>
                              )}
                            </div>
                          </TableCell>
                        </TableRow>
                      )}
                    </>
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

// --- Queue Tab ---

function QueueTab() {
  const [items, setItems] = useState<QueueItem[]>([]);
  const [count, setCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchQueue = useCallback(async () => {
    try {
      const data = await api.get<QueueResponse>('/queue');
      setItems(data.items || []);
      setCount(data.count);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to load queue';
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchQueue();
    intervalRef.current = setInterval(fetchQueue, 30000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [fetchQueue]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-32 text-slate-500 dark:text-slate-400">
        Loading...
      </div>
    );
  }

  if (count === 0 || items.length === 0) {
    return (
      <Card className="dark:bg-slate-800">
        <CardContent className="flex flex-col items-center justify-center py-12 text-center">
          <CheckCircle2 className="h-10 w-10 text-green-500 mb-3" />
          <p className="text-slate-500 dark:text-slate-400 text-sm">
            Queue is empty &mdash; all messages delivered.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {/* Amber banner */}
      <div className="flex items-center gap-2 rounded-md border border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/30 px-4 py-2 text-sm text-amber-800 dark:text-amber-300">
        <AlertTriangle className="h-4 w-4 shrink-0" />
        <span>{count} message{count !== 1 ? 's' : ''} waiting for delivery</span>
      </div>

      <Card className="dark:bg-slate-800">
        <CardContent className="p-0">
          <TooltipProvider>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Queued At</TableHead>
                  <TableHead>From</TableHead>
                  <TableHead>To</TableHead>
                  <TableHead>Attempts</TableHead>
                  <TableHead>Last Error</TableHead>
                  <TableHead>Queue</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => {
                  const allRecipients = item.recipients.map((r) => r.address).join(', ');
                  const maxAttempts = Math.max(...item.recipients.map((r) => r.attempts), 0);
                  const firstError = item.recipients.find((r) => r.last_error)?.last_error || '';

                  return (
                    <TableRow key={item.msg_id}>
                      <TableCell className="text-xs text-slate-500 dark:text-slate-400 whitespace-nowrap">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span>{relativeTime(item.queued_at)}</span>
                          </TooltipTrigger>
                          <TooltipContent side="top">
                            {formatTime(item.queued_at)}
                          </TooltipContent>
                        </Tooltip>
                      </TableCell>
                      <TableCell className="text-sm max-w-[180px] truncate">
                        {item.from}
                      </TableCell>
                      <TableCell className="text-sm max-w-[220px] truncate">
                        {allRecipients}
                      </TableCell>
                      <TableCell className="text-sm">{maxAttempts}</TableCell>
                      <TableCell className="text-sm max-w-[200px]">
                        {firstError ? (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span className="text-red-600 dark:text-red-400 truncate block max-w-[200px]">
                                {firstError}
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top" className="max-w-xs">
                              {firstError}
                            </TooltipContent>
                          </Tooltip>
                        ) : (
                          <span className="text-slate-400 dark:text-slate-500">&mdash;</span>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-slate-600 dark:text-slate-400">
                        {item.queue_name}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TooltipProvider>
        </CardContent>
      </Card>
    </div>
  );
}

// --- Main Page ---

export function MailLog() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">Mail Log</h1>

      <Tabs defaultValue="log">
        <TabsList>
          <TabsTrigger value="log">Mail Log</TabsTrigger>
          <TabsTrigger value="queue">Queue</TabsTrigger>
        </TabsList>

        <TabsContent value="log">
          <MailLogTab />
        </TabsContent>

        <TabsContent value="queue">
          <QueueTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
