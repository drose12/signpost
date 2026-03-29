// web/src/components/StatusBadge.tsx

interface StatusBadgeProps {
  status: string;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  let colorClass: string;

  switch (status.toLowerCase()) {
    case 'running':
    case 'sent':
    case 'active':
      colorClass = 'text-green-600 dark:text-green-400';
      break;
    case 'stopped':
    case 'failed':
    case 'inactive':
      colorClass = 'text-red-500 dark:text-red-400';
      break;
    case 'deferred':
      colorClass = 'text-amber-500 dark:text-amber-400';
      break;
    default:
      colorClass = 'text-slate-500 dark:text-slate-400';
  }

  return (
    <span className={`inline-flex items-center gap-1.5 ${colorClass}`}>
      <span className="inline-block w-2 h-2 rounded-full bg-current" />
      {status}
    </span>
  );
}
