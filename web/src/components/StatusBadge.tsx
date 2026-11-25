import type { Status } from '../types';

export function statusColor(status?: Status | null, isActive = true): string {
  if (!isActive) return 'bg-zinc-600';
  switch (status) {
    case 'up':
      return 'bg-emerald-500';
    case 'degraded':
      return 'bg-amber-500';
    case 'down':
      return 'bg-rose-500';
    default:
      return 'bg-zinc-700';
  }
}

export function StatusBadge({ status, isActive = true }: { status?: Status | null; isActive?: boolean }) {
  if (!isActive) {
    return (
      <span className="inline-flex items-center gap-1.5 rounded-full bg-zinc-800 px-2.5 py-0.5 text-xs font-medium text-zinc-400 border border-zinc-700">
        <span className="h-1.5 w-1.5 rounded-full bg-zinc-500" />
        Paused
      </span>
    );
  }
  switch (status) {
    case 'up':
      return (
        <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-xs font-medium text-emerald-400 border border-emerald-500/20">
          <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
          Operational
        </span>
      );
    case 'degraded':
      return (
        <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-500/10 px-2.5 py-0.5 text-xs font-medium text-amber-400 border border-amber-500/20">
          <span className="h-1.5 w-1.5 rounded-full bg-amber-400" />
          Degraded
        </span>
      );
    case 'down':
      return (
        <span className="inline-flex items-center gap-1.5 rounded-full bg-rose-500/10 px-2.5 py-0.5 text-xs font-medium text-rose-400 border border-rose-500/20">
          <span className="h-1.5 w-1.5 rounded-full bg-rose-400" />
          Down
        </span>
      );
    default:
      return (
        <span className="inline-flex items-center gap-1.5 rounded-full bg-zinc-800 px-2.5 py-0.5 text-xs font-medium text-zinc-400 border border-zinc-700">
          <span className="h-1.5 w-1.5 rounded-full bg-zinc-500" />
          Pending
        </span>
      );
  }
}

export function formatTPS(tps?: number | null): string {
  if (tps == null || Number.isNaN(tps)) return '—';
  return `${tps.toFixed(1)} tok/s`;
}

export function formatMs(ms?: number | null): string {
  if (ms == null || Number.isNaN(ms)) return '—';
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`;
  return `${Math.round(ms)} ms`;
}
